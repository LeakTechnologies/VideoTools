//go:build native_media

package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/inspect"
	"git.leaktechnologies.dev/stu/VideoTools/internal/interlace"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// showInspectViewForPath navigates to the inspect/player module with the given
// file pre-loaded. Used by the job queue "Play Video" button.
func (s *appState) showInspectViewForPath(path string) {
	src, err := probeVideo(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
		return
	}
	s.inspectFile = src
	s.inspectInterlaceResult = nil
	s.inspectInterlaceAnalyzing = true
	s.showInspectView()
	logging.Debug(logging.CatModule, "queue: opened in player: %s", path)

	go func() {
		if len(src.PreviewFrames) == 0 {
			if frames, ferr := capturePreviewFrames(path, src.Duration); ferr == nil && len(frames) > 0 {
				src.PreviewFrames = frames
			}
		}
		detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := detector.QuickAnalyze(ctx, path)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.inspectInterlaceAnalyzing = false
			if err != nil {
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
	s.setContent(buildInspectView(s))
}

// buildInspectView creates the UI for inspecting a single video with player
func buildInspectView(state *appState) fyne.CanvasObject {
	opts := inspect.Options{
		Window:                    state.window,
		ModuleColor:               moduleColor("inspect"),
		InspectFile:               state.inspectFile,
		InspectInterlaceAnalyzing: state.inspectInterlaceAnalyzing,
		InspectInterlaceResult:    state.inspectInterlaceResult,
		OnShowMainMenu:            func() { state.showMainMenu() },
		OnShowQueue:               func() { state.showQueue() },
		OnShowInspectView:         func() { state.showInspectView() },
		OnClearCompletedJobs:      func() { state.clearCompletedJobs() },
		OnGetStatsBar:             func() fyne.CanvasObject { return state.statsBar },
		OnOpenLogViewer:           func(title, path string, isTemp bool) { state.openLogViewer(title, path, isTemp) },
		OnLoadFile: func(path string) {
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}
			state.inspectFile = src
			state.inspectInterlaceResult = nil
			state.inspectInterlaceAnalyzing = true
			state.showInspectView()
			logging.Debug(logging.CatModule, "loaded inspect file: %s", path)

			go func() {
				// Capture first frame before interlace so it's ready when view refreshes
				if len(src.PreviewFrames) == 0 {
					if frames, ferr := capturePreviewFrames(path, src.Duration); ferr == nil && len(frames) > 0 {
						src.PreviewFrames = frames
					}
				}

				detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := detector.QuickAnalyze(ctx, path)

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.inspectInterlaceAnalyzing = false
					if err != nil {
						logging.Debug(logging.CatSystem, "auto interlacing analysis failed: %v", err)
						state.inspectInterlaceResult = nil
					} else {
						state.inspectInterlaceResult = result
						logging.Debug(logging.CatSystem, "auto interlacing analysis complete: %s", result.Status)
					}
					state.showInspectView()
				}, false)
			}()
		},
		OnClearFile: func() {
			state.inspectFile = nil
		},
		OnGetFormat:       func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.Format },
		OnGetVideoCodec:   func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.VideoCodec },
		OnGetWidth:        func() int { if state.inspectFile == nil { return 0 }; return state.inspectFile.Width },
		OnGetHeight:       func() int { if state.inspectFile == nil { return 0 }; return state.inspectFile.Height },
		OnGetAspectRatio:  func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.AspectRatioString() },
		OnGetFrameRate:    func() float64 { if state.inspectFile == nil { return 0 }; return state.inspectFile.FrameRate },
		OnGetBitrate:      func() int64 { if state.inspectFile == nil { return 0 }; return int64(state.inspectFile.Bitrate) },
		OnGetPixelFormat:  func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.PixelFormat },
		OnGetColorSpace:   func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.ColorSpace },
		OnGetColorRange:   func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.ColorRange },
		OnGetFieldOrder:   func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.FieldOrder },
		OnGetGOPSize:      func() int { if state.inspectFile == nil { return 0 }; return state.inspectFile.GOPSize },
		OnGetAudioCodec:   func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.AudioCodec },
		OnGetAudioBitrate: func() int64 { if state.inspectFile == nil { return 0 }; return int64(state.inspectFile.AudioBitrate) },
		OnGetAudioRate:    func() int { if state.inspectFile == nil { return 0 }; return state.inspectFile.AudioRate },
		OnGetChannels:     func() int { if state.inspectFile == nil { return 0 }; return state.inspectFile.Channels },
		OnGetDuration:     func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.DurationString() },
		OnGetSampleAspect: func() string { if state.inspectFile == nil { return "" }; return state.inspectFile.SampleAspectRatio },
		OnGetHasChapters:  func() bool { if state.inspectFile == nil { return false }; return state.inspectFile.HasChapters },
		OnGetHasMetadata:  func() bool { if state.inspectFile == nil { return false }; return state.inspectFile.HasMetadata },
		OnGetTitle: func() string {
			if state.inspectFile == nil || state.inspectFile.Metadata == nil {
				return ""
			}
			return state.inspectFile.Metadata["title"]
		},
		OnGetPreviewFrame: func() string {
			if state.inspectFile == nil || len(state.inspectFile.PreviewFrames) == 0 {
				return ""
			}
			return state.inspectFile.PreviewFrames[0]
		},
		OnGetFilePath: func() string {
			if state.inspectFile == nil {
				return ""
			}
			return state.inspectFile.Path
		},
	}
	return inspect.BuildView(opts)
}
