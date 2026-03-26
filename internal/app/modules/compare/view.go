package compare

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
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"image/color"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	CompareFile1 interface{}
	CompareFile2 interface{}
	QueueBtn     *widget.Button

	OnShowMainMenu           func()
	OnShowQueue              func()
	OnShowCompareFullscreen  func()
	OnRefreshView            func()
	OnUpdateQueueButtonLabel func()
	OnGetStatsBar            func() fyne.CanvasObject
	OnGetCompareFooter       func(content fyne.CanvasObject) fyne.CanvasObject
	OnProbeVideo             func(path string) (interface{}, error)
	OnBuildVideoPane         func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject
}

func toVideoSource(v interface{}) *VideoSource {
	if v == nil {
		return nil
	}
	if vs, ok := v.(*VideoSource); ok {
		return vs
	}
	return nil
}

// VideoSource holds probed metadata for a video file loaded into the compare module.
type VideoSource struct {
	Path              string
	Format            string
	VideoCodec        string
	Width             int
	Height            int
	FrameRate         float64
	Bitrate           int
	PixelFormat       string
	ColorSpace        string
	ColorRange        string
	FieldOrder        string
	GOPSize           int
	AudioCodec        string
	AudioBitrate      int
	AudioRate         int
	Channels          int
	SampleAspectRatio string
	HasChapters       bool
	HasMetadata       bool
	Duration          float64
}

func (v *VideoSource) AspectRatioString() string {
	if v.Height == 0 {
		return "N/A"
	}
	gcd := func(a, b int) int {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}
	g := gcd(v.Width, v.Height)
	return fmt.Sprintf("%d:%d", v.Width/g, v.Height/g)
}

