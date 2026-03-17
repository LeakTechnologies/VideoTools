package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// buildDVDRipTab creates a DVD/ISO ripping tab with import support
func buildDVDRipTab(state *appState) fyne.CanvasObject {
	// DVD/ISO source
	var sourceType string // "dvd" or "iso"
	var isDVD5 bool
	var isDVD9 bool
	var titles []DVDTitle

	sourceLabel := widget.NewLabel("No DVD/ISO selected")
	sourceLabel.TextStyle = fyne.TextStyle{Bold: true}

	var updateTitleList func()
	importBtn := widget.NewButton("Import DVD/ISO", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			path := reader.URI().Path()

			if strings.ToLower(filepath.Ext(path)) == ".iso" {
				sourceType = "iso"
				sourceLabel.SetText(fmt.Sprintf("ISO: %s", filepath.Base(path)))
			} else if isDVDPath(path) {
				sourceType = "dvd"
				sourceLabel.SetText(fmt.Sprintf("DVD: %s", path))
			} else {
				dialog.ShowError(fmt.Errorf("not a valid DVD or ISO file"), state.window)
				return
			}

			// Analyze DVD/ISO
			analyzedTitles, dvd5, dvd9 := analyzeDVDStructure(path, sourceType)
			titles = analyzedTitles
			isDVD5 = dvd5
			isDVD9 = dvd9
			updateTitleList()
		}, state.window)
	})
	importBtn.Importance = widget.HighImportance

	// Title list
	titleList := container.NewVBox()

	updateTitleList = func() {
		titleList.Objects = nil

		if len(titles) == 0 {
			emptyLabel := widget.NewLabel("Import a DVD or ISO to analyze")
			emptyLabel.Alignment = fyne.TextAlignCenter
			titleList.Add(container.NewCenter(emptyLabel))
			return
		}

		// Add DVD5/DVD9 indicators
		if isDVD5 {
			dvd5Label := widget.NewLabel("🎞 DVD-5 Detected (Single Layer)")
			dvd5Label.Importance = widget.LowImportance
			titleList.Add(dvd5Label)
		}
		if isDVD9 {
			dvd9Label := widget.NewLabel("🎞 DVD-9 Detected (Dual Layer)")
			dvd9Label.Importance = widget.LowImportance
			titleList.Add(dvd9Label)
		}

		// Add titles
		for i, title := range titles {
			idx := i
			titleCard := widget.NewCard(
				fmt.Sprintf("Title %d: %s", idx+1, title.Name),
				fmt.Sprintf("%.2fs (%.1f GB)", title.Duration, title.SizeGB),
				nil,
			)

			// Title details
			details := container.NewVBox(
				widget.NewLabel(fmt.Sprintf("Duration: %.2f seconds", title.Duration)),
				widget.NewLabel(fmt.Sprintf("Size: %.1f GB", title.SizeGB)),
				widget.NewLabel(fmt.Sprintf("Video: %s", title.VideoCodec)),
				widget.NewLabel(fmt.Sprintf("Audio: %d tracks", len(title.AudioTracks))),
				widget.NewLabel(fmt.Sprintf("Subtitles: %d tracks", len(title.SubtitleTracks))),
				widget.NewLabel(fmt.Sprintf("Chapters: %d", len(title.Chapters))),
			)
			titleCard.SetContent(details)

			// Rip button for this title
			ripBtn := widget.NewButton("Rip Title", func() {
				ripTitle(title, state)
			})
			ripBtn.Importance = widget.HighImportance

			// Add to controls
			controls := container.NewVBox(details, widget.NewSeparator(), ripBtn)
			titleCard.SetContent(controls)
			titleList.Add(titleCard)
		}
	}

	// Rip all button
	ripAllBtn := widget.NewButton("Rip All Titles", func() {
		if len(titles) == 0 {
			dialog.ShowInformation("No Titles", "Please import a DVD or ISO first", state.window)
			return
		}
		ripAllTitles(titles, state)
	})
	ripAllBtn.Importance = widget.HighImportance

	controls := container.NewVBox(
		widget.NewLabel("DVD/ISO Source:"),
		sourceLabel,
		importBtn,
		widget.NewSeparator(),
		widget.NewLabel("Titles Found:"),
		container.NewScroll(titleList),
		widget.NewSeparator(),
		container.NewHBox(ripAllBtn),
	)

	return container.NewPadded(controls)
}

// DVDTitle represents a DVD title
type DVDTitle struct {
	Number         int
	Name           string
	Duration       float64
	SizeGB         float64
	VideoCodec     string
	AudioTracks    []DVDTrack
	SubtitleTracks []DVDTrack
	Chapters       []DVDChapter
	AngleCount     int
	IsPAL          bool
}

// DVDTrack represents an audio/subtitle track
type DVDTrack struct {
	ID         int
	Language   string
	Codec      string
	Channels   int
	SampleRate int
	Bitrate    int
}

// DVDChapter represents a chapter
type DVDChapter struct {
	Number    int
	Title     string
	StartTime float64
	Duration  float64
}

// isDVDPath checks if path is likely a DVD structure
func isDVDPath(path string) bool {
	// Check for VIDEO_TS directory
	videoTS := filepath.Join(path, "VIDEO_TS")
	if _, err := os.Stat(videoTS); err == nil {
		return true
	}

	// Check for common DVD file patterns
	dirs, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, dir := range dirs {
		name := strings.ToUpper(dir.Name())
		if strings.Contains(name, "VIDEO_TS") ||
			strings.Contains(name, "VTS_") {
			return true
		}
	}

	return false
}

