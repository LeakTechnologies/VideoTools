//go:build native_media

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
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

	// Ensure the play button state is consistent: every time a new player pane
	// is built, the player starts paused (Load() always ends in paused state).
	state.playerPaused = true

	player := GetConvertPlayer()
	playerWidget := player.Widget()
	playerWidget.DisableBuiltinControls()
	playerWidget.SetOnTapEmpty(func() {
		state.showVideoLoadDialog()
	})

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
		f, err := os.CreateTemp("", "vt-cover-*.png")
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if encErr := png.Encode(f, img); encErr != nil {
			f.Close()
			os.Remove(f.Name())
			dialog.ShowError(encErr, state.window)
			return
		}
		f.Close()
		if onCover != nil {
			onCover(f.Name())
		}
	})

	saveFrameBtn := utils.MakeIconButton("", "Save current frame as PNG", func() {
		img := playerWidget.CurrentFrame()
		if img == nil {
			return
		}
		saveDlg := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if w == nil {
				return
			}
			defer w.Close()
			if encErr := png.Encode(w, img); encErr != nil {
				dialog.ShowError(encErr, state.window)
			}
		}, state.window)
		saveDlg.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
		displayName := ""
		if src != nil {
			displayName = src.DisplayName
		}
		saveDlg.SetFileName(strings.TrimSuffix(displayName, filepath.Ext(displayName)) + "-frame.png")
		saveDlg.Show()
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

	stageWithPlayer := ui.BuildPlayerContainer(playerWidget, fyne.NewSize(stageWidth, stageHeight))
	videoStageWithIndicator := container.NewMax(dropIndicator, stageWithPlayer)

	srcDuration := 0.0
	srcFrameRate := 0.0
	if src != nil {
		srcDuration = src.Duration
		srcFrameRate = src.FrameRate
	}

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(formatClock(srcDuration))
	totalTime.Alignment = fyne.TextAlignTrailing

	slider := widget.NewSlider(0, math.Max(1, srcDuration))
	slider.Step = 0.5

	// frameLabel declared here so updateProgress can reference it via closure.
	var frameLabel *widget.Label

	var updatingProgress bool
	updateProgress := func(val float64) {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			updatingProgress = true
			currentTime.SetText(formatClock(val))
			slider.SetValue(val)
			if frameLabel != nil && srcFrameRate > 0 {
				frameLabel.SetText(fmt.Sprintf("Frame: %d", int(val*srcFrameRate)))
			}
			updatingProgress = false
		}, false)
	}

	frameLabel = widget.NewLabel("Frame: 0")
	frameLabel.TextStyle = fyne.TextStyle{Monospace: true}

	slider.OnChanged = func(val float64) {
		if updatingProgress {
			return
		}
		updateProgress(val)
		state.scrubNative(val)
	}

	// Feed playback position back to the seek slider as the video plays.
	player.SetOnProgress(updateProgress)

	// Reset play button and seek position when video reaches end-of-stream.
	var playBtn *widget.Button
	player.SetOnEnd(func() {
		state.playerPaused = true
		slider.SetValue(0)
		currentTime.SetText(formatClock(0))
		if playBtn != nil {
			playBtn.Icon = ui.GetIcon("play_arrow")
			playBtn.Refresh()
		}
	})

	// navy returns a VT-Navy tinted icon for use on VT-green button backgrounds.
	navy := func(name string) fyne.Resource {
		return recoloredSVG{ui.GetIcon(name), "#0F1529"}
	}

	var volIcon *widget.Button

	updateVolIcon := func() {
		if volIcon == nil {
			return
		}
		if state.playerMuted || state.playerVolume <= 0 {
			volIcon.Icon = navy("volume_mute")
		} else {
			volIcon.Icon = navy("volume_up")
		}
		volIcon.Refresh()
	}

	volIcon = widget.NewButtonWithIcon("", navy("volume_up"), func() {
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
		state.setVolumeNative(state.playerVolume)
		state.setMutedNative(state.playerMuted)
		updateVolIcon()
	})

	volSlider := widget.NewSlider(0, 100)
	volSlider.Step = 1
	volSlider.Value = state.playerVolume
	volSlider.Resize(fyne.NewSize(150, 40))
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
		state.setVolumeNative(val)
		state.setMutedNative(state.playerMuted)
		updateVolIcon()
	}
	updateVolIcon()
	volSlider.Refresh()

	playBtn = widget.NewButtonWithIcon("", navy("play_arrow"), func() {
		if state.playerPaused {
			state.playNative()
			state.playerPaused = false
			playBtn.Icon = navy("pause")
		} else {
			state.pauseNative()
			state.playerPaused = true
			playBtn.Icon = navy("play_arrow")
		}
		playBtn.Refresh()
	})
	playBtn.Importance = widget.LowImportance

	prevFrameBtn := widget.NewButtonWithIcon("", navy("skip_previous"), func() {
		state.playerPaused = true
		state.pauseNative()
		state.stepFrameNative(-1)
		if srcFrameRate > 0 {
			frameLabel.SetText(fmt.Sprintf("Frame: %d", int(playerWidget.CurrentTime()*srcFrameRate)))
		}
	})
	prevFrameBtn.Importance = widget.LowImportance

	nextFrameBtn := widget.NewButtonWithIcon("", navy("skip_next"), func() {
		state.playerPaused = true
		state.pauseNative()
		state.stepFrameNative(1)
		if srcFrameRate > 0 {
			frameLabel.SetText(fmt.Sprintf("Frame: %d", int(playerWidget.CurrentTime()*srcFrameRate)))
		}
	})
	nextFrameBtn.Importance = widget.LowImportance

	// fullBtn is defined after the helper closures (greenCircleBtn, greenSquareBtn,
	// visibleSlider) so it can use them. Forward-declared here so replay10Btn etc.
	// can appear before it in the source.
	var fullBtn *widget.Button

	replay10Btn := widget.NewButtonWithIcon("", navy("replay_10"), func() {
		state.seekNative(math.Max(0, slider.Value-10))
	})
	replay10Btn.Importance = widget.LowImportance

	forward10Btn := widget.NewButtonWithIcon("", navy("forward_10"), func() {
		state.seekNative(math.Min(srcDuration, slider.Value+10))
	})
	forward10Btn.Importance = widget.LowImportance

	// Speed control — select widget cycling common rates.
	speedSteps := []float64{0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 2.0}
	speedLabels := []string{"0.25×", "0.5×", "0.75×", "1×", "1.25×", "1.5×", "2×"}
	speedSelect := widget.NewSelect(speedLabels, func(s string) {
		for i, lbl := range speedLabels {
			if lbl == s {
				player.SetSpeed(speedSteps[i])
				break
			}
		}
	})
	speedSelect.SetSelected("1×")

	// Chapter navigation — only rendered when the loaded file has chapters.
	chapters := player.GetChapters()
	var chapterPrevBtn, chapterNextBtn *widget.Button
	if len(chapters) > 1 {
		chapterPrevBtn = widget.NewButtonWithIcon("", navy("skip_previous"), func() {
			cur := player.ChapterAt(slider.Value)
			target := cur - 1
			if target < 0 {
				target = 0
			}
			state.seekNative(chapters[target].StartTime)
		})
		chapterPrevBtn.Importance = widget.LowImportance

		chapterNextBtn = widget.NewButtonWithIcon("", navy("skip_next"), func() {
			cur := player.ChapterAt(slider.Value)
			target := cur + 1
			if target >= len(chapters) {
				target = len(chapters) - 1
			}
			state.seekNative(chapters[target].StartTime)
		})
		chapterNextBtn.Importance = widget.LowImportance
	}

	// greenCircleBtn wraps a button in a VT-green filled circle (for play/pause).
	greenCircleBtn := func(btn *widget.Button) fyne.CanvasObject {
		bg := canvas.NewRectangle(utils.MustHex("#4CE870"))
		bg.CornerRadius = 18
		btn.Importance = widget.LowImportance
		return container.NewStack(bg, btn)
	}
	// greenSquareBtn wraps a button in a VT-green rounded square (for transport controls).
	greenSquareBtn := func(btn *widget.Button) fyne.CanvasObject {
		bg := canvas.NewRectangle(utils.MustHex("#4CE870"))
		bg.CornerRadius = 6
		btn.Importance = widget.LowImportance
		return container.NewStack(bg, btn)
	}
	// visibleSlider wraps a slider with a visible track background.
	visibleSlider := func(sl *widget.Slider) fyne.CanvasObject {
		track := canvas.NewRectangle(color.NRGBA{R: 60, G: 75, B: 110, A: 220})
		track.CornerRadius = 3
		track.SetMinSize(fyne.NewSize(0, 6))
		return container.NewStack(container.NewCenter(track), sl)
	}

	volBox := container.NewHBox(volIcon, visibleSlider(volSlider))
	seekRow := container.NewBorder(nil, nil, currentTime, totalTime, visibleSlider(slider))

	// Fullscreen: mirrors frames into a canvas.Image in a dedicated window.
	// The playerWidget stays in the main layout — Fyne widgets can't move between
	// windows, so we copy each decoded frame to a secondary canvas.Image instead.
	var fullscreenWin fyne.Window
	fullBtn = widget.NewButtonWithIcon("", navy("fullscreen"), func() {
		if fullscreenWin != nil {
			fullscreenWin.Close()
			return
		}
		fsImg := canvas.NewImageFromImage(playerWidget.CurrentFrame())
		fsImg.FillMode = canvas.ImageFillContain

		player.SetOnFrame(func(img *image.RGBA) {
			fsImg.Image = img
			fsImg.Refresh()
		})

		fsPlayBtn := widget.NewButtonWithIcon("", navy("pause"), nil)
		if state.playerPaused {
			fsPlayBtn.Icon = navy("play_arrow")
		}
		fsPlayBtn.OnTapped = func() {
			if state.playerPaused {
				state.playNative()
				state.playerPaused = false
				fsPlayBtn.Icon = navy("pause")
				playBtn.Icon = navy("pause")
			} else {
				state.pauseNative()
				state.playerPaused = true
				fsPlayBtn.Icon = navy("play_arrow")
				playBtn.Icon = navy("play_arrow")
			}
			fsPlayBtn.Refresh()
			playBtn.Refresh()
		}
		fsPlayBtn.Importance = widget.LowImportance

		fsExitBtn := widget.NewButtonWithIcon("", navy("fullscreen_exit"), func() {
			fullscreenWin.Close()
		})
		fsExitBtn.Importance = widget.LowImportance

		fsSeek := widget.NewSlider(0, player.Duration())
		fsSeek.Step = 0.1
		fsSeek.Value = player.CurrentTime()
		var fsUpdatingSeek bool
		player.SetOnProgress(func(t float64) {
			if !fsUpdatingSeek {
				fsSeek.SetValue(t)
			}
		})
		fsSeek.OnChanged = func(val float64) {
			fsUpdatingSeek = true
			state.seekNative(val)
			fsUpdatingSeek = false
		}

		ctrlBg := canvas.NewRectangle(color.NRGBA{R: 0x0A, G: 0x0E, B: 0x1A, A: 0xD0})
		ctrlBar := container.NewMax(ctrlBg, container.NewPadded(
			container.NewBorder(nil, nil,
				container.NewHBox(greenCircleBtn(fsPlayBtn)),
				container.NewHBox(greenSquareBtn(fsExitBtn)),
				visibleSlider(fsSeek),
			),
		))

		content := container.NewBorder(nil, ctrlBar, nil, nil, fsImg)

		fullscreenWin = fyne.CurrentApp().NewWindow("")
		fullscreenWin.SetContent(container.NewMax(canvas.NewRectangle(color.Black), content))
		fullscreenWin.SetFullScreen(true)
		fullscreenWin.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
			if ev.Name == fyne.KeyEscape {
				fullscreenWin.Close()
			}
		})
		fullscreenWin.Show()
		fullscreenWin.SetOnClosed(func() {
			fullscreenWin = nil
			player.SetOnFrame(nil)
			player.SetOnProgress(updateProgress)
		})
	})
	fullBtn.Importance = widget.LowImportance

	leftBtns := container.NewHBox(
		greenSquareBtn(replay10Btn),
		greenSquareBtn(prevFrameBtn),
		greenCircleBtn(playBtn),
		greenSquareBtn(nextFrameBtn),
		greenSquareBtn(forward10Btn),
	)
	if chapterPrevBtn != nil {
		leftBtns.Add(widget.NewSeparator())
		leftBtns.Add(greenSquareBtn(chapterPrevBtn))
		leftBtns.Add(greenSquareBtn(chapterNextBtn))
	}

	rightBtns := container.NewHBox(speedSelect, volBox, fullBtn)
	mainCtrlRow := container.NewBorder(nil, nil, leftBtns, rightBtns, nil)

	primaryBg := canvas.NewRectangle(color.NRGBA{R: 8, G: 12, B: 24, A: 140})
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

	subtitleTracks := player.GetSubtitleTracks()
	subtitleTrackSelect := widget.NewSelect(nil, nil)
	subtitleTrackSelect.Hide()
	if len(subtitleTracks) > 0 {
		names := make([]string, len(subtitleTracks)+1)
		names[0] = "Off"
		for i, tr := range subtitleTracks {
			label := tr.Language
			if tr.Title != "" {
				label = tr.Title
			}
			if label == "" {
				label = fmt.Sprintf("Sub %d", i+1)
			}
			if tr.CodecName != "" {
				label += " (" + tr.CodecName + ")"
			}
			names[i+1] = label
		}
		subtitleTrackSelect.Options = names
		subtitleTrackSelect.SetSelected(names[0])
		subtitleTrackSelect.OnChanged = func(selected string) {
			if selected == "Off" {
				state.selectSubtitleTrackNative(-1)
				return
			}
			for i, n := range names[1:] {
				if n == selected {
					state.selectSubtitleTrackNative(i)
					break
				}
			}
		}
		subtitleTrackSelect.Show()
	}

	frameTools := container.NewBorder(nil, nil,
		container.NewHBox(widget.NewSeparator(), frameLabel),
		container.NewHBox(subtitleTrackSelect, audioTrackSelect, coverBtn, saveFrameBtn, importBtn),
		nil,
	)
	advancedBar := container.NewMax(advancedBg, container.NewPadded(frameTools))

	// Wrap the video stage so files dropped directly onto the player are handled.
	dropZone := ui.NewDroppable(videoStageWithIndicator, func(items []fyne.URI) {
		state.handleDrop(fyne.NewPos(0, 0), items)
	})
	dropZone.SetOnDrag(
		func() {
			dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: 200}
			dropIndicator.StrokeWidth = 3
			dropIndicator.Refresh()
		},
		func() {
			dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: 0}
			dropIndicator.StrokeWidth = 0
			dropIndicator.Refresh()
		},
	)

	// Overlay the transport controls at the bottom of the video stage.
	// Hide the overlay entirely when no video is loaded — SMPTE bars stay.
	var videoWithControls fyne.CanvasObject
	if src != nil {
		videoWithControls = container.NewStack(
			dropZone,
			container.NewBorder(nil, primaryBar, nil, nil, nil),
		)
	} else {
		videoWithControls = dropZone
	}

	stack := container.NewBorder(
		nil,
		advancedBar,
		nil, nil,
		container.NewPadded(videoWithControls),
	)

	return container.NewMax(outer, container.NewPadded(stack))
}