func (v *VideoSource) DurationString() string {
	if v.Duration <= 0 {
		return "N/A"
	}
	h := int(v.Duration) / 3600
	m := (int(v.Duration) % 3600) / 60
	s := int(v.Duration) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func BuildView(opts Options) fyne.CanvasObject {
	compareColor := opts.ModuleColor
	if compareColor == nil {
		compareColor = utils.MustHex("#E64A19")
	}
	t := i18n.T()

	file1 := toVideoSource(opts.CompareFile1)
	file2 := toVideoSource(opts.CompareFile2)

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleCompare), func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})
	if opts.QueueBtn != nil {
		opts.QueueBtn = queueBtn
	}
	if opts.OnUpdateQueueButtonLabel != nil {
		opts.OnUpdateQueueButtonLabel()
	}
	playerVisible := true
	togglePlayerBtn := widget.NewButton(t.CompareHidePlayer, nil)

	topBar := ui.TintedBar(compareColor, container.NewHBox(backBtn, layout.NewSpacer(), togglePlayerBtn, queueBtn))
	statsBar := opts.OnGetStatsBar()
	var bottomBar fyne.CanvasObject
	if opts.OnGetCompareFooter != nil {
		bottomBar = opts.OnGetCompareFooter(layout.NewSpacer())
	} else {
		bottomBar = container.NewVBox(statsBar, layout.NewSpacer())
	}

	instructions := widget.NewLabel(t.CompareInstructions)
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	fullscreenBtn := widget.NewButton(t.CompareFullscreen, func() {
		if file1 == nil && file2 == nil {
			dialog.ShowInformation(t.CompareNoVideosTitle, t.CompareNoVideosFSMsg, opts.Window)
			return
		}
		if opts.OnShowCompareFullscreen != nil {
			opts.OnShowCompareFullscreen()
		}
	})
	fullscreenBtn.Importance = widget.MediumImportance

	copyComparisonBtn := widget.NewButton(t.CompareCopyReport, func() {
		if file1 == nil && file2 == nil {
			dialog.ShowInformation(t.CompareNoVideosTitle, t.CompareNoVideosCopyMsg, opts.Window)
			return
		}

		var comparisonText strings.Builder
		comparisonText.WriteString("-----------------------------------------------------------------------\n")
		comparisonText.WriteString("                        VIDEO COMPARISON REPORT\n")
		comparisonText.WriteString("-----------------------------------------------------------------------\n\n")

		file1Name := "Not loaded"
		file2Name := "Not loaded"
		if file1 != nil {
			file1Name = filepath.Base(file1.Path)
		}
		if file2 != nil {
			file2Name = filepath.Base(file2.Path)
		}

		comparisonText.WriteString(fmt.Sprintf("FILE 1: %s\n", file1Name))
		comparisonText.WriteString(fmt.Sprintf("FILE 2: %s\n", file2Name))
		comparisonText.WriteString("\n\n")

		getField := func(src *VideoSource, getter func(*VideoSource) string) string {
			if src == nil {
				return ""
			}
			return getter(src)
		}

		comparisonText.WriteString(" FILE INFO \n")

		var file1SizeBytes int64
		file1Size := getField(file1, func(src *VideoSource) string {
			if fi, err := os.Stat(src.Path); err == nil {
				file1SizeBytes = fi.Size()
				return utils.FormatBytes(fi.Size())
			}
			return "Unknown"
		})
		file2Size := getField(file2, func(src *VideoSource) string {
			if fi, err := os.Stat(src.Path); err == nil {
				if file1SizeBytes > 0 {
					return utils.DeltaBytes(fi.Size(), file1SizeBytes)
				}
				return utils.FormatBytes(fi.Size())
			}
			return "Unknown"
		})

		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n", "File Size:", file1Size, file2Size))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Format Family:",
			getField(file1, func(s *VideoSource) string { return s.Format }),
			getField(file2, func(s *VideoSource) string { return s.Format })))

		comparisonText.WriteString("\n VIDEO \n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Codec:",
			getField(file1, func(s *VideoSource) string { return s.VideoCodec }),
			getField(file2, func(s *VideoSource) string { return s.VideoCodec })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Resolution:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%dx%d", s.Width, s.Height) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%dx%d", s.Width, s.Height) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Aspect Ratio:",
			getField(file1, func(s *VideoSource) string { return s.AspectRatioString() }),
			getField(file2, func(s *VideoSource) string { return s.AspectRatioString() })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Frame Rate:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%.2f fps", s.FrameRate) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%.2f fps", s.FrameRate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Bitrate:",
			getField(file1, func(s *VideoSource) string { return formatBitrateFull(s.Bitrate) }),
			getField(file2, func(s *VideoSource) string {
				if file1 != nil {
					return utils.DeltaBitrate(s.Bitrate, file1.Bitrate)
				}
				return formatBitrateFull(s.Bitrate)
			})))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Pixel Format:",
			getField(file1, func(s *VideoSource) string { return s.PixelFormat }),
			getField(file2, func(s *VideoSource) string { return s.PixelFormat })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Color Space:",
			getField(file1, func(s *VideoSource) string { return s.ColorSpace }),
			getField(file2, func(s *VideoSource) string { return s.ColorSpace })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Color Range:",
			getField(file1, func(s *VideoSource) string { return s.ColorRange }),
			getField(file2, func(s *VideoSource) string { return s.ColorRange })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Field Order:",
			getField(file1, func(s *VideoSource) string { return s.FieldOrder }),
			getField(file2, func(s *VideoSource) string { return s.FieldOrder })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"GOP Size:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%d", s.GOPSize) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%d", s.GOPSize) })))

		comparisonText.WriteString("\n AUDIO \n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Codec:",
			getField(file1, func(s *VideoSource) string { return s.AudioCodec }),
			getField(file2, func(s *VideoSource) string { return s.AudioCodec })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Bitrate:",
			getField(file1, func(s *VideoSource) string { return formatBitrateFull(s.AudioBitrate) }),
			getField(file2, func(s *VideoSource) string { return formatBitrateFull(s.AudioBitrate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Sample Rate:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%d Hz", s.AudioRate) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%d Hz", s.AudioRate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Channels:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%d", s.Channels) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%d", s.Channels) })))

		comparisonText.WriteString("\n OTHER \n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Duration:",
			getField(file1, func(s *VideoSource) string { return s.DurationString() }),
			getField(file2, func(s *VideoSource) string { return s.DurationString() })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"SAR (Pixel Aspect):",
			getField(file1, func(s *VideoSource) string { return s.SampleAspectRatio }),
			getField(file2, func(s *VideoSource) string { return s.SampleAspectRatio })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Chapters:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%v", s.HasChapters) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%v", s.HasChapters) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Metadata:",
			getField(file1, func(s *VideoSource) string { return fmt.Sprintf("%v", s.HasMetadata) }),
			getField(file2, func(s *VideoSource) string { return fmt.Sprintf("%v", s.HasMetadata) })))

		comparisonText.WriteString("\n-----------------------------------------------------------------------\n")

		opts.Window.Clipboard().SetContent(comparisonText.String())
		dialog.ShowInformation(t.CompareCopied, t.CompareCopiedMsg, opts.Window)
	})
	copyComparisonBtn.Importance = widget.LowImportance

	clearAllBtn := widget.NewButton(t.ActionClearAll, func() {
		file1 = nil
		file2 = nil
		if opts.OnRefreshView != nil {
			opts.OnRefreshView()
		}
	})
	clearAllBtn.Importance = widget.LowImportance

	buildCompareBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
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

	instructionsRow := container.NewBorder(nil, nil, nil, container.NewHBox(fullscreenBtn, copyComparisonBtn, clearAllBtn), instructions)

	file1Label := widget.NewLabel(t.CompareFile1NotLoaded)
	file1Label.TextStyle = fyne.TextStyle{Bold: true}

	file2Label := widget.NewLabel(t.CompareFile2NotLoaded)
	file2Label.TextStyle = fyne.TextStyle{Bold: true}

	file1VideoContainer := container.NewMax()
	file2VideoContainer := container.NewMax()

	file1VideoContainer.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))}
	file2VideoContainer.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))}

	file1Info := widget.NewLabel(t.LabelNoFile)
	file1Info.Wrapping = fyne.TextWrapWord
	file1Info.TextStyle = fyne.TextStyle{}

	file2Info := widget.NewLabel(t.LabelNoFile)
	file2Info.Wrapping = fyne.TextWrapWord
	file2Info.TextStyle = fyne.TextStyle{}

	formatMetadata := func(src *VideoSource, ref *VideoSource) string {
		var (
			fileSize       = "Unknown"
			refSize  int64 = 0
		)
		if src == nil {
			return ""
		}
		if fi, err := os.Stat(src.Path); err == nil {
			if ref != nil {
				if rfi, err := os.Stat(ref.Path); err == nil {
					refSize = rfi.Size()
				}
			}
			if refSize > 0 {
				fileSize = utils.DeltaBytes(fi.Size(), refSize)
			} else {
				fileSize = utils.FormatBytes(fi.Size())
			}
		}

		var (
			bitrateStr = "--"
			refBitrate = 0
		)
		if ref != nil {
			refBitrate = ref.Bitrate
		}
		if src.Bitrate > 0 {
			if refBitrate > 0 {
				bitrateStr = utils.DeltaBitrate(src.Bitrate, refBitrate)
			} else {
				bitrateStr = formatBitrateFull(src.Bitrate)
			}
		}

		return fmt.Sprintf(
			" FILE INFO \n"+
				"Path: %s\n"+
				"File Size: %s\n"+
				"Format Family: %s\n"+
				"\n VIDEO \n"+
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
				"\n AUDIO \n"+
				"Codec: %s\n"+
				"Bitrate: %s\n"+
				"Sample Rate: %d Hz\n"+
				"Channels: %d\n"+
				"\n OTHER \n"+
				"Duration: %s\n"+
				"SAR (Pixel Aspect): %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			filepath.Base(src.Path),
			fileSize,
			src.Format,
			src.VideoCodec,
			src.Width, src.Height,
			src.AspectRatioString(),
			src.FrameRate,
			bitrateStr,
			src.PixelFormat,
			src.ColorSpace,
			src.ColorRange,
			src.FieldOrder,
			src.GOPSize,
			src.AudioCodec,
			formatBitrate(src.AudioBitrate),
			src.AudioRate,
			src.Channels,
			src.DurationString(),
			src.SampleAspectRatio,
			src.HasChapters,
			src.HasMetadata,
		)
	}

	truncateFilename := func(filename string, maxLen int) string {
		if len(filename) <= maxLen {
			return filename
		}
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)

		if len(ext) > 10 {
			return filename[:maxLen-3] + "..."
		}

		availableLen := maxLen - len(ext) - 3
		if availableLen < 1 {
			return filename[:maxLen-3] + "..."
		}
		return nameWithoutExt[:availableLen] + "..." + ext
	}

	updateFile1 := func() {
		if file1 != nil {
			filename := filepath.Base(file1.Path)
			displayName := truncateFilename(filename, 35)
			file1Label.SetText(fmt.Sprintf(t.CompareFile1Fmt, displayName))
			file1Info.SetText(formatMetadata(file1, file2))
			if opts.OnBuildVideoPane != nil {
				file1VideoContainer.Objects = []fyne.CanvasObject{
					opts.OnBuildVideoPane(nil, fyne.NewSize(320, 180), file1, nil),
				}
			}
			file1VideoContainer.Refresh()
		} else {
			file1Label.SetText(t.CompareFile1NotLoaded)
			file1Info.SetText(t.LabelNoFile)
			file1VideoContainer.Objects = []fyne.CanvasObject{
				container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded)),
			}
			file1VideoContainer.Refresh()
		}
	}

	updateFile2 := func() {
		if file2 != nil {
			filename := filepath.Base(file2.Path)
			displayName := truncateFilename(filename, 35)
			file2Label.SetText(fmt.Sprintf(t.CompareFile2Fmt, displayName))
			file2Info.SetText(formatMetadata(file2, file1))
			if opts.OnBuildVideoPane != nil {
				file2VideoContainer.Objects = []fyne.CanvasObject{
					opts.OnBuildVideoPane(nil, fyne.NewSize(320, 180), file2, nil),
				}
			}
			file2VideoContainer.Refresh()
		} else {
			file2Label.SetText(t.CompareFile2NotLoaded)
			file2Info.SetText(t.LabelNoFile)
			file2VideoContainer.Objects = []fyne.CanvasObject{
				container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded)),
			}
			file2VideoContainer.Refresh()
		}
	}

	updateFile1()
	updateFile2()

	file1SelectBtn := widget.NewButton(t.CompareLoadFile1, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			if opts.OnProbeVideo != nil {
				src, err := opts.OnProbeVideo(path)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), opts.Window)
					return
				}

				file1 = toVideoSource(src)
				opts.CompareFile1 = file1
				updateFile1()
			}
		}, opts.Window)
	})

	file2SelectBtn := widget.NewButton(t.CompareLoadFile2, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			if opts.OnProbeVideo != nil {
				src, err := opts.OnProbeVideo(path)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), opts.Window)
					return
				}

				file2 = toVideoSource(src)
				opts.CompareFile2 = file2
				updateFile2()
			}
		}, opts.Window)
	})

	file1CopyBtn := widget.NewButton(t.ActionCopyMetadata, func() {
		if file1 == nil {
			return
		}
		metadata := formatMetadata(file1, file2)
		opts.Window.Clipboard().SetContent(metadata)
		dialog.ShowInformation(t.CompareCopied, t.CompareCopiedFileMsg, opts.Window)
	})
	file1CopyBtn.Importance = widget.LowImportance

	file1ClearBtn := widget.NewButton(t.ActionClear, func() {
		file1 = nil
		updateFile1()
	})
	file1ClearBtn.Importance = widget.LowImportance

	file2CopyBtn := widget.NewButton(t.ActionCopyMetadata, func() {
		if file2 == nil {
			return
		}
		metadata := formatMetadata(file2, file1)
		opts.Window.Clipboard().SetContent(metadata)
		dialog.ShowInformation(t.CompareCopied, t.CompareCopiedFileMsg, opts.Window)
	})
	file2CopyBtn.Importance = widget.LowImportance

	file2ClearBtn := widget.NewButton(t.ActionClear, func() {
		file2 = nil
		updateFile2()
	})
	file2ClearBtn.Importance = widget.LowImportance

	file1Header := container.NewVBox(
		file1Label,
		container.NewHBox(file1SelectBtn, file1CopyBtn, file1ClearBtn),
	)

	file2Header := container.NewVBox(
		file2Label,
		container.NewHBox(file2SelectBtn, file2CopyBtn, file2ClearBtn),
	)

	file1InfoScroll := container.NewVScroll(file1Info)
	file2InfoScroll := container.NewVScroll(file2Info)

	file1MetaBox := buildCompareBox(t.CompareFile1Info, file1InfoScroll)
	file2MetaBox := buildCompareBox(t.CompareFile2Info, file2InfoScroll)

	file1PlayerRow := container.NewVBox(file1VideoContainer, widget.NewSeparator())
	file2PlayerRow := container.NewVBox(file2VideoContainer, widget.NewSeparator())

	file1Column := container.NewBorder(
		container.NewVBox(
			file1Header,
			widget.NewSeparator(),
			file1PlayerRow,
		),
		nil, nil, nil,
		file1MetaBox,
	)

	file2Column := container.NewBorder(
		container.NewVBox(
			file2Header,
			widget.NewSeparator(),
			file2PlayerRow,
		),
		nil, nil, nil,
		file2MetaBox,
	)

	togglePlayerBtn.OnTapped = func() {
		playerVisible = !playerVisible
		if playerVisible {
			file1PlayerRow.Show()
			file2PlayerRow.Show()
			togglePlayerBtn.SetText(t.CompareHidePlayer)
		} else {
			file1PlayerRow.Hide()
			file2PlayerRow.Hide()
			togglePlayerBtn.SetText(t.CompareShowPlayer)
		}
	}

	content := container.NewBorder(
		container.NewVBox(instructionsRow, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, file1Column, file2Column),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}


func formatBitrate(bps int) string {
	if bps == 0 {
		return "N/A"
	}
	kbps := float64(bps) / 1000.0
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps", kbps/1000.0)
	}
	return fmt.Sprintf("%.0f kbps", kbps)
}

func formatBitrateFull(bps int) string {
	if bps <= 0 {
		return "N/A"
	}
	kbps := float64(bps) / 1000.0
	mbps := kbps / 1000.0
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps (%.0f kbps)", mbps, kbps)
	}
	return fmt.Sprintf("%.0f kbps (%.2f Mbps)", kbps, mbps)
}
