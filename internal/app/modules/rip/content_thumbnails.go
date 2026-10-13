package rip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/css"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

type thumbnailSession struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// The existing CSS copy cannot be interrupted inside its loop. Serialize source
// preparation so rapid source changes cannot start several disc copies at once.
var thumbnailPreparationGate = make(chan struct{}, 1)

func (cb *ContentBrowser) extractScanThumbnails(session *thumbnailSession, source string, cards []*titleCardState) {
	path, cleanup, err := prepareThumbnailSource(session.ctx, source)
	if err != nil {
		if session.ctx.Err() == nil {
			logging.Debug(logging.CatDVD, "content_browser: thumbnail source preparation failed: %v", err)
			cb.finishThumbnailPreparation(session, cards)
		}
		return
	}
	defer cleanup()
	dvdVideo := SupportsDVDVideo()
	type thumbnailJob struct {
		card      *titleCardState
		extractor *titleThumbnailExtractor
		position  float64
		done      *sync.WaitGroup
	}
	extractors := make([]*titleThumbnailExtractor, len(cards))
	for i, card := range cards {
		extractors[i] = &titleThumbnailExtractor{source: path, title: card.title.Number, dvdVideo: dvdVideo}
		defer extractors[i].close()
	}
	jobs := make(chan thumbnailJob)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				cb.extractThumbnail(session, job.extractor, job.card, job.position)
				job.done.Done()
			}
		}()
	}
	// Give every title its first still before preparing their later cycling
	// frames. Phase barriers keep each title's extractor on one worker at a time.
	for _, position := range []float64{0.10, 0.25, 0.50, 0.75, 0.90} {
		var phase sync.WaitGroup
		for i, card := range cards {
			phase.Add(1)
			select {
			case <-session.ctx.Done():
				phase.Done()
				close(jobs)
				workers.Wait()
				return
			case jobs <- thumbnailJob{card: card, extractor: extractors[i], position: position, done: &phase}:
			}
		}
		phase.Wait()
	}
	close(jobs)
	workers.Wait()
	cb.finishThumbnailPreparation(session, cards)
}

// Called by UI dispatchers only. Session identity prevents a delayed worker
// result from changing either frames or status on a replacement source.
func (cb *ContentBrowser) applyThumbnailResult(session *thumbnailSession, card *titleCardState, resource fyne.Resource, state string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.thumbnails != session || session.ctx.Err() != nil {
		return false
	}
	if resource != nil {
		card.frames = append(card.frames, resource)
	}
	if state == "failed" && len(card.frames) > 0 {
		return false
	}
	card.previewState = state
	return true
}

func (cb *ContentBrowser) finishThumbnailPreparation(session *thumbnailSession, cards []*titleCardState) {
	fyne.Do(func() {
		changed := false
		for _, card := range cards {
			if cb.applyThumbnailResult(session, card, nil, "failed") {
				changed = true
			}
		}
		if changed {
			cb.list.Refresh()
		}
	})
}

func (cb *ContentBrowser) extractThumbnail(session *thumbnailSession, extractor *titleThumbnailExtractor, card *titleCardState, position float64) {
	duration := card.title.Duration
	if duration <= 0 {
		duration = 120
	}
	if session.ctx.Err() != nil {
		return
	}
	data, err := extractor.frame(session.ctx, duration*position)
	if err != nil {
		if session.ctx.Err() != nil {
			return
		}
		logging.Debug(logging.CatDVD, "content_browser: thumb extract failed T%02d @%.0fs: %v", card.title.Number, duration*position, err)
		return
	}
	resource := fyne.NewStaticResource(fmt.Sprintf("thumb_t%d_%.0f.png", card.title.Number, position*100), data)
	fyne.Do(func() {
		current := cb.applyThumbnailResult(session, card, resource, "ready")
		// Refresh the list to bind the current card owner. Never write through an
		// image pointer retained by a scrolled-out row.
		if current {
			cb.list.Refresh()
		}
	})
}

// prepareThumbnailSource resolves/decrypts once for the entire scan, not once
// per title. CSS copying currently has no cancellation API; cancellation is
// checked on return and its scratch tree is always removed by the worker.
func prepareThumbnailSource(ctx context.Context, source string) (string, func(), error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	select {
	case thumbnailPreparationGate <- struct{}{}:
		defer func() { <-thumbnailPreparationGate }()
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
	path, clean, err := ResolveVideoTSPath(ctx, source)
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		if clean != nil {
			clean()
		}
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return "", nil, err
	}
	encrypted, err := css.IsCSSEncrypted(filepath.Join(path, "VIDEO_TS.IFO"))
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("thumbnail encryption check: %w", err)
	}
	if encrypted {
		decrypted, dclean, err := decryptVideoTSPath(path, func(string) {})
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("thumbnail decryption: %w", err)
		}
		prior := cleanup
		cleanup = func() { dclean(); prior() }
		path = decrypted
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

type titleThumbnailExtractor struct {
	source   string
	title    int
	dvdVideo bool
	fallback string
	cleanup  func()
}

