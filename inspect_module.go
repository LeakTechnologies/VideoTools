//go:build native_media

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modules/inspect"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/interlace"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

func (s *appState) showInspectViewForPath(path string) {
	s.recentFiles.Add(path, filepath.Base(path), "inspect")
	// Show the view immediately — probe runs in the background so the UI doesn't freeze.
	s.inspectFile = nil
	s.inspectInterlaceResult = nil
	s.inspectInterlaceAnalyzing = true
	s.showInspectView()
	logging.Debug(logging.CatModule, "queue: opening in inspect: %s", path)

	go func() {
		src, err := probeVideo(path)
		if err != nil {
			logging.Error(logging.CatInspect, "inspect probe failed: path=%s err=%v", path, err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.inspectInterlaceAnalyzing = false
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
			}, false)
			return
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.inspectFile = src
			s.showInspectView()
		}, false)

		// Start native player load now that probe has the file metadata.
		// This runs off the main goroutine; the player widget is already embedded
		// in the view and will update itself when the engine is ready.
		if err := GetInspectPlayer().Load(path); err != nil {
			logging.Error(logging.CatPlayer, "inspect player load failed: %v", err)
		}

		detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, intErr := detector.QuickAnalyze(ctx, path)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.inspectInterlaceAnalyzing = false
			if intErr != nil {
				s.inspectInterlaceResult = nil
			} else {
				s.inspectInterlaceResult = result
			}
			s.showInspectView()
		}, false)
	}()
}

func (s *appState) showInspectView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "inspect"
	s.maximizeWindow()
	s.setContent(inspect.BuildView(&inspectAdapter{s: s}))
}

type inspectAdapter struct {
	s *appState
}

func (a *inspectAdapter) Window() fyne.Window {
	return a.s.window
}

func (a *inspectAdapter) ShowMainMenu() {
	a.s.showMainMenu()
}

func (a *inspectAdapter) ShowQueue() {
	a.s.showQueue()
}

func (a *inspectAdapter) ShowInspectView() {
	a.s.showInspectView()
}

func (a *inspectAdapter) ClearCompletedJobs() {
	a.s.clearCompletedJobs()
}

func (a *inspectAdapter) StatsBar() fyne.CanvasObject {
	return a.s.statsBar
}

func (a *inspectAdapter) OpenLogViewer(title string, path string, isTemp bool) {
	a.s.openLogViewer(title, path, isTemp)
}

func (a *inspectAdapter) LoadFile(path string) {
	// Show view immediately with loading state — probeVideo blocks on ffprobe and must
	// not run on the main goroutine or the UI will freeze.
	a.s.inspectFile = nil
	a.s.inspectInterlaceResult = nil
	a.s.inspectInterlaceAnalyzing = true
	a.s.showInspectView()
	logging.Info(logging.CatInspect, "inspect: loading file: %s", path)

	go func() {
		src, err := probeVideo(path)
		if err != nil {
			logging.Error(logging.CatInspect, "inspect probe failed: path=%s err=%v", path, err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				a.s.inspectInterlaceAnalyzing = false
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), a.s.window)
			}, false)
			return
		}
		logging.Info(logging.CatInspect, "inspect: probe complete, loading player")
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			a.s.inspectFile = src
			a.s.showInspectView()
		}, false)

		// Start native player load now that probe has the file metadata.
		if err := GetInspectPlayer().Load(path); err != nil {
			logging.Error(logging.CatPlayer, "inspect player load failed: %v", err)
		}

		detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, intErr := detector.QuickAnalyze(ctx, path)

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			a.s.inspectInterlaceAnalyzing = false
			if intErr != nil {
				logging.Debug(logging.CatSystem, "auto interlacing analysis failed: %v", intErr)
				a.s.inspectInterlaceResult = nil
			} else {
				a.s.inspectInterlaceResult = result
				logging.Debug(logging.CatSystem, "auto interlacing analysis complete: %s", result.Status)
			}
			a.s.showInspectView()
		}, false)
	}()
}

func (a *inspectAdapter) ClearFile() {
	a.s.inspectFile = nil
}

func (a *inspectAdapter) GetFormat() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.Format
}

func (a *inspectAdapter) GetVideoCodec() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.VideoCodec
}

func (a *inspectAdapter) GetWidth() int {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.Width
}

func (a *inspectAdapter) GetHeight() int {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.Height
}

func (a *inspectAdapter) GetAspectRatio() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.AspectRatioString()
}

func (a *inspectAdapter) GetFrameRate() float64 {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.FrameRate
}

func (a *inspectAdapter) GetBitrate() int64 {
	if a.s.inspectFile == nil {
		return 0
	}
	return int64(a.s.inspectFile.Bitrate)
}

