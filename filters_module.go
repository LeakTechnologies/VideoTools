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

func (s *appState) showFiltersView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "filters"
	s.setContent(buildFiltersView(s))
}

func buildFiltersView(state *appState) fyne.CanvasObject {
	filtersColor := moduleColor("filters")

	// Back button
	backBtn := widget.NewButton("< FILTERS", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	// Top bar with module color
	topBar := ui.TintedBar(filtersColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(filtersColor, layout.NewSpacer(), state.statsBar)

	// Instructions
	instructions := widget.NewLabel("Apply filters and color corrections to your video. Preview changes in real-time.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Initialize state defaults
	if state.filterBrightness == 0 && state.filterContrast == 0 && state.filterSaturation == 0 {
		state.filterBrightness = 0.0 // -1.0 to 1.0
		state.filterContrast = 1.0   // 0.0 to 3.0
		state.filterSaturation = 1.0 // 0.0 to 3.0
		state.filterSharpness = 0.0  // 0.0 to 5.0
		state.filterDenoise = 0.0    // 0.0 to 10.0
	}
	if state.filterInterpPreset == "" {
		state.filterInterpPreset = "Balanced"
	}
	if state.filterInterpFPS == "" {
		state.filterInterpFPS = "60"
	}

	buildFilterChain := func() {
		var chain []string
		if state.filterInterpEnabled {
			fps := state.filterInterpFPS
			if fps == "" {
				fps = "60"
			}
			var filter string
			switch state.filterInterpPreset {
			case "Ultra Fast":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=blend", fps)
			case "Fast":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=duplicate", fps)
			case "High Quality":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1:search_param=32", fps)
			case "Maximum Quality":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1:search_param=64", fps)
			default: // Balanced
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=obmc:me_mode=bidir:me=epzs:search_param=16:vsbmc=0", fps)
			}
			chain = append(chain, filter)
		}
		state.filterActiveChain = chain
	}

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.filtersFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.filtersFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.filtersFile, nil)
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
					state.filtersFile = src
					state.showFiltersView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Navigation to Upscale module
	upscaleNavBtn := widget.NewButton("Send to Upscale →", func() {
		if state.filtersFile != nil {
			state.upscaleFile = state.filtersFile
			buildFilterChain()
			state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
		}
		state.showUpscaleView()
	})

	// Color Correction Section
	colorSection := widget.NewCard("Color Correction", "", container.NewVBox(
		widget.NewLabel("Adjust brightness, contrast, and saturation"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Brightness:"),
			widget.NewSlider(-1.0, 1.0),
			widget.NewLabel("Contrast:"),
			widget.NewSlider(0.0, 3.0),
			widget.NewLabel("Saturation:"),
			widget.NewSlider(0.0, 3.0),
		),
	))

	// Enhancement Section
	enhanceSection := widget.NewCard("Enhancement", "", container.NewVBox(
		widget.NewLabel("Sharpen, blur, and denoise"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Sharpness:"),
			widget.NewSlider(0.0, 5.0),
			widget.NewLabel("Denoise:"),
			widget.NewSlider(0.0, 10.0),
		),
	))

	// Transform Section
	transformSection := widget.NewCard("Transform", "", container.NewVBox(
		widget.NewLabel("Rotate and flip video"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Rotation:"),
			widget.NewSelect([]string{"0°", "90°", "180°", "270°"}, func(s string) {}),
			widget.NewLabel("Flip Horizontal:"),
			widget.NewCheck("", func(b bool) { state.filterFlipH = b }),
			widget.NewLabel("Flip Vertical:"),
			widget.NewCheck("", func(b bool) { state.filterFlipV = b }),
		),
	))

	// Creative Effects Section
	creativeSection := widget.NewCard("Creative Effects", "", container.NewVBox(
		widget.NewLabel("Apply artistic effects"),
		widget.NewCheck("Grayscale", func(b bool) { state.filterGrayscale = b }),
	))

	// Frame Interpolation Section
	interpEnabledCheck := widget.NewCheck("Enable Frame Interpolation", func(checked bool) {
		state.filterInterpEnabled = checked
		buildFilterChain()
	})
	interpEnabledCheck.SetChecked(state.filterInterpEnabled)

	interpPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(val string) {
		state.filterInterpPreset = val
		buildFilterChain()
	})
	interpPresetSelect.SetSelected(state.filterInterpPreset)

	interpFPSSelect := widget.NewSelect([]string{"24", "30", "50", "59.94", "60"}, func(val string) {
		state.filterInterpFPS = val
		buildFilterChain()
	})
	interpFPSSelect.SetSelected(state.filterInterpFPS)

	interpHint := widget.NewLabel("Balanced preset is recommended; higher presets are CPU-intensive.")
	interpHint.TextStyle = fyne.TextStyle{Italic: true}
	interpHint.Wrapping = fyne.TextWrapWord

	interpSection := widget.NewCard("Frame Interpolation (Minterpolate)", "", container.NewVBox(
		widget.NewLabel("Generate smoother motion by interpolating new frames"),
		interpEnabledCheck,
		container.NewGridWithColumns(2,
			widget.NewLabel("Preset:"),
			interpPresetSelect,
			widget.NewLabel("Target FPS:"),
			interpFPSSelect,
		),
		interpHint,
	))
	buildFilterChain()

	// Apply button
	applyBtn := widget.NewButton("Apply Filters", func() {
		if state.filtersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", state.window)
			return
		}
		buildFilterChain()
		dialog.ShowInformation("Filters", "Filters are now configured and will be applied when sent to Upscale.", state.window)
	})
	applyBtn.Importance = widget.HighImportance

	// Main content
	leftPanel := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		loadBtn,
		upscaleNavBtn,
	)

	settingsPanel := container.NewVBox(
		colorSection,
		enhanceSection,
		transformSection,
		interpSection,
		creativeSection,
		applyBtn,
	)

	settingsScroll := container.NewVScroll(settingsPanel)
	// Adaptive height for small screens - allow content to flow
	settingsScroll.SetMinSize(fyne.NewSize(350, 400))

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6},
		container.NewVBox(leftPanel, container.NewCenter(videoContainer)),
		settingsScroll,
	)

	content := container.NewPadded(mainContent)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