func (e *titleThumbnailExtractor) close() {
	if e.cleanup != nil {
		e.cleanup()
	}
}

func (e *titleThumbnailExtractor) frame(ctx context.Context, timestamp float64) ([]byte, error) {
	var dvdErr error
	if e.dvdVideo {
		data, err := runThumbnailFFmpeg(ctx, []string{"-f", "dvdvideo", "-title", fmt.Sprint(e.title), "-i", dvdThumbnailInput(e.source)}, timestamp)
		if err == nil {
			return data, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		dvdErr = err
		e.dvdVideo = false // Do not retry the same failed demuxer for every still.
	}
	if e.fallback == "" {
		list, cleanup, err := thumbnailTitleConcat(e.source, e.title)
		if err != nil {
			return nil, fmt.Errorf("dvdvideo: %v; title fallback: %w", dvdErr, err)
		}
		e.fallback, e.cleanup = list, cleanup
	}
	return runThumbnailFFmpeg(ctx, []string{"-fflags", "+genpts", "-f", "concat", "-safe", "0", "-i", e.fallback}, timestamp)
}

func dvdThumbnailInput(videoTS string) string {
	if strings.EqualFold(filepath.Base(videoTS), "VIDEO_TS") {
		return filepath.Dir(videoTS)
	}
	return videoTS
}

// Reject missing title identity/cells rather than show the first title of a
// shared VTS. Existing rip fallback semantics are unchanged.
func thumbnailTitleConcat(videoTS string, title int) (string, func(), error) {
	titles, err := ifo.ReadTitleList(filepath.Join(videoTS, "VIDEO_TS.IFO"))
	if err != nil {
		return "", nil, err
	}
	if title < 1 || title > len(titles) {
		return "", nil, fmt.Errorf("title %d not in VMG", title)
	}
	target := titles[title-1]
	if target.VTSNumber == 0 || target.VTS_TitleNumber == 0 {
		return "", nil, fmt.Errorf("title %d has no VTS mapping", title)
	}
	sets, err := CollectVOBSets(videoTS)
	if err != nil {
		return "", nil, err
	}
	for _, set := range sets {
		if !strings.EqualFold(set.Name, fmt.Sprintf("VTS_%02d", target.VTSNumber)) {
			continue
		}
		info, err := ifo.ReadTitleInfoForTTN(filepath.Join(videoTS, fmt.Sprintf("VTS_%02d_0.IFO", target.VTSNumber)), int(target.VTS_TitleNumber))
		if err != nil {
			return "", nil, err
		}
		return thumbnailCellConcat(videoTS, set, info)
	}
	return "", nil, fmt.Errorf("title %d VOB set unavailable", title)
}

func thumbnailCellConcat(videoTS string, set VobSet, info *ifo.TitleInfo) (string, func(), error) {
	if info == nil || info.HasAngles || len(info.Cells) == 0 {
		return "", nil, fmt.Errorf("no unambiguous title cells")
	}
	files := vobSetFileByID(videoTS, set)
	for _, cell := range info.Cells {
		path := files[uint16(cell.VOBID)]
		sectors := vobSectorCount(path)
		if sectors == 0 || cell.FirstSector > cell.LastSector || cell.LastSector >= sectors {
			return "", nil, fmt.Errorf("title cell outside VOB bounds")
		}
	}
	// A synthetic one-program span forces exact slicing even for full coverage;
	// the general helper's empty-list result can also mean invalid metadata.
	exact := *info
	exact.ProgramEntryCells = []int{1}
	list, cleanup, err := cellConcatList(videoTS, set, &exact, 1, 1)
	if err != nil {
		return "", nil, err
	}
	if list == "" {
		return "", nil, fmt.Errorf("title cells could not be resolved")
	}
	return list, cleanup, nil
}

func runThumbnailFFmpeg(parent context.Context, input []string, timestamp float64) ([]byte, error) {
	tmp, err := os.CreateTemp(utils.TempDir(), "vt-cb-*.png")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(name)
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-ss", formatTimestamp(timestamp)}
	args = append(args, input...)
	output := []string{"-map", "0:v:0", "-frames:v", "1", "-vf", fmt.Sprintf("scale=%d:%d/dar,setsar=1", thumbWidth, thumbWidth), "-an", "-sn", "-y", name}
	args = append(args, output...)
	out, err := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...).CombinedOutput()
	if err != nil && ctx.Err() == nil {
		// Some authored DVDs reject demux seeking. Retry the same title while
		// decoding/discarding to its requested timestamp, within the same timeout.
		args = append([]string{"-hide_banner", "-loglevel", "error"}, input...)
		args = append(args, "-ss", formatTimestamp(timestamp))
		args = append(args, output...)
		retryOut, retryErr := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...).CombinedOutput()
		if retryErr == nil {
			err = nil
		} else {
			out = append(out, retryOut...)
			err = retryErr
		}
	}
	if err != nil {
		return nil, fmt.Errorf("ffmpeg thumbnail: %w: %s", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no thumbnail")
	}
	return data, nil
}