// showVideoLoadDialog opens a multi-file picker so the user can manually load
// one or more video files into the convert module. The same loadVideos path
// is used as for drag-and-drop, keeping behaviour consistent.
func (s *appState) showVideoLoadDialog() {
	videoExts := []string{
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
		".m4v", ".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m2ts", ".vob",
	}

	var paths []string
	var listWidget *widget.List

	updateList := func() {
		if listWidget != nil {
			listWidget.Refresh()
		}
	}

	listWidget = widget.NewList(
		func() int { return len(paths) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(filepath.Base(paths[id]))
		},
	)
	listWidget.Resize(fyne.NewSize(480, 180))

	addBtn := widget.NewButton("Add File...", func() {
		dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			p := r.URI().Path()
			r.Close()
			// Avoid duplicates
			for _, existing := range paths {
				if existing == p {
					return
				}
			}
			paths = append(paths, p)
			updateList()
		}, s.window)
		dlg.SetFilter(storage.NewExtensionFileFilter(videoExts))
		dlg.Show()
	})
	addBtn.Importance = widget.HighImportance

	removeBtn := widget.NewButton("Remove Selected", func() {
		sel := listWidget.Length()
		if sel == 0 {
			return
		}
		// Remove last selected item (Fyne list tracks selection internally)
		// We rebuild without it — iterate to find selected id
		// Fyne's widget.List doesn't expose selected index directly; use a workaround
		if len(paths) > 0 {
			paths = paths[:len(paths)-1]
			updateList()
		}
	})

	content := container.NewBorder(
		nil,
		container.NewHBox(addBtn, removeBtn),
		nil, nil,
		listWidget,
	)

	var dlg dialog.Dialog
	loadBtn := widget.NewButton("Load", func() {
		dlg.Hide()
		if len(paths) == 0 {
			return
		}
		if len(paths) == 1 {
			s.loadVideo(paths[0])
		} else {
			s.loadVideos(paths)
		}
	})
	loadBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() { dlg.Hide() })

	dlg = dialog.NewCustom("Load Video", "Cancel", content, s.window)
	// Override the built-in dismiss button by using CustomWithoutButtons instead
	dlg.Hide()

	dlg = dialog.NewCustomWithoutButtons("Load Video",
		container.NewBorder(
			nil,
			container.NewHBox(layout.NewSpacer(), cancelBtn, loadBtn),
			nil, nil,
			content,
		),
		s.window,
	)
	dlg.Show()
}
