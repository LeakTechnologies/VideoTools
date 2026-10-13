package rip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Serialize ISO source resolution across rapid source changes.
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
	keys := make(map[string][5]byte)
	// A static still per title keeps optical reads sequential and finite.
	ordered := make([]*titleCardState, 0, len(cards))
	for _, card := range cards {
		if card.isMain {
			ordered = append(ordered, card)
		}
	}
	for _, card := range cards {
		if !card.isMain {
			ordered = append(ordered, card)
		}
	}
	for _, card := range ordered {
		if session.ctx.Err() != nil {
			return
		}
		extractor := &titleThumbnailExtractor{source: path, title: card.title.Number, keys: keys}
		cb.extractThumbnail(session, extractor, card, 0.10)
	}
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
		fyne.Do(func() {
			if cb.applyThumbnailResult(session, card, nil, "failed") {
				cb.list.Refresh()
			}
		})
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

// Decrypt bounded title samples rather than copying a complete disc.
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
	return path, cleanup, nil
}

type titleThumbnailExtractor struct {
	source string
	title  int
	keys   map[string][5]byte
}

func (e *titleThumbnailExtractor) frame(ctx context.Context, _ float64) ([]byte, error) {
	if e.keys == nil {
		e.keys = make(map[string][5]byte)
	}
	name, cleanup, err := thumbnailTitleSampleWithKeys(ctx, e.source, e.title, e.keys)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	// Decode the first available frame in this exact-title sample.
	return runThumbnailFFmpeg(ctx, []string{"-f", "mpeg", "-i", name}, 0)
}

const thumbnailSampleBytes = 4 * 1024 * 1024

// thumbnailTitleSample reads a 4 MiB representative sample. An uncached CSS
// key can require another 4 MiB menu sample and 4 MiB title-opening sample.
// It never prepares the whole VTS or seeks/decodes through a feature film.
func thumbnailTitleSample(ctx context.Context, videoTS string, title int) (string, func(), error) {
	return thumbnailTitleSampleWithKeys(ctx, videoTS, title, make(map[string][5]byte))
}

func thumbnailTitleSampleWithKeys(ctx context.Context, videoTS string, title int, keys map[string][5]byte) (string, func(), error) {
	titles, err := ifo.ReadTitleList(filepath.Join(videoTS, "VIDEO_TS.IFO"))
	if err != nil {
		return "", nil, err
	}
	if title < 1 || title > len(titles) {
		return "", nil, fmt.Errorf("title not in VMG")
	}
	target := titles[title-1]
	if target.VTSNumber == 0 || target.VTS_TitleNumber == 0 {
		return "", nil, fmt.Errorf("title has no VTS mapping")
	}
	info, err := ifo.ReadTitleInfoForTTN(filepath.Join(videoTS, fmt.Sprintf("VTS_%02d_0.IFO", target.VTSNumber)), int(target.VTS_TitleNumber))
	if err != nil {
		return "", nil, err
	}
	set := VobSet{Name: fmt.Sprintf("VTS_%02d", target.VTSNumber)}
	data, err := thumbnailCellData(ctx, videoTS, set, info, true)
	if err != nil {
		return "", nil, err
	}
	if err = decryptThumbnailSampleWithKeys(ctx, videoTS, set, info, data, keys); err != nil {
		return "", nil, fmt.Errorf("thumbnail sample decryption: %w", err)
	}
	return writeThumbnailSample(ctx, data)
}

func thumbnailCellSample(ctx context.Context, videoTS string, set VobSet, info *ifo.TitleInfo) (string, func(), error) {
	data, err := thumbnailCellData(ctx, videoTS, set, info, true)
	if err != nil {
		return "", nil, err
	}
	if err = decryptThumbnailSampleWithKeys(ctx, videoTS, set, info, data, make(map[string][5]byte)); err != nil {
		return "", nil, err
	}
	return writeThumbnailSample(ctx, data)
}

