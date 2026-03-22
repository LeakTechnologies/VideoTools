//go:build native_media

package main

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func buildVideoPaneNative(state *appState, min fyne.Size, src *videoSource, onCover func(string)) fyne.CanvasObject {
	t := i18n.T()
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = ui.GridColor
	outer.StrokeWidth = 1

	defaultAspect := 16.0 / 9.0
	if src != nil && src.Width > 0 && src.Height > 0 {
		defaultAspect = float64(src.Width) / float64(src.Height)
	}
	if defaultAspect < 0.6 {
		defaultAspect = 0.6
	} else if defaultAspect > 2.4 {
		defaultAspect = 2.4
	}

	targetWidth := float32(min.Width)
	targetHeight := float32(min.Height)
	if targetWidth <= 0 {
		targetWidth = 480
	}
	if targetHeight <= 0 {
		targetHeight = 360
	}

	aspect := float32(defaultAspect)
	stageWidth := targetWidth
	stageHeight := stageWidth / aspect
	if stageHeight < targetHeight {
		stageHeight = targetHeight
		stageWidth = stageHeight * aspect
	}

	player := GetConvertPlayer()
	playerWidget := player.Widget()

	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.CornerRadius = 6

	dropIndicator := canvas.NewRectangle(color.NRGBA{R: 76, G: 175, B: 80, A: 0})
	dropIndicator.CornerRadius = 8
	dropIndicator.StrokeWidth = 3
	dropIndicator.StrokeColor = utils.MustHex("#4CE870")

	dropAnimation := fyne.NewAnimation(800*time.Millisecond, func(progress float32) {
		alpha := uint8(255 * (1 - progress))
		dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: alpha}
		dropIndicator.Refresh()
		if progress >= 1.0 {
			dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: 0}
			dropIndicator.StrokeWidth = 0
			dropIndicator.Refresh()
		}
	})
	dropAnimation.AutoReverse = true
	dropAnimation.RepeatCount = 3

	coverBtn := utils.MakeIconButton("", t.ActionSave+" Frame", func() {
		img := playerWidget.CurrentFrame()
		if img == nil {
			return
		}
		if onCover != nil {
			onCover("")
		}
	})

	saveFrameBtn := utils.MakeIconButton("", "Save current frame as PNG", func() {
		img := playerWidget.CurrentFrame()
		if img == nil {
			return
		}
		dlg := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if w == nil {
				return
			}
			defer w.Close()
			name := strings.TrimSuffix(src.DisplayName, ".avi") + "-frame.png"
			dlg.SetFileName(name)
			dlg.Show()
		}, state.window)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
	})

	importBtn := utils.MakeIconButton("", "Import cover art file", func() {
		dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			if dest, err := state.importCoverImage(path); err == nil {
				if onCover != nil {
					onCover(dest)
				}
			} else {
				dialog.ShowError(err, state.window)
			}
		}, state.window)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		dlg.Show()
	})

	dropAnimation.Start()

	stageWithPlayer := container.NewMax(bg, playerWidget)
	videoStageWithIndicator := container.NewMax(dropIndicator, stageWithPlayer)

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(formatClock(src.Duration))
	totalTime.Alignment = fyne.TextAlignTrailing

	slider := widget.NewSlider(0, math.Max(1, src.Duration))
	slider.Step = 0.5

	var updatingProgress bool
	updateProgress := func(val float64) {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			updatingProgress = true
			currentTime.SetText(formatClock(val))
			slider.SetValue(val)
			updatingProgress = false
		}, false)
	}

	frameLabel := widget.NewLabel("Frame: 0")
	frameLabel.TextStyle = fyne.TextStyle{Monospace: true}

	slider.OnChanged = func(val float64) {
		if updatingProgress {
			return
		}
		updateProgress(val)
		state.scrubNative(val)
	}

	var volIcon *widget.Button
	ensureSession := func() bool {
		return true
	}

	updateVolIcon := func() {
		if volIcon == nil {
			return
		}
		if state.playerMuted || state.playerVolume <= 0 {
			volIcon.Icon = ui.GetIcon("volume_mute")
		} else {
			volIcon.Icon = ui.GetIcon("volume_up")
		}
		volIcon.Refresh()
	}

	volIcon = widget.NewButtonWithIcon("", ui.GetIcon("volume_up"), func() {
		if !ensureSession() {
			return
		}
		if state.playerMuted || state.playerVolume <= 0 {
			target := state.lastVolume
			if target <= 0 {
				target = 50
			}
			state.playerVolume = target
			state.playerMuted = false
		} else {
			state.lastVolume = state.playerVolume
			state.playerVolume = 0
			state.playerMuted = true
		}
		updateVolIcon()
	})

	volSlider := widget.NewSlider(0, 100)
	volSlider.Step = 1
	volSlider.Value = state.playerVolume
	var updatingVolume bool
	volSlider.OnChanged = func(val float64) {
		if updatingVolume {
			return
		}
		state.playerVolume = val
		if val > 0 {
			state.lastVolume = val
			state.playerMuted = false
		} else {
			state.playerMuted = true
		}
		updateVolIcon()
	}
	updateVolIcon()
	volSlider.Refresh()

	var playBtn *widget.Button
	playBtn = widget.NewButtonWithIcon("", ui.GetIcon("play_arrow"), func() {
		if state.playerPaused {
			state.playNative()
			state.playerPaused = false
			playBtn.Icon = ui.GetIcon("pause")
		} else {
			state.pauseNative()
			state.playerPaused = true
			playBtn.Icon = ui.GetIcon("play_arrow")
		}
		playBtn.Refresh()
	})
	playBtn.Importance = widget.LowImportance

	prevFrameBtn := widget.NewButtonWithIcon("", ui.GetIcon("skip_previous"), func() {
		state.playerPaused = true
		state.pauseNative()
		state.stepFrameNative(-1)
		frameLabel.SetText(fmt.Sprintf("Frame: %d", int(playerWidget.CurrentTime()*src.FrameRate)))
	})
	prevFrameBtn.Importance = widget.LowImportance

	nextFrameBtn := widget.NewButtonWithIcon("", ui.GetIcon("skip_next"), func() {
		state.playerPaused = true
		state.pauseNative()
		state.stepFrameNative(1)
		frameLabel.SetText(fmt.Sprintf("Frame: %d", int(playerWidget.CurrentTime()*src.FrameRate)))
	})
	nextFrameBtn.Importance = widget.LowImportance

	fullBtn := utils.MakeIconButton("", "Toggle fullscreen", func() {
		if state.window == nil {
			return
		}
		state.window.SetFullScreen(!state.window.FullScreen())
	})

	replay10Btn := widget.NewButtonWithIcon("", ui.GetIcon("replay_10"), func() {
		state.seekNative(math.Max(0, slider.Value-10))
	})
	replay10Btn.Importance = widget.LowImportance

	forward10Btn := widget.NewButtonWithIcon("", ui.GetIcon("forward_10"), func() {
		state.seekNative(math.Min(src.Duration, slider.Value+10))
	})
	forward10Btn.Importance = widget.LowImportance

	volBox := container.NewHBox(volIcon, container.NewMax(volSlider))
	seekRow := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
	leftBtns := container.NewHBox(replay10Btn, prevFrameBtn, playBtn, nextFrameBtn, forward10Btn)
	rightBtns := container.NewHBox(volBox, fullBtn)
	mainCtrlRow := container.NewBorder(nil, nil, leftBtns, rightBtns, nil)

	primaryBg := canvas.NewRectangle(color.NRGBA{R: 12, G: 17, B: 31, A: 230})
	primaryBar := container.NewMax(primaryBg, container.NewPadded(container.NewVBox(seekRow, mainCtrlRow)))

	gridColor := ui.GridColor
	advancedBg := canvas.NewRectangle(utils.MustHex("#0C111F"))
	advancedBg.StrokeColor = gridColor
	advancedBg.StrokeWidth = 1

	audioTracks := player.GetAudioTracks()
	audioTrackSelect := widget.NewSelect(nil, nil)
	audioTrackSelect.Hide()
	if len(audioTracks) > 1 {
		names := make([]string, len(audioTracks))
		for i, tr := range audioTracks {
			label := tr.Language
			if tr.Title != "" {
				label = tr.Title
			}
			if label == "" {
				label = fmt.Sprintf("Track %d", i+1)
			}
			if tr.CodecName != "" {
				label += " (" + tr.CodecName + ")"
			}
			names[i] = label
		}
		audioTrackSelect.Options = names
		audioTrackSelect.SetSelected(names[0])
		audioTrackSelect.OnChanged = func(selected string) {
			for i, n := range names {
				if n == selected {
					state.selectAudioTrackNative(i)
					break
				}
			}
		}
		audioTrackSelect.Show()
	}

	frameTools := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewSeparator(), frameLabel),
		container.NewHBox(audioTrackSelect, coverBtn, saveFrameBtn, importBtn),
		nil,
	)
	advancedBar := container.NewMax(advancedBg, container.NewPadded(frameTools))

	controls := container.NewVBox(primaryBar, advancedBar)

	stack := container.NewBorder(
		nil,
		controls,
		nil, nil,
		container.NewPadded(videoStageWithIndicator),
	)

	return container.NewMax(outer, container.NewPadded(stack))
}
