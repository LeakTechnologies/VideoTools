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

// analyzeDVDStructure analyzes a DVD or ISO file for titles
func analyzeDVDStructure(path string, sourceType string) ([]DVDTitle, bool, bool) {
	// This is a placeholder implementation
	// In reality, you would use FFmpeg with DVD input support
	dialog.ShowInformation("DVD Analysis",
		fmt.Sprintf("Analyzing %s: %s\n\nThis will extract DVD structure and find all titles, audio tracks, and subtitles.", sourceType, filepath.Base(path)),
		nil)

	// Return sample titles
	return []DVDTitle{
		{
			Number:     1,
			Name:       "Main Feature",
			Duration:   7200, // 2 hours
			SizeGB:     7.8,
			VideoCodec: "MPEG-2",
			AudioTracks: []DVDTrack{
				{ID: 1, Language: "en", Codec: "AC-3", Channels: 6, SampleRate: 48000, Bitrate: 448000},
				{ID: 2, Language: "es", Codec: "AC-3", Channels: 2, SampleRate: 48000, Bitrate: 192000},
			},
			SubtitleTracks: []DVDTrack{
				{ID: 1, Language: "en", Codec: "SubRip"},
				{ID: 2, Language: "es", Codec: "SubRip"},
			},
			Chapters: []DVDChapter{
				{Number: 1, Title: "Chapter 1", StartTime: 0, Duration: 1800},
				{Number: 2, Title: "Chapter 2", StartTime: 1800, Duration: 1800},
				{Number: 3, Title: "Chapter 3", StartTime: 3600, Duration: 1800},
				{Number: 4, Title: "Chapter 4", StartTime: 5400, Duration: 1800},
			},
			AngleCount: 1,
			IsPAL:      false,
		},
	}, false, false // DVD-5 by default for this example
}

// ripTitle rips a single DVD title to MKV format
func ripTitle(title DVDTitle, state *appState) {
	// Default to AV1 in MKV for best quality
	outputPath := fmt.Sprintf("%s_%s_Title%d.mkv",
		strings.TrimSuffix(strings.TrimSuffix(filepath.Base(state.authorFile.Path), filepath.Ext(state.authorFile.Path)), ".dvd"),
		title.Name,
		title.Number)

	dialog.ShowInformation("Rip Title",
		fmt.Sprintf("Ripping Title %d: %s\n\nOutput: %s\nFormat: MKV (AV1)\nAudio: All tracks\nSubtitles: All tracks",
			title.Number, title.Name, outputPath),
		state.window)

	// TODO: Implement actual ripping with FFmpeg
	// This would use FFmpeg to extract the title with selected codec
	// For DVD: ffmpeg -i dvd://1 -c:v libaom-av1 -c:a libopus -map_metadata 0 output.mkv
	// For ISO: ffmpeg -i path/to/iso -map 0:v:0 -map 0:a -c:v libaom-av1 -c:a libopus output.mkv
}

// ripAllTitles rips all DVD titles
func ripAllTitles(titles []DVDTitle, state *appState) {
	dialog.ShowInformation("Rip All Titles",
		fmt.Sprintf("Ripping all %d titles\n\nThis will extract each title to separate MKV files with AV1 encoding.", len(titles)),
		state.window)

	// TODO: Implement batch ripping
	for _, title := range titles {
		ripTitle(title, state)
	}
}
