//go:build native_media

package inspect

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

const ModuleColor = "#629C1C"

var valueBg = utils.MustHex("#2B334A")
var valueBorder = utils.MustHex("#3A4360")

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

func BuildView(cb ViewCallbacks) fyne.CanvasObject {
	t := i18n.T()
	inspectColor := utils.MustHex(ModuleColor)

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleInspect), ui.BorderDim, func() {
		cb.ShowMainMenu()
	})

	queueBtn := ui.MakePillButton(t.ActionViewQueue, inspectColor, func() {
		cb.ShowQueue()
	})

	clearCompletedBtn := ui.MakePillButton("⌫", ui.BorderDim, func() {
		cb.ClearCompletedJobs()
	})

	topBar := ui.TintedBar(inspectColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	bottomBar := cb.ModuleFooter(nil)

	instructions := widget.NewLabel(t.InspectInstructions)
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	clearBtn := ui.MakePillButton(t.ActionClear, ui.BorderDim, func() {
		cb.ClearFile()
		cb.ShowInspectView()
	})

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
		colorTransfer := cb.GetColorTransfer()
		colorPrimaries := cb.GetColorPrimaries()
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

		colorTransferStr := colorTransfer
		if colorTransferStr == "smpte2084" {
			colorTransferStr = "PQ (HDR10)"
		} else if colorTransferStr == "bt1886" {
			colorTransferStr = "Rec. 1886"
		} else if colorTransferStr == "hlg" {
			colorTransferStr = "HLG (HDR)"
		} else if colorTransferStr == "arib-std-b67" {
			colorTransferStr = "HLG (ARIB)"
		}

		colorPrimariesStr := colorPrimaries
		if colorPrimariesStr == "bt2020" {
			colorPrimariesStr = "Rec. 2020"
		} else if colorPrimariesStr == "bt709" {
			colorPrimariesStr = "Rec. 709"
		} else if colorPrimariesStr == "bt601" {
			colorPrimariesStr = "Rec. 601"
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
			makeRow("Color Transfer", makeValuePill(colorTransferStr)),
			makeRow("Color Primaries", makeValuePill(colorPrimariesStr)),
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
	// Disable built-in controls: inspect manages playback state itself.
	player.Widget().DisableBuiltinControls()
	videoContainer := ui.BuildPlayerContainer(player.Widget(), fyne.NewSize(480, 270))

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

	openInspectFile := func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()
			cb.LoadFile(path)
		}, cb.Window())
	}
	loadBtn := ui.MakePillButton(t.ActionLoadVideo, inspectColor, openInspectFile)
	player.Widget().SetOnTapEmpty(openInspectFile)

	copyBtn := ui.MakePillButton(t.ActionCopyMetadata, ui.BorderDim, func() {
		metadata := formatMetadata()
		cb.Clipboard().SetContent(metadata)
		dialog.ShowInformation(t.DialogCopied, t.CompareCopiedFileMsg, cb.Window())
	})

	viewLogBtn := ui.MakePillButton(t.ActionCopyLog, ui.BorderDim, func() {
		dialog.ShowInformation(t.DialogNoLog, t.DialogNoLog, cb.Window())
	})
	viewLogBtn.Disable()

	actionButtons := container.NewHBox(loadBtn, copyBtn, viewLogBtn, clearBtn)

	editMetaBtn := ui.MakePillButton("Edit Metadata", ui.BorderDim, func() {
		currentTitle := cb.GetTitle()
		currentAuthor := ""
		currentDesc := ""
		if cb.GetFilePath() != "" {
			if src := cb.GetFilePath(); src != "" {
				if m := cb.GetFilePath(); m != "" {
				}
			}
		}

		titleEntry := widget.NewEntry()
		titleEntry.SetText(currentTitle)
		authorEntry := widget.NewEntry()
		authorEntry.SetText(currentAuthor)
		descEntry := widget.NewMultiLineEntry()
		descEntry.SetText(currentDesc)

		form := dialog.NewForm("Edit Metadata", "Save", "Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Title", titleEntry),
				widget.NewFormItem("Author", authorEntry),
				widget.NewFormItem("Description", descEntry),
			},
			func(confirmed bool) {
				if !confirmed {
					return
				}
				go func() {
					err := cb.SaveMetadata(titleEntry.Text, authorEntry.Text, descEntry.Text)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						if err != nil {
							dialog.ShowError(fmt.Errorf("failed to save metadata: %w", err), cb.Window())
						} else {
							dialog.ShowInformation("Metadata Saved", "Metadata updated successfully", cb.Window())
							cb.ShowInspectView()
						}
					}, false)
				}()
			}, cb.Window())
		form.Show()
	})
	actionButtons = container.NewHBox(loadBtn, editMetaBtn, copyBtn, viewLogBtn, clearBtn)

	if coverPath := cb.GetEmbeddedCoverArt(); coverPath != "" {
		exportCoverBtn := ui.MakePillButton("Export Cover", ui.BorderDim, func() {
			dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				destPath := writer.URI().Path()
				writer.Close()
				if err := copyFile(coverPath, destPath); err != nil {
					dialog.ShowError(fmt.Errorf("failed to export cover: %w", err), cb.Window())
				} else {
					dialog.ShowInformation("Cover Exported", fmt.Sprintf("Saved to: %s", destPath), cb.Window())
				}
			}, cb.Window())
		})
		actionButtons = container.NewHBox(loadBtn, copyBtn, exportCoverBtn, viewLogBtn, clearBtn)
	}

	exportJSONBtn := ui.MakePillButton("Export JSON", ui.BorderDim, func() {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			destPath := writer.URI().Path()
			writer.Close()

			jsonData := map[string]interface{}{
				"file":   cb.GetFilePath(),
				"format": cb.GetFormat(),
				"video": map[string]interface{}{
					"codec":          cb.GetVideoCodec(),
					"width":          cb.GetWidth(),
					"height":         cb.GetHeight(),
					"frameRate":      cb.GetFrameRate(),
					"bitrate":        cb.GetBitrate(),
					"pixelFmt":       cb.GetPixelFormat(),
					"colorSpace":     cb.GetColorSpace(),
					"colorRange":     cb.GetColorRange(),
					"colorTransfer":  cb.GetColorTransfer(),
					"colorPrimaries": cb.GetColorPrimaries(),
				},
				"audio": map[string]interface{}{
					"codec":    cb.GetAudioCodec(),
					"bitrate":  cb.GetAudioBitrate(),
					"rate":     cb.GetAudioRate(),
					"channels": cb.GetChannels(),
				},
				"chapters": cb.GetChapters(),
				"metadata": map[string]string{
					"title": cb.GetTitle(),
				},
			}
			jsonBytes, err := json.MarshalIndent(jsonData, "", "  ")
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to create JSON: %w", err), cb.Window())
				return
			}
			if err := os.WriteFile(destPath, jsonBytes, 0644); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save JSON: %w", err), cb.Window())
			} else {
				dialog.ShowInformation("JSON Exported", fmt.Sprintf("Saved to: %s", destPath), cb.Window())
			}
		}, cb.Window())
	})
	if coverPath := cb.GetEmbeddedCoverArt(); coverPath != "" {
		exportCoverBtn := ui.MakePillButton("Export Cover", ui.BorderDim, func() {
			dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				destPath := writer.URI().Path()
				writer.Close()
				if err := copyFile(coverPath, destPath); err != nil {
					dialog.ShowError(fmt.Errorf("failed to export cover: %w", err), cb.Window())
				} else {
					dialog.ShowInformation("Cover Exported", fmt.Sprintf("Saved to: %s", destPath), cb.Window())
				}
			}, cb.Window())
		})
		actionButtons = container.NewHBox(loadBtn, copyBtn, exportJSONBtn, exportCoverBtn, viewLogBtn, clearBtn)
	} else {
		actionButtons = container.NewHBox(loadBtn, copyBtn, exportJSONBtn, viewLogBtn, clearBtn)
	}

	var mainSplit *container.Split
	playerHdr, _ := ui.BuildCollapsibleHeader(t.ConvertSectionPlayer, inspectColor, func(open bool) {
		if open {
			mainSplit.SetOffset(0.5)
		} else {
			mainSplit.SetOffset(0.03)
		}
	})

	leftColumn := container.NewBorder(
		container.NewVBox(playerHdr, fileLabel),
		nil, nil, nil,
		videoContainer,
	)

	chaptersTab := container.NewVBox()
	if hasChapters := cb.GetHasChapters(); hasChapters {
		chapters := cb.GetChapters()
		if len(chapters) > 0 {
			for _, ch := range chapters {
				startDur := time.Duration(ch.StartTime * float64(time.Second))
				endDur := time.Duration(ch.EndTime * float64(time.Second))
				row := container.NewHBox(
					widget.NewLabel(fmt.Sprintf("%d", ch.Index)),
					widget.NewLabel(ch.Title),
					layout.NewSpacer(),
					widget.NewLabel(startDur.String()),
					widget.NewLabel(" → "),
					widget.NewLabel(endDur.String()),
				)
				chaptersTab.Add(row)
			}
		}
	} else {
		chaptersTab.Add(widget.NewLabel("No chapters"))
	}

	// Sync panel — live A/V sync diagnostics updated on every playback frame.
	clockPill := makeValuePill("--")
	audioPTSPill := makeValuePill("--")
	videoPTSPill := makeValuePill("--")
	deltaPill := makeValuePill("--")
	statusPill := makeValuePill("--")

	// setPillText walks the value-pill container to update the embedded label.
	// makeValuePill builds: container.NewMax(bg, container.NewPadded(lbl))
	setPillText := func(pill fyne.CanvasObject, text string) {
		c, ok := pill.(*fyne.Container)
		if !ok {
			return
		}
		for _, obj := range c.Objects {
			if inner, ok := obj.(*fyne.Container); ok {
				for _, o2 := range inner.Objects {
					if lbl, ok := o2.(*widget.Label); ok {
						lbl.SetText(text)
						return
					}
				}
			}
		}
	}
	setPillBG := func(pill fyne.CanvasObject, col color.Color) {
		c, ok := pill.(*fyne.Container)
		if !ok {
			return
		}
		if bg, ok := c.Objects[0].(*canvas.Rectangle); ok {
			bg.FillColor = col
			bg.Refresh()
		}
	}

	formatPTS := func(pts float64) string {
		if pts < 0 {
			return "--"
		}
		h := int(pts) / 3600
		m := (int(pts) % 3600) / 60
		s := int(pts) % 60
		ms := int((pts-float64(int(pts)))*1000)
		return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
	}

	syncRow := func(key string, pill fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil,
			widget.NewLabelWithStyle(key+":", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			nil, pill)
	}

	// Load state pills — updated by SetOnLoad milestones from the player.
	loadOpenPill := makeValuePill("--")
	loadFramePill := makeValuePill("--")
	loadReadyPill := makeValuePill("--")

	formatWall := func(t time.Time) string {
		return t.Format("15:04:05.000")
	}

	cb.Player().SetOnLoad(func(evt ui.LoadEvent) {
		// Already on main goroutine; update pills directly.
		switch evt.Phase {
		case ui.LoadPhaseStarted:
			setPillText(loadOpenPill, "--")
			setPillText(loadFramePill, "--")
			setPillText(loadReadyPill, "--")
			setPillBG(loadReadyPill, valueBg)
		case ui.LoadPhaseOpen:
			setPillText(loadOpenPill, formatWall(evt.At))
		case ui.LoadPhaseFirstFrame:
			setPillText(loadFramePill, formatWall(evt.At))
		case ui.LoadPhaseReady:
			setPillText(loadReadyPill, formatWall(evt.At))
			setPillBG(loadReadyPill, color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF})
		case ui.LoadPhaseFailed:
			errText := "Failed"
			if evt.Err != nil {
				errText = "Failed: " + evt.Err.Error()
			}
			setPillText(loadReadyPill, errText)
			setPillBG(loadReadyPill, color.RGBA{R: 0xFF, G: 0x4C, B: 0x4C, A: 0xFF})
		}
	})

	syncContent := container.NewVBox(
		widget.NewLabelWithStyle("Load State", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		syncRow("Engine open", loadOpenPill),
		syncRow("First frame", loadFramePill),
		syncRow("Ready", loadReadyPill),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("A/V Sync", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		syncRow("Clock", clockPill),
		syncRow("Audio PTS", audioPTSPill),
		syncRow("Video PTS", videoPTSPill),
		widget.NewSeparator(),
		syncRow("A/V Delta", deltaPill),
		syncRow("Status", statusPill),
		widget.NewSeparator(),
		widget.NewLabel("green <16ms · yellow <50ms · red ≥50ms"),
	)

	syncGreen := color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	syncYellow := color.RGBA{R: 0xFF, G: 0xD0, B: 0x00, A: 0xFF}
	syncRed := color.RGBA{R: 0xFF, G: 0x4C, B: 0x4C, A: 0xFF}

	cb.Player().SetOnProgress(func(_ float64) {
		clockT := cb.GetClockTime()
		aPTS := cb.GetLastAudioPTS()
		vPTS := cb.GetLastVideoPTS()

		var deltaStr, statusStr string
		var accentColor color.Color = valueBg

		if aPTS >= 0 && vPTS >= 0 {
			deltaMs := (vPTS - aPTS) * 1000
			if deltaMs > 0 {
				deltaStr = fmt.Sprintf("+%.1f ms (video ahead)", deltaMs)
			} else if deltaMs < 0 {
				deltaStr = fmt.Sprintf("%.1f ms (audio ahead)", deltaMs)
			} else {
				deltaStr = "0.0 ms"
			}
			abs := deltaMs
			if abs < 0 {
				abs = -abs
			}
			switch {
			case abs < 16:
				statusStr, accentColor = "SYNCED", syncGreen
			case abs < 50:
				statusStr, accentColor = "DRIFTING", syncYellow
			default:
				statusStr, accentColor = "OUT OF SYNC", syncRed
			}
		} else {
			deltaStr, statusStr = "--", "--"
		}

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setPillText(clockPill, formatPTS(clockT))
			setPillText(audioPTSPill, formatPTS(aPTS))
			setPillText(videoPTSPill, formatPTS(vPTS))
			setPillText(deltaPill, deltaStr)
			setPillText(statusPill, statusStr)
			setPillBG(deltaPill, accentColor)
			setPillBG(statusPill, accentColor)
		}, false)
	})

	metadataTabContent := container.NewScroll(metadataGrid)
	chaptersTabContent := container.NewScroll(chaptersTab)
	syncTabContent := container.NewScroll(syncContent)

	tabContainer := container.NewAppTabs(
		container.NewTabItem("Metadata", metadataTabContent),
		container.NewTabItem("Chapters", chaptersTabContent),
		container.NewTabItem("Sync", syncTabContent),
	)
	tabContainer.SetTabLocation(container.TabLocationTop)

	rightColumn := buildInspectBox("Information", tabContainer)

	mainSplit = container.NewHSplit(leftColumn, rightColumn)
	mainSplit.SetOffset(0.5)
	content := container.NewBorder(
		container.NewVBox(instructionsRow, actionButtons, widget.NewSeparator()),
		nil, nil, nil,
		mainSplit,
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
