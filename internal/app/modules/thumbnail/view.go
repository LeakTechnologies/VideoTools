package thumbnail

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	ThumbnailFile           any
	ThumbnailFiles          []any
	ThumbnailFilePaths      []string
	ThumbnailFileName       string            // base filename of the active file
	ThumbnailFileNames      []string          // base filenames for all loaded files
	ThumbnailPreviewFrame   string            // path to first preview frame image
	LivePreviewGrid         fyne.CanvasObject // persistent live-preview container; updated externally as thumbnails are generated
	ThumbnailCount          int
	ThumbnailWidth          int
	ThumbnailSheetWidth     int
	ThumbnailColumns        int
	ThumbnailRows           int
	ThumbnailOutputMode     string
	ThumbnailContactSheet   bool
	ThumbnailShowTimestamps bool

	OnShowMainMenu       func()
	OnShowQueue          func()
	OnShowThumbnailView  func()
	OnClearCompletedJobs func()
	OnGetStatsBar        func() fyne.CanvasObject

	OnLoadFile           func(path string)
	OnClearFiles         func()
	OnAddThumbnailSource func(src any)

	OnSetThumbnailCount          func(i int)
	OnSetThumbnailWidth          func(i int)
	OnSetThumbnailSheetWidth     func(i int)
	OnSetThumbnailColumns        func(i int)
	OnSetThumbnailRows           func(i int)
	OnSetThumbnailOutputMode     func(mode string)
	OnSetThumbnailContactSheet   func(b bool)
	OnSetThumbnailShowTimestamps func(b bool)

	OnCreateThumbJob        func()
	OnCreateThumbJobForPath func(path string)
	OnSelectThumbnailFile   func(id int)

	OnPersistConfig func()

	// Labels — all user-visible strings (fall back to English if empty)
	BackLabel               string
	ViewQueueLabel          string
	InstructionsLabel       string
	NoFileLabel             string
	FileLoadedLabel         string
	LoadVideoLabel          string
	ClearLabel              string
	ContactSheetToggleLabel string
	ShowTimestampsLabel     string
	ContactSheetGridLabel   string
	IndividualThumbsLabel   string
	ThumbnailSizeLabel      string
	OutputModeLabel         string
	ModeIndividualLabel     string
	ModeContactSheetLabel   string
	ModeBothLabel           string
	ColumnsFmt              string // "Columns: %d"
	RowsFmt                 string // "Rows: %d"
	TotalFmt                string // "Total thumbnails: %d"
	CountFmt                string // "Thumbnail Count: %d"
	WidthFmt                string // "Thumbnail Width: %d px"
	GenerateNowLabel        string
	AddToQueueLabel         string
	AddAllToQueueLabel      string
	LoadedVideosLabel       string
	VideoFmt                string // "Video %d"
	// Dialog strings
	NoVideoTitle   string
	NoVideoMsg     string
	StartedTitle   string
	StartedMsg     string
	JobQueuedTitle string
	JobQueuedMsg   string
	NoVideosTitle  string
	NoVideosMsg    string
	JobsQueuedFmt  string // "Queued %d thumbnail jobs."
}