func (a *inspectAdapter) GetPixelFormat() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.PixelFormat
}

func (a *inspectAdapter) GetColorSpace() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.ColorSpace
}

func (a *inspectAdapter) GetColorRange() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.ColorRange
}

func (a *inspectAdapter) GetFieldOrder() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.FieldOrder
}

func (a *inspectAdapter) GetGOPSize() int {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.GOPSize
}

func (a *inspectAdapter) GetAudioCodec() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.AudioCodec
}

func (a *inspectAdapter) GetAudioBitrate() int64 {
	if a.s.inspectFile == nil {
		return 0
	}
	return int64(a.s.inspectFile.AudioBitrate)
}

func (a *inspectAdapter) GetAudioRate() int {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.AudioRate
}

func (a *inspectAdapter) GetChannels() int {
	if a.s.inspectFile == nil {
		return 0
	}
	return a.s.inspectFile.Channels
}

func (a *inspectAdapter) GetDuration() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.DurationString()
}

func (a *inspectAdapter) GetSampleAspect() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.SampleAspectRatio
}

func (a *inspectAdapter) GetHasChapters() bool {
	if a.s.inspectFile == nil {
		return false
	}
	return a.s.inspectFile.HasChapters
}

func (a *inspectAdapter) GetChapters() []inspect.Chapter {
	if a.s.inspectFile == nil {
		return nil
	}
	chapters := make([]inspect.Chapter, len(a.s.inspectFile.Chapters))
	for i, ch := range a.s.inspectFile.Chapters {
		chapters[i] = inspect.Chapter{
			Index:     ch.Index,
			StartTime: ch.StartTime,
			EndTime:   ch.EndTime,
			Title:     ch.Title,
		}
	}
	return chapters
}

func (a *inspectAdapter) GetColorTransfer() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.ColorTransfer
}

func (a *inspectAdapter) GetColorPrimaries() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.ColorPrimaries
}

func (a *inspectAdapter) GetEmbeddedCoverArt() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.EmbeddedCoverArt
}

func (a *inspectAdapter) SaveMetadata(title, author, description string) error {
	if a.s.inspectFile == nil {
		return fmt.Errorf("no file loaded")
	}
	inputPath := a.s.inspectFile.Path

	tmpPath := inputPath + ".tmp"

	args := []string{
		"-i", inputPath,
		"-c", "copy",
		"-metadata", fmt.Sprintf("title=%s", title),
		"-metadata", fmt.Sprintf("artist=%s", author),
		"-metadata", fmt.Sprintf("description=%s", description),
		"-y",
		tmpPath,
	}

	cmd := utils.CreateCommand(context.Background(), utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	if err := os.Rename(tmpPath, inputPath); err != nil {
		return fmt.Errorf("failed to replace file: %w", err)
	}

	a.s.inspectFile.Metadata["title"] = title
	a.s.inspectFile.Metadata["artist"] = author
	a.s.inspectFile.Metadata["description"] = description

	return nil
}

func (a *inspectAdapter) GetHasMetadata() bool {
	if a.s.inspectFile == nil {
		return false
	}
	return a.s.inspectFile.HasMetadata
}

func (a *inspectAdapter) GetTitle() string {
	if a.s.inspectFile == nil || a.s.inspectFile.Metadata == nil {
		return ""
	}
	return a.s.inspectFile.Metadata["title"]
}

func (a *inspectAdapter) GetPreviewFrame() string {
	if a.s.inspectFile == nil || len(a.s.inspectFile.PreviewFrames) == 0 {
		return ""
	}
	return a.s.inspectFile.PreviewFrames[0]
}

func (a *inspectAdapter) GetFilePath() string {
	if a.s.inspectFile == nil {
		return ""
	}
	return a.s.inspectFile.Path
}

func (a *inspectAdapter) Player() *ui.InlineVideoPlayer {
	return GetInspectPlayer()
}

func (a *inspectAdapter) GetClockTime() float64 {
	return GetInspectPlayer().GetClockTime()
}

func (a *inspectAdapter) GetLastVideoPTS() float64 {
	return GetInspectPlayer().GetLastVideoPTS()
}

func (a *inspectAdapter) GetLastAudioPTS() float64 {
	return GetInspectPlayer().GetLastAudioPTS()
}

func (a *inspectAdapter) Clipboard() fyne.Clipboard {
	return a.s.window.Clipboard()
}

func (a *inspectAdapter) ModuleFooter(content fyne.CanvasObject) fyne.CanvasObject {
	return moduleFooter(utils.MustHex(inspect.ModuleColor), content, a.s.statsBar)
}
