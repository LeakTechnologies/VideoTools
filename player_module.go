package main

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func (s *appState) showPlayerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.maximizeWindow()
	s.setContent(buildPlayerView(s))
}

// buildPlayerView creates the VT_Player UI
func buildPlayerView(state *appState) fyne.CanvasObject {
	playerColor := moduleColor("player")

	// Back button
	backBtn := widget.NewButton("< PLAYER", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()
	topBar := ui.TintedBar(playerColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// Instructions
	instructions := widget.NewLabel("VT_Player - Advanced video playback with frame-accurate seeking and analysis tools.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Use a stable base size; the player container handles aspect-safe scaling.
	playerSize := fyne.NewSize(640, 360)

	var videoContainer fyne.CanvasObject
	if state.playerFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.playerFile.Path)))
		videoContainer = buildVideoPane(state, playerSize, state.playerFile, nil)
	} else {
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, state.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.playerFile = src
					state.showPlayerView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Clear video button
	clearBtn := widget.NewButton("Clear Video", func() {
		state.releasePlaybackSession()
		state.stopPlayer()
		state.playerFile = nil
		state.showPlayerView()
	})
	clearBtn.Importance = widget.MediumImportance

	// Button container
	buttonContainer := container.NewHBox(loadBtn, clearBtn)

	// Main content
	mainContent := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		buttonContainer,
		videoContainer,
	)

	content := container.NewPadded(mainContent)
	bottomBar := moduleFooter(playerColor, layout.NewSpacer(), state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