func or(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func BuildView(opts Options) fyne.CanvasObject {
	thumbColor := opts.ModuleColor
	if thumbColor == nil {
		thumbColor = utils.MustHex("#5E35B1")
	}

	backBtn := widget.NewButton(or(opts.BackLabel, "< THUMBNAILS"), func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(or(opts.ViewQueueLabel, "View Queue"), func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})

	clearCompletedBtn := widget.NewButton("⌫", func() {
		if opts.OnClearCompletedJobs != nil {
			opts.OnClearCompletedJobs()
		}
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(thumbColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	instructions := widget.NewLabel(or(opts.InstructionsLabel, "Generate thumbnails from a video file. Load a video and configure settings."))
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	if opts.ThumbnailCount == 0 {
		opts.ThumbnailCount = 24
	}
	if opts.ThumbnailWidth == 0 {
		opts.ThumbnailWidth = 320
	}
	if opts.ThumbnailSheetWidth == 0 {
		opts.ThumbnailSheetWidth = 360
	}
	if opts.ThumbnailColumns == 0 {
		opts.ThumbnailColumns = 4
	}
	if opts.ThumbnailRows == 0 {
		opts.ThumbnailRows = 8
	}

	fileLabel := widget.NewLabel(or(opts.NoFileLabel, "[NO VIDEO LOADED]"))
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	if opts.ThumbnailFile != nil && opts.ThumbnailFileName != "" {
		name := opts.ThumbnailFileName
		fileLabel.SetText(name)
	}

	loadBtn := widget.NewButton(or(opts.LoadVideoLabel, "Load Video"), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()
			if opts.OnLoadFile != nil {
				opts.OnLoadFile(path)
			}
		}, opts.Window)
	})

	clearBtn := widget.NewButton(or(opts.ClearLabel, "Clear"), func() {
		if opts.OnClearFiles != nil {
			opts.OnClearFiles()
		}
		if opts.OnShowThumbnailView != nil {
			opts.OnShowThumbnailView()
		}
	})
	clearBtn.Importance = widget.LowImportance

	// Construct with nil OnChanged so that the initialising SetSelected call
	// below does not fire the callback and trigger infinite recursion
	// (SetSelected fires OnChanged when the value changes from the zero "").
	outputModeRadio := widget.NewRadioGroup(
		[]string{
			or(opts.ModeIndividualLabel, "Individual"),
			or(opts.ModeContactSheetLabel, "Contact Sheet"),
			or(opts.ModeBothLabel, "Both"),
		},
		nil,
	)
	outputModeRadio.Horizontal = true
	switch opts.ThumbnailOutputMode {
	case "contactSheet":
		outputModeRadio.SetSelected(or(opts.ModeContactSheetLabel, "Contact Sheet"))
	case "both":
		outputModeRadio.SetSelected(or(opts.ModeBothLabel, "Both"))
	default:
		outputModeRadio.SetSelected(or(opts.ModeIndividualLabel, "Individual"))
	}
	outputModeRadio.OnChanged = func(value string) {
		switch value {
		case or(opts.ModeIndividualLabel, "Individual"):
			if opts.OnSetThumbnailOutputMode != nil {
				opts.OnSetThumbnailOutputMode("individual")
			}
		case or(opts.ModeContactSheetLabel, "Contact Sheet"):
			if opts.OnSetThumbnailOutputMode != nil {
				opts.OnSetThumbnailOutputMode("contactSheet")
			}
		case or(opts.ModeBothLabel, "Both"):
			if opts.OnSetThumbnailOutputMode != nil {
				opts.OnSetThumbnailOutputMode("both")
			}
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
		if opts.OnShowThumbnailView != nil {
			opts.OnShowThumbnailView()
		}
	}

	outputModeBox := container.NewVBox(
		widget.NewLabelWithStyle(or(opts.OutputModeLabel, "Output Mode"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputModeRadio,
	)

	timestampCheck := widget.NewCheck("", func(checked bool) {
		if opts.OnSetThumbnailShowTimestamps != nil {
			opts.OnSetThumbnailShowTimestamps(checked)
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	timestampCheck.Checked = opts.ThumbnailShowTimestamps
	timestampLabel := widget.NewLabel(or(opts.ShowTimestampsLabel, "Show timestamps on thumbnails"))
	timestampLabel.Wrapping = fyne.TextWrapWord
	timestampToggle := ui.NewTappable(timestampLabel, func() {
		timestampCheck.SetChecked(!timestampCheck.Checked)
	})
	timestampRow := container.NewBorder(nil, nil, timestampCheck, nil, timestampToggle)

	buildThumbBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		header := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
		)
		body := container.NewBorder(header, nil, nil, nil, content)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	columnsFmt := or(opts.ColumnsFmt, "Columns: %d")
	rowsFmt := or(opts.RowsFmt, "Rows: %d")
	totalFmt := or(opts.TotalFmt, "Total thumbnails: %d")
	countFmt := or(opts.CountFmt, "Thumbnail Count: %d")
	widthFmt := or(opts.WidthFmt, "Thumbnail Width: %d px")

	var settingsOptions fyne.CanvasObject
	showContactSheet := opts.ThumbnailOutputMode == "contactSheet" || opts.ThumbnailOutputMode == "both"

	if showContactSheet {
		colLabel := widget.NewLabel(fmt.Sprintf(columnsFmt, opts.ThumbnailColumns))
		rowLabel := widget.NewLabel(fmt.Sprintf(rowsFmt, opts.ThumbnailRows))

		totalThumbs := opts.ThumbnailColumns * opts.ThumbnailRows
		totalLabel := widget.NewLabel(fmt.Sprintf(totalFmt, totalThumbs))
		totalLabel.TextStyle = fyne.TextStyle{Italic: true}
		totalLabel.Wrapping = fyne.TextWrapWord

		colSlider := ui.MakeSlider(2, 9)
		colSlider.Value = float64(opts.ThumbnailColumns)
		colSlider.Step = 1
		colSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailColumns != nil {
				opts.OnSetThumbnailColumns(int(val))
			}
			colLabel.SetText(fmt.Sprintf(columnsFmt, int(val)))
			totalLabel.SetText(fmt.Sprintf(totalFmt, opts.ThumbnailColumns*opts.ThumbnailRows))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		rowSlider := ui.MakeSlider(2, 12)
		rowSlider.Value = float64(opts.ThumbnailRows)
		rowSlider.Step = 1
		rowSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailRows != nil {
				opts.OnSetThumbnailRows(int(val))
			}
			rowLabel.SetText(fmt.Sprintf(rowsFmt, int(val)))
			totalLabel.SetText(fmt.Sprintf(totalFmt, opts.ThumbnailColumns*opts.ThumbnailRows))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		sizeOptions := []string{"240 px", "300 px", "360 px", "420 px", "480 px", "540 px", "576 px", "640 px"}
		sizeSelect := widget.NewSelect(sizeOptions, func(val string) {
			var width int
			switch val {
			case "240 px":
				width = 240
			case "300 px":
				width = 300
			case "360 px":
				width = 360
			case "420 px":
				width = 420
			case "480 px":
				width = 480
			case "540 px":
				width = 540
			case "576 px":
				width = 576
			case "640 px":
				width = 640
			}
			if opts.OnSetThumbnailSheetWidth != nil {
				opts.OnSetThumbnailSheetWidth(width)
			}
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		})
		switch opts.ThumbnailSheetWidth {
		case 240:
			sizeSelect.SetSelected("240 px")
		case 300:
			sizeSelect.SetSelected("300 px")
		case 360:
			sizeSelect.SetSelected("360 px")
		case 420:
			sizeSelect.SetSelected("420 px")
		case 480:
			sizeSelect.SetSelected("480 px")
		case 540:
			sizeSelect.SetSelected("540 px")
		case 576:
			sizeSelect.SetSelected("576 px")
		case 640:
			sizeSelect.SetSelected("640 px")
		default:
			sizeSelect.SetSelected("360 px")
		}

		settingsOptions = buildThumbBox(or(opts.ContactSheetGridLabel, "Contact Sheet Grid"), container.NewVBox(
			widget.NewLabel(or(opts.ThumbnailSizeLabel, "Thumbnail Size:")),
			sizeSelect,
			colLabel,
			colSlider,
			rowLabel,
			rowSlider,
			totalLabel,
		))
	} else {
		countLabel := widget.NewLabel(fmt.Sprintf(countFmt, opts.ThumbnailCount))
		countSlider := ui.MakeSlider(3, 50)
		countSlider.Value = float64(opts.ThumbnailCount)
		countSlider.Step = 1
		countSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailCount != nil {
				opts.OnSetThumbnailCount(int(val))
			}
			countLabel.SetText(fmt.Sprintf(countFmt, int(val)))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		widthLabel := widget.NewLabel(fmt.Sprintf(widthFmt, opts.ThumbnailWidth))
		individualSizeOptions := []string{"240 px", "300 px", "360 px", "420 px", "480 px", "540 px", "576 px", "640 px"}
		widthSelect := widget.NewSelect(individualSizeOptions, func(val string) {
			var width int
			switch val {
			case "240 px":
				width = 240
			case "300 px":
				width = 300
			case "360 px":
				width = 360
			case "420 px":
				width = 420
			case "480 px":
				width = 480
			case "540 px":
				width = 540
			case "576 px":
				width = 576
			case "640 px":
				width = 640
			}
			if opts.OnSetThumbnailWidth != nil {
				opts.OnSetThumbnailWidth(width)
			}
			widthLabel.SetText(fmt.Sprintf(widthFmt, width))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		})
		switch opts.ThumbnailWidth {
		case 240, 300, 360, 420, 480, 540, 576, 640:
			widthSelect.SetSelected(fmt.Sprintf("%d px", opts.ThumbnailWidth))
		default:
			widthSelect.SetSelected("320 px")
		}

		settingsOptions = buildThumbBox(or(opts.IndividualThumbsLabel, "Individual Thumbnails"), container.NewVBox(
			countLabel,
			countSlider,
			widthLabel,
			widthSelect,
		))
	}

	noVideoTitle := or(opts.NoVideoTitle, "No Video")
	noVideoMsg := or(opts.NoVideoMsg, "Please load a video file first.")
	startedTitle := or(opts.StartedTitle, "Thumbnails")
	startedMsg := or(opts.StartedMsg, "Thumbnail generation started! View progress in Job Queue.")
	jobQueuedTitle := or(opts.JobQueuedTitle, "Queue")
	jobQueuedMsg := or(opts.JobQueuedMsg, "Thumbnail job added to queue!")
	noVideosTitle := or(opts.NoVideosTitle, "No Videos")
	noVideosMsg := or(opts.NoVideosMsg, "Load videos first to add to queue.")
	jobsQueuedFmt := or(opts.JobsQueuedFmt, "Queued %d thumbnail jobs.")

	generateNowBtn := ui.NewPillButton(or(opts.GenerateNowLabel, "GENERATE NOW"), thumbColor, func() {
		if opts.ThumbnailFile == nil {
			dialog.ShowInformation(noVideoTitle, noVideoMsg, opts.Window)
			return
		}
		if opts.OnCreateThumbJob != nil {
			opts.OnCreateThumbJob()
		}
		dialog.ShowInformation(startedTitle, startedMsg, opts.Window)
	})
	if opts.ThumbnailFile == nil {
		generateNowBtn.Disable()
	}

	addQueueBtn := widget.NewButton(or(opts.AddToQueueLabel, "Add to Queue"), func() {
		if opts.ThumbnailFile == nil {
			dialog.ShowInformation(noVideoTitle, noVideoMsg, opts.Window)
			return
		}
		if opts.OnCreateThumbJob != nil {
			opts.OnCreateThumbJob()
		}
		dialog.ShowInformation(jobQueuedTitle, jobQueuedMsg, opts.Window)
	})
	addQueueBtn.Importance = widget.MediumImportance

	if opts.ThumbnailFile == nil {
		addQueueBtn.Disable()
	}

	addAllBtn := widget.NewButton(or(opts.AddAllToQueueLabel, "Add All to Queue"), func() {
		if len(opts.ThumbnailFiles) == 0 {
			dialog.ShowInformation(noVideosTitle, noVideosMsg, opts.Window)
			return
		}
		if opts.OnCreateThumbJobForPath != nil && len(opts.ThumbnailFilePaths) > 0 {
			for _, path := range opts.ThumbnailFilePaths {
				opts.OnCreateThumbJobForPath(path)
			}
		} else if opts.OnCreateThumbJob != nil {
			for range opts.ThumbnailFiles {
				opts.OnCreateThumbJob()
			}
		}
		dialog.ShowInformation(jobQueuedTitle, fmt.Sprintf(jobsQueuedFmt, len(opts.ThumbnailFiles)), opts.Window)
	})
	addAllBtn.Importance = widget.MediumImportance

	viewQueueBtn := widget.NewButton(or(opts.ViewQueueLabel, "View Queue"), func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})
	viewQueueBtn.Importance = widget.MediumImportance

	// --- Settings panel (left, fixed width) ---
	var previewWidget fyne.CanvasObject
	if opts.ThumbnailPreviewFrame != "" {
		img := canvas.NewImageFromFile(opts.ThumbnailPreviewFrame)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(0, 120))
		bg := canvas.NewRectangle(navyBlue)
		previewWidget = container.NewMax(bg, img)
	}

	settingsPanelItems := []fyne.CanvasObject{}
	if previewWidget != nil {
		settingsPanelItems = append(settingsPanelItems, previewWidget, widget.NewSeparator())
	}
	settingsPanelItems = append(settingsPanelItems, outputModeBox, timestampRow, widget.NewSeparator(), settingsOptions)

	if len(opts.ThumbnailFiles) > 0 {
		videoFmt := or(opts.VideoFmt, "Video %d")
		list := widget.NewList(
			func() int { return len(opts.ThumbnailFiles) },
			func() fyne.CanvasObject { return widget.NewLabel("template") },
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				if label, ok := obj.(*widget.Label); ok {
					name := ""
					if id < len(opts.ThumbnailFileNames) {
						name = opts.ThumbnailFileNames[id]
					}
					if name == "" {
						name = fmt.Sprintf(videoFmt, id+1)
					}
					label.SetText(name)
				}
			},
		)
		list.OnSelected = func(id widget.ListItemID) {
			if opts.OnSelectThumbnailFile != nil {
				opts.OnSelectThumbnailFile(id)
			} else if opts.OnShowThumbnailView != nil {
				opts.OnShowThumbnailView()
			}
		}
		listScroll := container.NewVScroll(list)
		listScroll.SetMinSize(fyne.NewSize(0, 80))
		settingsPanelItems = append(settingsPanelItems, widget.NewSeparator(), widget.NewLabel(or(opts.LoadedVideosLabel, "Loaded Videos:")), listScroll)
	}

	settingsPanel := container.NewVScroll(container.NewVBox(settingsPanelItems...))

	// --- Live preview panel (right, expands to fill) ---
	liveHeader := widget.NewLabelWithStyle("Generated Thumbnails", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	var liveBody fyne.CanvasObject
	if opts.LivePreviewGrid != nil {
		liveBody = container.NewScroll(opts.LivePreviewGrid)
	} else {
		placeholder := widget.NewLabel("Thumbnails will appear here as they are generated.")
		placeholder.Alignment = fyne.TextAlignCenter
		liveBody = container.NewCenter(placeholder)
	}

	livePanel := container.NewBorder(
		container.NewVBox(liveHeader, widget.NewSeparator()),
		nil, nil, nil,
		liveBody,
	)

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.32}, settingsPanel, livePanel)

	topRow := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, nil, container.NewHBox(loadBtn, clearBtn), fileLabel),
		widget.NewSeparator(),
	)

	content := container.NewBorder(topRow, nil, nil, nil, mainContent)

	statsBar := opts.OnGetStatsBar()

	bottomItems := []fyne.CanvasObject{
		container.NewHBox(addAllBtn, addQueueBtn, layout.NewSpacer(), generateNowBtn),
	}
	if statsBar != nil {
		bottomItems = append(bottomItems, statsBar)
	}
	bottomBar := container.NewVBox(bottomItems...)
	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

type fixedHSplitLayout struct {
	ratio float32
}

func (f *fixedHSplitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	width := float32(size.Width)
	leftWidth := float32(int(width * f.ratio))
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(leftWidth, size.Height))
	objects[1].Move(fyne.NewPos(leftWidth, 0))
	objects[1].Resize(fyne.NewSize(size.Width-leftWidth, size.Height))
}

func (f *fixedHSplitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	min1 := objects[0].MinSize()
	min2 := objects[1].MinSize()
	return fyne.NewSize(min1.Width+min2.Width, max(min1.Height, min2.Height))
}
