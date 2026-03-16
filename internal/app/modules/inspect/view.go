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

	metadataText := widget.NewLabel("No file loaded")
	metadataText.Wrapping = fyne.TextWrapWord

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

	metadataScroll := container.NewScroll(metadataText)

	formatBitrateFull := func(bitrate int64) string {
		if bitrate <= 0 {
			return "N/A"
		}
		if bitrate >= 1_000_000 {
			return fmt.Sprintf("%.2f Mbps", float64(bitrate)/1_000_000)
		} else if bitrate >= 1_000 {
			return fmt.Sprintf("%.2f Kbps", float64(bitrate)/1_000)
		}
		return fmt.Sprintf("%d bps", bitrate)
	}

	formatMetadata := func() string {
		if opts.InspectFile == nil {
			return "No file loaded"
		}

		src := opts.InspectFile
		_ = src

		fileSize := "Unknown"
		path := ""
		if p, ok := any(src).(interface{ GetPath() string }); ok {
			path = p.GetPath()
		}
		if path != "" {
			if fi, err := os.Stat(path); err == nil {
				fileSize = utils.FormatBytes(fi.Size())
			}
		}

		format := ""
		if opts.OnGetFormat != nil {
			format = opts.OnGetFormat()
		}
		videoCodec := ""
		if opts.OnGetVideoCodec != nil {
			videoCodec = opts.OnGetVideoCodec()
		}
		width := 0
		if opts.OnGetWidth != nil {
			width = opts.OnGetWidth()
		}
		height := 0
		if opts.OnGetHeight != nil {
			height = opts.OnGetHeight()
		}
		aspectRatio := ""
		if opts.OnGetAspectRatio != nil {
			aspectRatio = opts.OnGetAspectRatio()
		}
		frameRate := 0.0
		if opts.OnGetFrameRate != nil {
			frameRate = opts.OnGetFrameRate()
		}
		bitrate := int64(0)
		if opts.OnGetBitrate != nil {
			bitrate = opts.OnGetBitrate()
		}
		pixelFormat := ""
		if opts.OnGetPixelFormat != nil {
			pixelFormat = opts.OnGetPixelFormat()
		}
		colorSpace := ""
		if opts.OnGetColorSpace != nil {
			colorSpace = opts.OnGetColorSpace()
		}
		colorRange := ""
		if opts.OnGetColorRange != nil {
			colorRange = opts.OnGetColorRange()
		}
		fieldOrder := ""
		if opts.OnGetFieldOrder != nil {
			fieldOrder = opts.OnGetFieldOrder()
		}
		gopSize := 0
		if opts.OnGetGOPSize != nil {
			gopSize = opts.OnGetGOPSize()
		}
		audioCodec := ""
		if opts.OnGetAudioCodec != nil {
			audioCodec = opts.OnGetAudioCodec()
		}
		audioBitrate := int64(0)
		if opts.OnGetAudioBitrate != nil {
			audioBitrate = opts.OnGetAudioBitrate()
		}
		audioRate := 0
		if opts.OnGetAudioRate != nil {
			audioRate = opts.OnGetAudioRate()
		}
		channels := 0
		if opts.OnGetChannels != nil {
			channels = opts.OnGetChannels()
		}
		duration := ""
		if opts.OnGetDuration != nil {
			duration = opts.OnGetDuration()
		}
		sar := ""
		if opts.OnGetSampleAspect != nil {
			sar = opts.OnGetSampleAspect()
		}
		hasChapters := false
		if opts.OnGetHasChapters != nil {
			hasChapters = opts.OnGetHasChapters()
		}
		hasMetadata := false
		if opts.OnGetHasMetadata != nil {
			hasMetadata = opts.OnGetHasMetadata()
		}

		metadata := fmt.Sprintf(
			"━━━ FILE INFO ━━━\n"+
				"Path: %s\n"+
				"File Size: %s\n"+
				"Format Family: %s\n"+
				"\n━━━ VIDEO ━━━\n"+
				"Codec: %s\n"+
				"Resolution: %dx%d\n"+
				"Aspect Ratio: %s\n"+
				"Frame Rate: %.2f fps\n"+
				"Bitrate: %s\n"+
				"Pixel Format: %s\n"+
				"Color Space: %s\n"+
				"Color Range: %s\n"+
				"Field Order: %s\n"+
				"GOP Size: %d\n"+
				"\n━━━ AUDIO ━━━\n"+
				"Codec: %s\n"+
				"Bitrate: %s\n"+
				"Sample Rate: %d Hz\n"+
				"Channels: %d\n"+
				"\n━━━ OTHER ━━━\n"+
				"Duration: %s\n"+
				"SAR (Pixel Aspect): %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			filepath.Base(path),
			fileSize,
			format,
			videoCodec,
			width, height,
			aspectRatio,
			frameRate,
			formatBitrateFull(bitrate),
			pixelFormat,
			colorSpace,
			colorRange,
			fieldOrder,
			gopSize,
			audioCodec,
			formatBitrateFull(audioBitrate),
			audioRate,
			channels,
			duration,
			sar,
			hasChapters,
			hasMetadata,
		)

		if opts.InspectInterlaceAnalyzing {
			metadata += "\n\n━━━ INTERLACING DETECTION ━━━\n"
			metadata += "Analyzing... (first 500 frames)"
		} else if opts.InspectInterlaceResult != nil {
			metadata += "\n\n━━━ INTERLACING DETECTION ━━━\n"
			metadata += "Results available"
		}

		return metadata
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
			metadataText.SetText(formatMetadata())
			videoContainer = container.NewCenter(widget.NewLabel("Video preview"))
		} else {
			fileLabel.SetText("No file loaded")
			metadataText.SetText("No file loaded")
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

	rightColumn := buildInspectBox("Metadata", metadataScroll)

	content := container.NewBorder(
		container.NewVBox(instructionsRow, actionButtons, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, leftColumn, rightColumn),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