// analyzeDVDStructure probes a DVD/ISO for title structure using ffprobe.
// Returns found titles, whether it appears to be DVD-5, and DVD-9.
func analyzeDVDStructure(path string, sourceType string) ([]DVDTitle, bool, bool) {
	// Find VOB files to probe — either from an ISO mount or a VIDEO_TS directory
	vobFiles, err := findVOBFiles(path, sourceType)
	if err != nil || len(vobFiles) == 0 {
		// Nothing to probe — return empty with no error dialog (caller shows state)
		return nil, false, false
	}

	var titles []DVDTitle
	var totalGB float64

	// Each VTS_xx_1.VOB is a separate title; group by title number
	titleMap := map[int]string{}
	for _, vob := range vobFiles {
		base := filepath.Base(vob)
		var vtsNum int
		if _, err := fmt.Sscanf(base, "VTS_%02d_1.VOB", &vtsNum); err == nil {
			titleMap[vtsNum] = vob
		}
	}

	for vtsNum := 1; vtsNum <= len(titleMap); vtsNum++ {
		vob, ok := titleMap[vtsNum]
		if !ok {
			continue
		}

		info, _ := os.Stat(vob)
		sizeGB := 0.0
		if info != nil {
			sizeGB = float64(info.Size()) / (1024 * 1024 * 1024)
			totalGB += sizeGB
		}

		// Use ffprobe to get stream info
		src, err := probeVideo(vob)
		if err != nil {
			titles = append(titles, DVDTitle{
				Number: vtsNum,
				Name:   fmt.Sprintf("Title %d", vtsNum),
				SizeGB: sizeGB,
			})
			continue
		}

		t := DVDTitle{
			Number:     vtsNum,
			Name:       fmt.Sprintf("Title %d", vtsNum),
			Duration:   src.Duration,
			SizeGB:     sizeGB,
			VideoCodec: src.VideoCodec,
			IsPAL:      src.Height == 576,
		}

		for i, a := range src.Audio {
			t.AudioTracks = append(t.AudioTracks, DVDTrack{
				ID:       i + 1,
				Language: a.Language,
				Codec:    a.Codec,
				Channels: a.Channels,
			})
		}
		for i, s := range src.Subtitles {
			t.SubtitleTracks = append(t.SubtitleTracks, DVDTrack{
				ID:       i + 1,
				Language: s.Language,
				Codec:    s.Codec,
			})
		}
		titles = append(titles, t)
	}

	isDVD5 := totalGB <= 4.7
	isDVD9 := totalGB > 4.7
	return titles, isDVD5, isDVD9
}

// findVOBFiles returns the list of VTS_xx_1.VOB files for a DVD/ISO path.
func findVOBFiles(path, sourceType string) ([]string, error) {
	var searchDir string
	if sourceType == "iso" {
		// For ISO files: look for a VIDEO_TS folder relative to path,
		// or treat the ISO as a directory if it was mounted/extracted.
		// Simple heuristic: probe the ISO directly via ffprobe (no mount needed).
		// Return the ISO itself as a pseudo-title for probing.
		return []string{path}, nil
	}
	// DVD directory: look for VIDEO_TS subdirectory
	videoTS := filepath.Join(path, "VIDEO_TS")
	if _, err := os.Stat(videoTS); err == nil {
		searchDir = videoTS
	} else {
		searchDir = path
	}
	matches, err := filepath.Glob(filepath.Join(searchDir, "VTS_*_1.VOB"))
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// ripTitle queues an FFmpeg job to extract a single DVD title to MKV.
func ripTitle(title DVDTitle, state *appState) {
	if state.authorFile == nil {
		dialog.ShowError(fmt.Errorf("no source file loaded"), state.window)
		return
	}
	srcPath := state.authorFile.Path
	base := strings.TrimSuffix(filepath.Base(srcPath), filepath.Ext(srcPath))
	outputPath := filepath.Join(filepath.Dir(srcPath), fmt.Sprintf("%s_Title%02d.mkv", base, title.Number))

	// Stream copy all audio and the first video stream — no re-encode
	args := []string{
		"-i", srcPath,
		"-map", "0:v:0",
		"-map", "0:a",
		"-c", "copy",
		"-map_metadata", "0",
		"-y", outputPath,
	}

	job := &queue.Job{
		Type:        queue.JobTypeRip,
		Title:       fmt.Sprintf("Rip: %s (Title %d)", title.Name, title.Number),
		Description: fmt.Sprintf("Extract title %d → %s", title.Number, filepath.Base(outputPath)),
		InputFile:   srcPath,
		OutputFile:  outputPath,
		Config: map[string]interface{}{
			"ffmpeg_path": utils.GetFFmpegPath(),
			"args":        args,
		},
	}

	if state.jobQueue != nil {
		state.jobQueue.Add(job)
		dialog.ShowInformation("Queued",
			fmt.Sprintf("Rip job for Title %d added to queue.\nOutput: %s", title.Number, filepath.Base(outputPath)),
			state.window)
	}
}

// ripAllTitles queues rip jobs for all DVD titles.
func ripAllTitles(titles []DVDTitle, state *appState) {
	if state.authorFile == nil {
		dialog.ShowError(fmt.Errorf("no source file loaded"), state.window)
		return
	}
	for _, title := range titles {
		ripTitle(title, state)
	}
	dialog.ShowInformation("Queued",
		fmt.Sprintf("%d rip jobs added to queue.", len(titles)),
		state.window)
}