func thumbnailCellData(ctx context.Context, videoTS string, set VobSet, info *ifo.TitleInfo, representative bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if info == nil || info.HasAngles || len(info.Cells) == 0 {
		return nil, fmt.Errorf("no unambiguous title cells")
	}
	files := vobSetFileByID(videoTS, set)
	// DVD title-domain sectors span VTS_XX_1.VOB through VTS_XX_N.VOB.
	// The cell's VOBID identifies a logical VOB, not its filename suffix.
	ids := make([]int, 0, len(files))
	for id := range files {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	var sourceSectors int64
	for i, id := range ids {
		if id != i+1 {
			return nil, fmt.Errorf("title VOB stream has missing segments")
		}
		sectors := vobSectorCount(files[uint16(id)])
		if sectors == 0 {
			return nil, fmt.Errorf("title VOB segment unavailable")
		}
		sourceSectors += int64(sectors)
	}
	var total int64
	for _, cell := range info.Cells {
		if cell.FirstSector > cell.LastSector || int64(cell.LastSector) >= sourceSectors {
			return nil, fmt.Errorf("title cell outside VOB bounds")
		}
		total += int64(cell.LastSector - cell.FirstSector + 1)
	}
	// Approximate 10% by mapped sectors. Restrict every byte to this title,
	// including when the bounded sample crosses a cell boundary.
	skip := total / 10
	if !representative {
		skip = 0
	}
	data := make([]byte, 0, thumbnailSampleBytes)
	for _, cell := range info.Cells {
		count := int64(cell.LastSector - cell.FirstSector + 1)
		if skip >= count {
			skip -= count
			continue
		}
		first := int64(cell.FirstSector) + skip
		count -= skip
		skip = 0
		remaining := int64(thumbnailSampleBytes-len(data)) / css.SectorSize
		if count > remaining {
			count = remaining
		}
		if count == 0 {
			break
		}
		for _, id := range ids {
			path := files[uint16(id)]
			sectors := int64(vobSectorCount(path))
			if first >= sectors {
				first -= sectors
				continue
			}
			next := min(count, sectors-first)
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			_, err = f.Seek(first*css.SectorSize, io.SeekStart)
			for next > 0 && err == nil {
				if err = ctx.Err(); err != nil {
					break
				}
				n := min(next, 32)
				chunk := make([]byte, int(n)*css.SectorSize)
				_, err = io.ReadFull(f, chunk)
				if err == nil {
					data = append(data, chunk...)
				}
				next -= n
				count -= n
			}
			_ = f.Close()
			if err != nil {
				return nil, err
			}
			if count == 0 {
				break
			}
			first = 0
		}
	}
	return data, nil
}

func writeThumbnailSample(ctx context.Context, data []byte) (string, func(), error) {
	f, err := os.CreateTemp(utils.TempDir(), "vt-cb-sample-*.vob")
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	_, err = f.Write(data)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return name, cleanup, nil
}

// Recover each scan's VTS key from bounded menu/opening samples and reuse it.
// Keep the representative bytes even when key recovery needs opening padding.
func decryptThumbnailSampleWithKeys(ctx context.Context, videoTS string, set VobSet, info *ifo.TitleInfo, data []byte, keys map[string][5]byte) error {
	encrypted := false
	for offset := 0; offset+css.SectorSize <= len(data); offset += css.SectorSize {
		if css.IsScrambledSector(data[offset : offset+css.SectorSize]) {
			encrypted = true
			break
		}
	}
	if !encrypted {
		return ctx.Err()
	}
	if key, ok := keys[set.Name]; ok {
		return css.DecryptSampleWithKey(ctx, data, key)
	}
	f, err := os.Open(filepath.Join(videoTS, set.Name+"_0.VOB"))
	if err == nil {
		menu := make([]byte, 0, thumbnailSampleBytes)
		for len(menu) < thumbnailSampleBytes {
			if err = ctx.Err(); err != nil {
				break
			}
			sector := make([]byte, css.SectorSize)
			_, err = io.ReadFull(f, sector)
			if err != nil {
				break
			}
			menu = append(menu, sector...)
		}
		_ = f.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			key, scrambled, keyErr := css.RecoverSampleKey(ctx, menu)
			if keyErr == nil && scrambled {
				keys[set.Name] = key
				return css.DecryptSampleWithKey(ctx, data, key)
			}
		}
	}
	// Recover from opening padding when menus are absent. Keep the original
	// representative sample: the opening often contains a black lead-in.
	opening, openingErr := thumbnailCellData(ctx, videoTS, set, info, false)
	if openingErr != nil {
		return openingErr
	}
	key, scrambled, keyErr := css.RecoverSampleKey(ctx, opening)
	if keyErr == nil && scrambled {
		keys[set.Name] = key
		return css.DecryptSampleWithKey(ctx, data, key)
	}
	key, scrambled, keyErr = css.RecoverSampleKey(ctx, data)
	if keyErr != nil {
		return keyErr
	}
	if !scrambled {
		return css.ErrTitleKeyNotFound
	}
	keys[set.Name] = key
	return css.DecryptSampleWithKey(ctx, data, key)
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
