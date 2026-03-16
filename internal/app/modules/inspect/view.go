package inspect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"image/color"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var valueBg     = utils.MustHex("#2B334A")
var valueBorder = utils.MustHex("#3A4360")

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	InspectFile               any
	InspectInterlaceAnalyzing bool
	InspectInterlaceResult    any

	OnShowMainMenu       func()
	OnShowQueue          func()
	OnShowInspectView    func()
	OnClearCompletedJobs func()
	OnGetStatsBar        func() fyne.CanvasObject
	OnOpenLogViewer      func(title, path string, isTemp bool)

	OnLoadFile  func(path string)
	OnClearFile func()

	OnGetFormat       func() string
	OnGetVideoCodec   func() string
	OnGetWidth        func() int
	OnGetHeight       func() int
	OnGetAspectRatio  func() string
	OnGetFrameRate    func() float64
	OnGetBitrate      func() int64
	OnGetPixelFormat  func() string
	OnGetColorSpace   func() string
	OnGetColorRange   func() string
	OnGetFieldOrder   func() string
	OnGetGOPSize      func() int
	OnGetAudioCodec   func() string
	OnGetAudioBitrate func() int64
	OnGetAudioRate    func() int
	OnGetChannels     func() int
	OnGetDuration     func() string
	OnGetSampleAspect func() string
	OnGetHasChapters  func() bool
	OnGetHasMetadata  func() bool
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()
	inspectColor := opts.ModuleColor
	if inspectColor == nil {
		inspectColor = utils.MustHex("#3A3F9F")
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleInspect), func() {
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

	topBar := ui.TintedBar(inspectColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	statsBar := opts.OnGetStatsBar()
	bottomBar := container.NewVBox(layout.NewSpacer(), statsBar)

	instructions := widget.NewLabel("Load a video to inspect its properties and preview playback. Drag a video here or use the button below.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	clearBtn := widget.NewButton("Clear", func() {
		if opts.OnClearFile != nil {
			opts.OnClearFile()
		}
		if opts.OnShowInspectView != nil {
			opts.OnShowInspectView()
		}
	})
	clearBtn.Importance = widget.LowImportance

	instructionsRow := container.NewBorder(nil, nil, nil, nil, instructions)

	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	buildInspectBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
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

	// --- pill helpers (mirrors buildMetadataPanel in main.go) ---
	makeValuePill := func(text string) fyne.CanvasObject {
		bg := canvas.NewRectangle(valueBg)
		bg.CornerRadius = 6
		bg.StrokeColor = valueBorder
		bg.StrokeWidth = 1
		lbl := widget.NewLabel(text)
		lbl.TextStyle = fyne.TextStyle{Monospace: true}
		lbl.Wrapping = fyne.TextTruncate
		return container.NewMax(bg, container.NewPadded(lbl))
	}
	makeValuePillWithChip := func(text string, chipColor color.Color) fyne.CanvasObject {
		bg := canvas.NewRectangle(valueBg)
		bg.CornerRadius = 6
		bg.StrokeColor = valueBorder
		bg.StrokeWidth = 1
		chip := canvas.NewRectangle(chipColor)
		chip.CornerRadius = 4
		chip.SetMinSize(fyne.NewSize(8, 0))
		lbl := widget.NewLabel(text)
		lbl.TextStyle = fyne.TextStyle{Monospace: true}
		lbl.Wrapping = fyne.TextTruncate
		pillContent := container.NewBorder(nil, nil, chip, nil, container.NewPadded(lbl))
		return container.NewMax(bg, pillContent)
	}
	makeRow := func(key string, value fyne.CanvasObject) fyne.CanvasObject {
		keyLbl := widget.NewLabel(key + ":")
		keyLbl.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewBorder(nil, nil, keyLbl, nil, value)
	}

	// placeholder shown when no file is loaded
	metadataPlaceholder := container.NewCenter(widget.NewLabel("No file loaded"))

	// metadataGrid holds the live pill grid; swapped in updateDisplay
	metadataGrid := container.NewMax(metadataPlaceholder)

	buildMetadataGrid := func() fyne.CanvasObject {
		if opts.InspectFile == nil {
			return metadataPlaceholder
		}

		// --- collect values via callbacks ---
		get := func(cb func() string) string {
			if cb == nil { return "Unknown" }
			if v := cb(); v != "" { return v }
			return "Unknown"
		}
		getInt := func(cb func() int) int {
			if cb == nil { return 0 }
			return cb()
		}
		getI64 := func(cb func() int64) int64 {
			if cb == nil { return 0 }
			return cb()
		}
		getF := func(cb func() float64) float64 {
			if cb == nil { return 0 }
			return cb()
		}
		getBool := func(cb func() bool) bool {
			if cb == nil { return false }
			return cb()
		}

		format     := get(opts.OnGetFormat)
		videoCodec := get(opts.OnGetVideoCodec)
		width      := getInt(opts.OnGetWidth)
		height     := getInt(opts.OnGetHeight)
		aspectRatio := get(opts.OnGetAspectRatio)
		frameRate  := getF(opts.OnGetFrameRate)
		bitrate    := getI64(opts.OnGetBitrate)
		pixelFmt   := get(opts.OnGetPixelFormat)
		colorSpace := get(opts.OnGetColorSpace)
		colorRange := get(opts.OnGetColorRange)
		fieldOrder := get(opts.OnGetFieldOrder)
		gopSize    := getInt(opts.OnGetGOPSize)
		audioCodec := get(opts.OnGetAudioCodec)
		audioBitrate := getI64(opts.OnGetAudioBitrate)
		audioRate  := getInt(opts.OnGetAudioRate)
		channels   := getInt(opts.OnGetChannels)
		duration   := get(opts.OnGetDuration)
		sar        := get(opts.OnGetSampleAspect)
		hasChapters := getBool(opts.OnGetHasChapters)
		hasMetadata := getBool(opts.OnGetHasMetadata)

		// file size
		fileSize := "Unknown"
		if p, ok := any(opts.InspectFile).(interface{ GetPath() string }); ok {
			if fi, err := os.Stat(p.GetPath()); err == nil {
				fileSize = utils.FormatBytes(fi.Size())
			}
		}

		// format values
		bitrateStr := "--"
		if bitrate > 0 { bitrateStr = fmt.Sprintf("%d kbps", bitrate/1000) }
		audioBitrateStr := "--"
		if audioBitrate > 0 { audioBitrateStr = fmt.Sprintf("%d kbps", audioBitrate/1000) }

		parStr := sar
		if parStr == "1:1" || parStr == "" { parStr = "1:1 (Square)" } else { parStr += " (Non-square)" }

		if colorRange == "tv" { colorRange = "Limited (TV)" } else if colorRange == "pc" || colorRange == "jpeg" { colorRange = "Full (PC)" }

		interlacing := "Progressive"
		if fieldOrder != "" && fieldOrder != "progressive" && fieldOrder != "unknown" && fieldOrder != "Unknown" {
			interlacing = "Interlaced (" + fieldOrder + ")"
		}

		gopStr := "--"
		if gopSize > 0 { gopStr = fmt.Sprintf("%d frames", gopSize) }

		chaptersStr := "No"
		if hasChapters { chaptersStr = "Yes" }
		metadataStr := "No"
		if hasMetadata { metadataStr = "Yes" }

		// --- plain-text copy string (unchanged from before) ---
		_ = fmt.Sprintf("Format: %s\nResolution: %dx%d\nDuration: %s\nFile Size: %s",
			format, width, height, duration, fileSize)

		col1 := container.NewVBox(
			makeRow("Format",       makeValuePill(format)),
			makeRow("Resolution",   makeValuePill(fmt.Sprintf("%dx%d", width, height))),
			makeRow("Aspect Ratio", makeValuePill(aspectRatio)),
			makeRow("Duration",     makeValuePill(duration)),
			makeRow("Frame Rate",   makeValuePill(fmt.Sprintf("%.2f fps", frameRate))),
			makeRow("Interlacing",  makeValuePill(interlacing)),
			makeRow("Color Space",  makeValuePill(colorSpace)),
			makeRow("Color Range",  makeValuePill(colorRange)),
			makeRow("GOP Size",     makeValuePill(gopStr)),
			makeRow("File Size",    makeValuePill(fileSize)),
		)

		col2 := container.NewVBox(
			makeRow("Video Codec",   makeValuePillWithChip(videoCodec, ui.GetVideoCodecColor(videoCodec))),
			makeRow("Video Bitrate", makeValuePill(bitrateStr)),
			makeRow("Pixel Format",  makeValuePill(pixelFmt)),
			makeRow("Pixel AR",      makeValuePill(parStr)),
			makeRow("Audio Codec",   makeValuePillWithChip(audioCodec, ui.GetAudioCodecColor(audioCodec))),
			makeRow("Audio Bitrate", makeValuePill(audioBitrateStr)),
			makeRow("Audio Rate",    makeValuePill(fmt.Sprintf("%d Hz", audioRate))),
			makeRow("Channels",      makeValuePill(utils.ChannelLabel(channels))),
			makeRow("Chapters",      makeValuePill(chaptersStr)),
			makeRow("Metadata",      makeValuePill(metadataStr)),
		)

		interlaceNote := ""
		if opts.InspectInterlaceAnalyzing {
			interlaceNote = "Analyzing interlacing... (first 500 frames)"
		} else if opts.InspectInterlaceResult != nil {
			interlaceNote = "Interlace analysis complete"
		}
		var extra fyne.CanvasObject
		if interlaceNote != "" {
			extra = widget.NewLabel(interlaceNote)
		}

		rows := []fyne.CanvasObject{container.NewGridWithColumns(2, col1, col2)}
		if extra != nil {
			rows = append(rows, extra)
		}
		return container.NewVBox(rows...)
	}

	// formatMetadata returns plain text for clipboard copy
	formatMetadata := func() string {
		if opts.InspectFile == nil {
			return "No file loaded"
		}
		get := func(cb func() string) string {
			if cb == nil { return "" }
			return cb()
		}
		path := ""
		if p, ok := any(opts.InspectFile).(interface{ GetPath() string }); ok {
			path = p.GetPath()
		}
		bitrate := int64(0)
		if opts.OnGetBitrate != nil { bitrate = opts.OnGetBitrate() }
		audioBitrate := int64(0)
		if opts.OnGetAudioBitrate != nil { audioBitrate = opts.OnGetAudioBitrate() }
		bitrateStr := "--"
		if bitrate > 0 { bitrateStr = fmt.Sprintf("%d kbps", bitrate/1000) }
		audioBitrateStr := "--"
		if audioBitrate > 0 { audioBitrateStr = fmt.Sprintf("%d kbps", audioBitrate/1000) }
		width := 0; height := 0
		if opts.OnGetWidth != nil { width = opts.OnGetWidth() }
		if opts.OnGetHeight != nil { height = opts.OnGetHeight() }
		fr := 0.0
		if opts.OnGetFrameRate != nil { fr = opts.OnGetFrameRate() }
		ar := 0
		if opts.OnGetAudioRate != nil { ar = opts.OnGetAudioRate() }
		ch := 0
		if opts.OnGetChannels != nil { ch = opts.OnGetChannels() }
		return fmt.Sprintf(
			"File: %s\nFormat: %s\nResolution: %dx%d\nAspect Ratio: %s\nDuration: %s\n"+
				"Video Codec: %s\nVideo Bitrate: %s\nFrame Rate: %.2f fps\n"+
				"Pixel Format: %s\nColor Space: %s\nField Order: %s\n"+
				"Audio Codec: %s\nAudio Bitrate: %s\nAudio Rate: %d Hz\nChannels: %d",
			filepath.Base(path),
			get(opts.OnGetFormat),
			width, height,
			get(opts.OnGetAspectRatio),
			get(opts.OnGetDuration),
			get(opts.OnGetVideoCodec), bitrateStr, fr,
			get(opts.OnGetPixelFormat), get(opts.OnGetColorSpace), get(opts.OnGetFieldOrder),
			get(opts.OnGetAudioCodec), audioBitrateStr, ar, ch,
		)
	}

	var videoContainer fyne.CanvasObject = container.NewCenter(widget.NewLabel("No video loaded"))

	updateDisplay := func() {
		if opts.InspectFile != nil {
			filename := "video"
			if p, ok := any(opts.InspectFile).(interface{ GetPath() string }); ok {
				filename = filepath.Base(p.GetPath())
			}
			if len(filename) > 50 {
				ext := filepath.Ext(filename)
				nameWithoutExt := strings.TrimSuffix(filename, ext)
				if len(ext) > 10 {
					filename = filename[:47] + "..."
				} else {
					availableLen := 47 - len(ext)
					if availableLen < 1 {
						filename = filename[:47] + "..."
					} else {
						filename = nameWithoutExt[:availableLen] + "..." + ext
					}
				}
			}
			fileLabel.SetText(fmt.Sprintf("File: %s", filename))
			metadataGrid.Objects = []fyne.CanvasObject{buildMetadataGrid()}
			metadataGrid.Refresh()
			videoContainer = container.NewCenter(widget.NewLabel("Video preview"))
		} else {
			fileLabel.SetText("No file loaded")
			metadataGrid.Objects = []fyne.CanvasObject{metadataPlaceholder}
			metadataGrid.Refresh()
			videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
		}
	}

	updateDisplay()

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

	copyBtn := widget.NewButton("Copy Metadata", func() {
		metadata := formatMetadata()
		opts.Window.Clipboard().SetContent(metadata)
		dialog.ShowInformation("Copied", "Metadata copied to clipboard", opts.Window)
	})
	copyBtn.Importance = widget.LowImportance

	viewLogBtn := widget.NewButton("View Conversion Log", func() {
		dialog.ShowInformation("No Log", "No conversion log found for this file.", opts.Window)
	})
	viewLogBtn.Importance = widget.LowImportance
	viewLogBtn.Disable()

	actionButtons := container.NewHBox(loadBtn, copyBtn, viewLogBtn, clearBtn)

	leftColumn := container.NewBorder(
		fileLabel,
		nil, nil, nil,
		videoContainer,
	)

	rightColumn := buildInspectBox("Metadata", container.NewScroll(metadataGrid))

	content := container.NewBorder(
		container.NewVBox(instructionsRow, actionButtons, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, leftColumn, rightColumn),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
