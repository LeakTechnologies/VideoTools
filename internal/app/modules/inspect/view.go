//go:build native_media

package inspect

import (
	"fmt"
	"image/color"
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
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const ModuleColor = "#4CAF50"

var valueBg = utils.MustHex("#2B334A")
var valueBorder = utils.MustHex("#3A4360")

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

func BuildView(cb ViewCallbacks) fyne.CanvasObject {
	t := i18n.T()
	inspectColor := utils.MustHex(ModuleColor)

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleInspect), func() {
		cb.ShowMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		cb.ShowQueue()
	})

	clearCompletedBtn := widget.NewButton("⌫", func() {
		cb.ClearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(inspectColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	statsBar := cb.StatsBar()
	bottomBar := container.NewVBox(layout.NewSpacer(), statsBar)

	instructions := widget.NewLabel(t.InspectInstructions)
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	clearBtn := widget.NewButton(t.ActionClear, func() {
		cb.ClearFile()
		cb.ShowInspectView()
	})
	clearBtn.Importance = widget.LowImportance

	instructionsRow := container.NewBorder(nil, nil, nil, nil, instructions)

	fileLabel := widget.NewLabel(t.LabelNoFile)
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

	metadataPlaceholder := container.NewCenter(widget.NewLabel(t.LabelNoFile))

	metadataGrid := container.NewMax(metadataPlaceholder)

	buildMetadataGrid := func() fyne.CanvasObject {
		inspectFile := cb.GetFilePath()
		if inspectFile == "" {
			return metadataPlaceholder
		}

		format := cb.GetFormat()
		videoCodec := cb.GetVideoCodec()
		width := cb.GetWidth()
		height := cb.GetHeight()
		aspectRatio := cb.GetAspectRatio()
		frameRate := cb.GetFrameRate()
		bitrate := cb.GetBitrate()
		pixelFmt := cb.GetPixelFormat()
		colorSpace := cb.GetColorSpace()
		colorRange := cb.GetColorRange()
		fieldOrder := cb.GetFieldOrder()
		gopSize := cb.GetGOPSize()
		audioCodec := cb.GetAudioCodec()
		audioBitrate := cb.GetAudioBitrate()
		audioRate := cb.GetAudioRate()
		channels := cb.GetChannels()
		duration := cb.GetDuration()
		sar := cb.GetSampleAspect()
		hasChapters := cb.GetHasChapters()
		hasMetadata := cb.GetHasMetadata()

		fileSize := "Unknown"
		if fi, err := os.Stat(inspectFile); err == nil {
			fileSize = utils.FormatBytes(fi.Size())
		}

		bitrateStr := "--"
		if bitrate > 0 {
			bitrateStr = fmt.Sprintf("%d kbps", bitrate/1000)
		}
		audioBitrateStr := "--"
		if audioBitrate > 0 {
			audioBitrateStr = fmt.Sprintf("%d kbps", audioBitrate/1000)
		}

		parStr := sar
		if parStr == "1:1" || parStr == "" {
			parStr = "1:1 (Square)"
		} else {
			parStr += " (Non-square)"
		}

		if colorRange == "tv" {
			colorRange = "Limited (TV)"
		} else if colorRange == "pc" || colorRange == "jpeg" {
			colorRange = "Full (PC)"
		}

		interlacing := "Progressive"
		if fieldOrder != "" && fieldOrder != "progressive" && fieldOrder != "unknown" && fieldOrder != "Unknown" {
			interlacing = "Interlaced (" + fieldOrder + ")"
		}

		gopStr := "--"
		if gopSize > 0 {
			gopStr = fmt.Sprintf("%d frames", gopSize)
		}

		chaptersStr := "No"
		if hasChapters {
			chaptersStr = "Yes"
		}
		metadataStr := "No"
		if hasMetadata {
			metadataStr = "Yes"
		}

		_ = fmt.Sprintf("Format: %s\nResolution: %dx%d\nDuration: %s\nFile Size: %s",
			format, width, height, duration, fileSize)

		title := cb.GetTitle()

		col1Rows := []fyne.CanvasObject{
			makeRow("Format", makeValuePill(format)),
			makeRow("Resolution", makeValuePill(fmt.Sprintf("%dx%d", width, height))),
			makeRow("Aspect Ratio", makeValuePill(aspectRatio)),
			makeRow("Duration", makeValuePill(duration)),
			makeRow("Frame Rate", makeValuePill(fmt.Sprintf("%.2f fps", frameRate))),
			makeRow("Interlacing", makeValuePill(interlacing)),
			makeRow("Color Space", makeValuePill(colorSpace)),
			makeRow("Color Range", makeValuePill(colorRange)),
			makeRow("GOP Size", makeValuePill(gopStr)),
			makeRow("File Size", makeValuePill(fileSize)),
		}
		if title != "" {
			col1Rows = append([]fyne.CanvasObject{makeRow("Title", makeValuePill(title))}, col1Rows...)
		}
		col1 := container.NewVBox(col1Rows...)

		col2 := container.NewVBox(
			makeRow("Video Codec", makeValuePillWithChip(videoCodec, ui.GetVideoCodecColor(videoCodec))),
			makeRow("Video Bitrate", makeValuePill(bitrateStr)),
			makeRow("Pixel Format", makeValuePill(pixelFmt)),
			makeRow("Pixel AR", makeValuePill(parStr)),
			makeRow("Audio Codec", makeValuePillWithChip(audioCodec, ui.GetAudioCodecColor(audioCodec))),
			makeRow("Audio Bitrate", makeValuePill(audioBitrateStr)),
			makeRow("Audio Rate", makeValuePill(fmt.Sprintf("%d Hz", audioRate))),
			makeRow("Channels", makeValuePill(utils.ChannelLabel(channels))),
			makeRow("Chapters", makeValuePill(chaptersStr)),
			makeRow("Metadata", makeValuePill(metadataStr)),
		)

		rows := []fyne.CanvasObject{container.NewGridWithColumns(2, col1, col2)}
		return container.NewVBox(rows...)
	}

	formatMetadata := func() string {
		inspectFile := cb.GetFilePath()
		if inspectFile == "" {
			return t.LabelNoFile
		}
		bitrate := cb.GetBitrate()
		audioBitrate := cb.GetAudioBitrate()
		bitrateStr := "--"
		if bitrate > 0 {
			bitrateStr = fmt.Sprintf("%d kbps", bitrate/1000)
		}
		audioBitrateStr := "--"
		if audioBitrate > 0 {
			audioBitrateStr = fmt.Sprintf("%d kbps", audioBitrate/1000)
		}
		width := cb.GetWidth()
		height := cb.GetHeight()
		fr := cb.GetFrameRate()
		ar := cb.GetAudioRate()
		ch := cb.GetChannels()
		return fmt.Sprintf(
			"File: %s\nFormat: %s\nResolution: %dx%d\nAspect Ratio: %s\nDuration: %s\n"+
				"Video Codec: %s\nVideo Bitrate: %s\nFrame Rate: %.2f fps\n"+
				"Pixel Format: %s\nColor Space: %s\nField Order: %s\n"+
				"Audio Codec: %s\nAudio Bitrate: %s\nAudio Rate: %d Hz\nChannels: %d",
			filepath.Base(inspectFile),
			cb.GetFormat(),
			width, height,
			cb.GetAspectRatio(),
			cb.GetDuration(),
			cb.GetVideoCodec(), bitrateStr, fr,
			cb.GetPixelFormat(), cb.GetColorSpace(), cb.GetFieldOrder(),
			cb.GetAudioCodec(), audioBitrateStr, ar, ch,
		)
	}

	player := cb.Player()
	var videoContainer fyne.CanvasObject
	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.SetMinSize(fyne.NewSize(480, 270))
	if cb.GetFilePath() != "" {
		// The adapter is responsible for calling player.Load — the view just embeds
		// the widget. This avoids re-triggering a load on every view rebuild.
		videoContainer = container.NewMax(bg, player.Widget())
	} else {
		videoContainer = container.NewMax(
			bg,
			container.NewCenter(widget.NewLabel("Load a video to preview")),
		)
	}

	updateDisplay := func() {
		inspectFile := cb.GetFilePath()
		if inspectFile != "" {
			filename := "Unknown"
			if p := inspectFile; p != "" {
				filename = filepath.Base(p)
			}
			if len(filename) > 50 {
				ext := filepath.Ext(filename)
				nameWithoutExt := strings.TrimSuffix(filename, ext)
				availableLen := 47 - len(ext)
				if availableLen < 1 {
					filename = filename[:47] + "..."
				} else {
					filename = nameWithoutExt[:availableLen] + "..." + ext
				}
			}
			fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filename))
			metadataGrid.Objects = []fyne.CanvasObject{buildMetadataGrid()}
			metadataGrid.Refresh()
		} else {
			fileLabel.SetText(t.LabelNoFile)
			metadataGrid.Objects = []fyne.CanvasObject{metadataPlaceholder}
			metadataGrid.Refresh()
		}
	}

	updateDisplay()

	loadBtn := widget.NewButton(t.ActionLoadVideo, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()
			cb.LoadFile(path)
		}, cb.Window())
	})

	copyBtn := widget.NewButton(t.ActionCopyMetadata, func() {
		metadata := formatMetadata()
		cb.Clipboard().SetContent(metadata)
		dialog.ShowInformation(t.DialogCopied, t.CompareCopiedFileMsg, cb.Window())
	})
	copyBtn.Importance = widget.LowImportance

	viewLogBtn := widget.NewButton(t.ActionCopyLog, func() {
		dialog.ShowInformation(t.DialogNoLog, t.DialogNoLog, cb.Window())
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

