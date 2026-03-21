//go:build native_media

package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type upscalePlayerState struct {
	player1    *media.VideoPlayer
	player2    *media.VideoPlayer
	engine1    *media.Engine
	engine2    *media.Engine
	splitView  *media.SplitView
	sourcePath string
	outputPath string
	showSplit  bool
	playing    bool
}

var upscalePlayers = &upscalePlayerState{
	showSplit: true,
}

func upscalePlayersInit() {
	if upscalePlayers.player1 == nil {
		upscalePlayers.player1 = media.NewInlineVideoPlayer()
		upscalePlayers.player1.SetMinimal(true)
	}
	if upscalePlayers.player2 == nil {
		upscalePlayers.player2 = media.NewInlineVideoPlayer()
		upscalePlayers.player2.SetMinimal(true)
	}
	if upscalePlayers.splitView == nil {
		upscalePlayers.splitView = media.NewSplitView()
	}
}

func (s *upscalePlayerState) LoadSource(path string) error {
	s.sourcePath = path

	if s.engine1 != nil {
		s.engine1.Close()
	}
	s.engine1 = media.NewEngine()
	s.engine1.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	s.engine1.SetDropFrames(true)

	if err := s.engine1.Open(path); err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}

	s.engine1.InitFrameCache(30)
	s.player1.SetDuration(s.engine1.Duration())

	if img, err := s.engine1.NextFrame(); err == nil {
		s.player1.SetFrame(img)
	}

	return nil
}

func (s *upscalePlayerState) LoadOutput(path string) error {
	s.outputPath = path

	if s.engine2 != nil {
		s.engine2.Close()
	}
	s.engine2 = media.NewEngine()
	s.engine2.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	s.engine2.SetDropFrames(true)

	if err := s.engine2.Open(path); err != nil {
		return fmt.Errorf("failed to open output: %w", err)
	}

	s.engine2.InitFrameCache(30)
	s.player2.SetDuration(s.engine2.Duration())

	if img, err := s.engine2.NextFrame(); err == nil {
		s.player2.SetFrame(img)
	}

	return nil
}

func (s *upscalePlayerState) Play() {
	if s.engine1 == nil {
		return
	}
	s.playing = true
	s.engine1.Start()
	if s.engine2 != nil {
		s.engine2.Start()
	}
	go s.playbackLoop()
}

func (s *upscalePlayerState) Pause() {
	if s.engine1 == nil {
		return
	}
	s.playing = false
	s.engine1.Pause()
	if s.engine2 != nil {
		s.engine2.Pause()
	}
}

func (s *upscalePlayerState) Seek(target float64) {
	if s.engine1 == nil {
		return
	}
	s.engine1.Seek(target)
	if img, err := s.engine1.NextFrame(); err == nil {
		s.player1.SetFrame(img)
	}
	if s.engine2 != nil {
		s.engine2.Seek(target)
		if img, err := s.engine2.NextFrame(); err == nil {
			s.player2.SetFrame(img)
		}
	}
}

func (s *upscalePlayerState) playbackLoop() {
	defer logging.RecoverPanic()

	for s.playing {
		var frame1, frame2 *image.RGBA
		if s.engine1 != nil {
			frame1, _ = s.engine1.NextFrame()
		}
		if s.engine2 != nil {
			frame2, _ = s.engine2.NextFrame()
		}

		if s.showSplit && s.splitView != nil {
			s.splitView.SetFrames(frame1, frame2)
		} else if s.player1 != nil {
			s.player1.SetFrame(frame1)
		}

		time.Sleep(16 * time.Millisecond)
	}
}

func (s *upscalePlayerState) ToggleSplit() {
	s.showSplit = !s.showSplit
}

func (s *upscalePlayerState) Close() {
	s.playing = false
	if s.engine1 != nil {
		s.engine1.Close()
		s.engine1 = nil
	}
	if s.engine2 != nil {
		s.engine2.Close()
		s.engine2 = nil
	}
}

func BuildUpscaleVideoCompare(size fyne.Size) fyne.CanvasObject {
	upscalePlayersInit()

	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.SetMinSize(size)

	if upscalePlayers.showSplit {
		return container.NewMax(bg, upscalePlayers.splitView)
	}
	return container.NewMax(bg, upscalePlayers.player1)
}

func BuildUpscaleDualPlayerControls() fyne.CanvasObject {
	t := i18n.T()

	compareBtn := widget.NewButton("Compare Source/Output", func() {
		upscalePlayers.ToggleSplit()
	})
	compareBtn.Importance = widget.MediumImportance

	return container.NewHBox(compareBtn)
}
