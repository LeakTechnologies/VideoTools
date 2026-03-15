package thumbnail

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window fyne.Window

	ThumbnailFile           any
	ThumbnailFiles          []any
	ThumbnailCount          int
	ThumbnailWidth          int
	ThumbnailSheetWidth     int
	ThumbnailColumns        int
	ThumbnailRows           int
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
	OnSetThumbnailContactSheet   func(b bool)
	OnSetThumbnailShowTimestamps func(b bool)

	OnCreateThumbJob func() any

	OnPersistConfig func()
}

func BuildView(opts Options) fyne.CanvasObject {
	thumbColor := utils.MustHex("#FF8F00")

	backBtn := widget.NewButton("< THUMBNAILS", func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton("View Queue", func() {
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

	instructions := widget.NewLabel("Generate thumbnails from a video file. Load a video and configure settings.")
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

	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	if opts.ThumbnailFile != nil {
		fileLabel.SetText("File: video loaded")
	}

	loadBtn := widget.NewButton("Load Video", func() {
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

	clearBtn := widget.NewButton("Clear", func() {
		if opts.OnClearFiles != nil {
			opts.OnClearFiles()
		}
		if opts.OnShowThumbnailView != nil {
			opts.OnShowThumbnailView()
		}
	})
	clearBtn.Importance = widget.LowImportance

	contactSheetCheck := widget.NewCheck("", func(checked bool) {
		if opts.OnSetThumbnailContactSheet != nil {
			opts.OnSetThumbnailContactSheet(checked)
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
		if opts.OnShowThumbnailView != nil {
			opts.OnShowThumbnailView()
		}
	})
	contactSheetCheck.Checked = opts.ThumbnailContactSheet
	contactSheetLabel := widget.NewLabel("Generate Contact Sheet (single image)")
	contactSheetLabel.Wrapping = fyne.TextWrapWord
	contactSheetToggle := ui.NewTappable(contactSheetLabel, func() {
		contactSheetCheck.SetChecked(!contactSheetCheck.Checked)
	})
	contactSheetRow := container.NewBorder(nil, nil, contactSheetCheck, nil, contactSheetToggle)

	timestampCheck := widget.NewCheck("", func(checked bool) {
		if opts.OnSetThumbnailShowTimestamps != nil {
			opts.OnSetThumbnailShowTimestamps(checked)
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	timestampCheck.Checked = opts.ThumbnailShowTimestamps
	timestampLabel := widget.NewLabel("Show timestamps on thumbnails")
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
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	var settingsOptions fyne.CanvasObject
	if opts.ThumbnailContactSheet {
		colLabel := widget.NewLabel(fmt.Sprintf("Columns: %d", opts.ThumbnailColumns))
		rowLabel := widget.NewLabel(fmt.Sprintf("Rows: %d", opts.ThumbnailRows))

		totalThumbs := opts.ThumbnailColumns * opts.ThumbnailRows
		totalLabel := widget.NewLabel(fmt.Sprintf("Total thumbnails: %d", totalThumbs))
		totalLabel.TextStyle = fyne.TextStyle{Italic: true}
		totalLabel.Wrapping = fyne.TextWrapWord

		colSlider := widget.NewSlider(2, 9)
		colSlider.Value = float64(opts.ThumbnailColumns)
		colSlider.Step = 1
		colSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailColumns != nil {
				opts.OnSetThumbnailColumns(int(val))
			}
			colLabel.SetText(fmt.Sprintf("Columns: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", opts.ThumbnailColumns*opts.ThumbnailRows))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		rowSlider := widget.NewSlider(2, 12)
		rowSlider.Value = float64(opts.ThumbnailRows)
		rowSlider.Step = 1
		rowSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailRows != nil {
				opts.OnSetThumbnailRows(int(val))
			}
			rowLabel.SetText(fmt.Sprintf("Rows: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", opts.ThumbnailColumns*opts.ThumbnailRows))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		sizeOptions := []string{"240 px", "300 px", "360 px", "420 px", "480 px"}
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
		case 420:
			sizeSelect.SetSelected("420 px")
		case 480:
			sizeSelect.SetSelected("480 px")
		default:
			sizeSelect.SetSelected("360 px")
		}

		settingsOptions = buildThumbBox("Contact Sheet Grid", container.NewVBox(
			widget.NewLabel("Thumbnail Size:"),
			sizeSelect,
			colLabel,
			colSlider,
			rowLabel,
			rowSlider,
			totalLabel,
		))
	} else {
		countLabel := widget.NewLabel(fmt.Sprintf("Thumbnail Count: %d", opts.ThumbnailCount))
		countSlider := widget.NewSlider(3, 50)
		countSlider.Value = float64(opts.ThumbnailCount)
		countSlider.Step = 1
		countSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailCount != nil {
				opts.OnSetThumbnailCount(int(val))
			}
			countLabel.SetText(fmt.Sprintf("Thumbnail Count: %d", int(val)))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		widthLabel := widget.NewLabel(fmt.Sprintf("Thumbnail Width: %d px", opts.ThumbnailWidth))
		widthSlider := widget.NewSlider(160, 640)
		widthSlider.Value = float64(opts.ThumbnailWidth)
		widthSlider.Step = 32
		widthSlider.OnChanged = func(val float64) {
			if opts.OnSetThumbnailWidth != nil {
				opts.OnSetThumbnailWidth(int(val))
			}
			widthLabel.SetText(fmt.Sprintf("Thumbnail Width: %d px", int(val)))
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}

		settingsOptions = buildThumbBox("Individual Thumbnails", container.NewVBox(
			countLabel,
			countSlider,
			widthLabel,
			widthSlider,
		))
	}

	generateNowBtn := widget.NewButton("GENERATE NOW", func() {
		if opts.ThumbnailFile == nil {
			dialog.ShowInformation("No Video", "Please load a video file first.", opts.Window)
			return
		}
		if opts.OnCreateThumbJob != nil {
			_ = opts.OnCreateThumbJob()
		}
		dialog.ShowInformation("Thumbnails", "Thumbnail generation started! View progress in Job Queue.", opts.Window)
	})
	generateNowBtn.Importance = widget.HighImportance

	if opts.ThumbnailFile == nil {
		generateNowBtn.Disable()
	}

	addQueueBtn := widget.NewButton("Add to Queue", func() {
		if opts.ThumbnailFile == nil {
			dialog.ShowInformation("No Video", "Please load a video file first.", opts.Window)
			return
		}
		if opts.OnCreateThumbJob != nil {
			_ = opts.OnCreateThumbJob()
		}
		dialog.ShowInformation("Queue", "Thumbnail job added to queue!", opts.Window)
	})
	addQueueBtn.Importance = widget.MediumImportance

	if opts.ThumbnailFile == nil {
		addQueueBtn.Disable()
	}

	addAllBtn := widget.NewButton("Add All to Queue", func() {
		if len(opts.ThumbnailFiles) == 0 {
			dialog.ShowInformation("No Videos", "Load videos first to add to queue.", opts.Window)
			return
		}
		if opts.OnCreateThumbJob != nil {
			for range opts.ThumbnailFiles {
				_ = opts.OnCreateThumbJob()
			}
		}
		dialog.ShowInformation("Queue", fmt.Sprintf("Queued %d thumbnail jobs.", len(opts.ThumbnailFiles)), opts.Window)
	})
	addAllBtn.Importance = widget.MediumImportance

	viewQueueBtn := widget.NewButton("View Queue", func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})
	viewQueueBtn.Importance = widget.MediumImportance

	leftColumn := container.NewVBox(
		contactSheetRow,
		timestampRow,
		widget.NewSeparator(),
	)

	if len(opts.ThumbnailFiles) > 0 {
		list := widget.NewList(
			func() int { return len(opts.ThumbnailFiles) },
			func() fyne.CanvasObject { return widget.NewLabel("template") },
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				if label, ok := obj.(*widget.Label); ok {
					label.SetText(fmt.Sprintf("Video %d", id+1))
				}
			},
		)
		list.OnSelected = func(id widget.ListItemID) {
			if opts.OnShowThumbnailView != nil {
				opts.OnShowThumbnailView()
			}
		}
		listScroll := container.NewVScroll(list)
		listScroll.SetMinSize(fyne.NewSize(0, 0))
		leftColumn.Add(widget.NewLabel("Loaded Videos:"))
		leftColumn.Add(listScroll)
	}

	rightColumn := container.NewVScroll(settingsOptions)

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6}, leftColumn, rightColumn)

	content := container.NewBorder(
		container.NewVBox(instructions, widget.NewSeparator(), fileLabel, container.NewHBox(loadBtn, clearBtn)),
		nil,
		nil,
		nil,
		mainContent,
	)

	statsBar := opts.OnGetStatsBar()

	bottomBar := container.NewVBox(
		container.NewHBox(addAllBtn, addQueueBtn, layout.NewSpacer(), generateNowBtn),
		statsBar,
	)
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
