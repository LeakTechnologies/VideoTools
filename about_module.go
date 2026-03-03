package main

import (
	"bytes"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/skip2/go-qrcode"
)

func generatePixelatedQRCode() (fyne.CanvasObject, error) {
	docURL := "https://docs.leaktechnologies.dev/VideoTools"

	// Generate QR code with fewer pixels for a chunkier, blockier look
	qrBytes, err := qrcode.Encode(docURL, qrcode.Low, 112)
	if err != nil {
		return nil, err
	}

	// Convert to Fyne image with pixelated look
	img := canvas.NewImageFromReader(bytes.NewReader(qrBytes), "qrcode.png")
	img.FillMode = canvas.ImageFillOriginal // Keep pixelated look
	img.SetMinSize(fyne.NewSize(112, 112))

	return img, nil
}

func (s *appState) showAbout() {
	version := fmt.Sprintf("VideoTools %s", versionWithPlatform())
	dev := "Leak Technologies"
	logsPath := getLogsDir()

	title := canvas.NewText("About / Support", textColor)
	title.TextSize = 20

	versionText := widget.NewLabel(version)
	devText := widget.NewLabel(fmt.Sprintf("Developer: %s", dev))

	loadLogo := func(name string, size float32) fyne.CanvasObject {
		candidates := []string{filepath.Join("assets", "logo", name)}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates, filepath.Join(dir, "assets", "logo", name))
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				img := canvas.NewImageFromFile(p)
				img.FillMode = canvas.ImageFillContain
				img.SetMinSize(fyne.NewSize(size, size))
				return img
			}
		}
		return nil
	}

	vtLogo := loadLogo("VT_Logo.png", 96)
	ltLogo := loadLogo("LT_Logo-26.png", 72)

	logsLink := widget.NewButton("Logs Folder", func() {
		if err := openFolder(logsPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), s.window)
		}
	})
	logsLink.Importance = widget.LowImportance

	feedbackLabel := widget.NewLabel("Feedback: use the Logs button on the main menu to view logs; send issues with attached logs.")
	feedbackLabel.Wrapping = fyne.TextWrapWord

	// X (Twitter) account
	xURL := "https://x.com/VT_VideoTools"
	xLabel := widget.NewLabel("X: @VT_VideoTools")
	xBtn := widget.NewButton("Open", func() {
		if err := openURL(xURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open X profile: %w", err), s.window)
		}
	})
	xBtn.Importance = widget.LowImportance
	xRow := container.NewHBox(xLabel, xBtn)

	mainContent := container.NewVBox(
		versionText,
		devText,
		widget.NewLabel(""),
		xRow,
		feedbackLabel,
	)

	logoColumn := container.NewVBox()
	if vtLogo != nil {
		logoColumn.Add(vtLogo)
	}
	if ltLogo != nil {
		logoColumn.Add(ltLogo)
	}

	// Add QR code for documentation
	qrCode, err := generatePixelatedQRCode()
	if err != nil {
		// Fallback to hyperlink if QR generation fails
		docURL, _ := url.Parse("https://docs.leaktechnologies.dev/VideoTools")
		fallbackLink := widget.NewHyperlink("View Documentation", docURL)
		logoColumn.Add(fallbackLink)
	} else {
		// Add QR code with label
		qrLabel := widget.NewLabel("Scan for docs")
		qrLabel.Alignment = fyne.TextAlignCenter
		logoColumn.Add(qrCode)
		logoColumn.Add(qrLabel)
	}

	logoColumn.Add(layout.NewSpacer())
	logoColumn.Add(logsLink)

	body := container.NewBorder(
		container.NewHBox(title),
		nil,
		nil,
		logoColumn,
		mainContent,
	)
	body = container.NewPadded(body)
	sizeShim := canvas.NewRectangle(color.Transparent)
	sizeShim.SetMinSize(fyne.NewSize(560, 280))
	dialog.ShowCustom("About & Support", "Close", container.NewMax(sizeShim, body), s.window)
}
