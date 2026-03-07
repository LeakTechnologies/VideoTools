package about

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

type Options struct {
	Window      fyne.Window
	Version     string
	Developer   string
	LogsPath    string
	TextColor   color.Color
	OpenFolder  func(string) error
	OpenURL     func(string) error
	DocsURL     string
	XProfileURL string
	XLabel      string
}

func generatePixelatedQRCode(docURL string) (fyne.CanvasObject, error) {
	qrBytes, err := qrcode.Encode(docURL, qrcode.Low, 112)
	if err != nil {
		return nil, err
	}
	img := canvas.NewImageFromReader(bytes.NewReader(qrBytes), "qrcode.png")
	img.FillMode = canvas.ImageFillOriginal
	img.SetMinSize(fyne.NewSize(112, 112))
	return img, nil
}

func Show(opts Options) {
	title := canvas.NewText("About / Support", opts.TextColor)
	title.TextSize = 20

	versionText := widget.NewLabel(opts.Version)
	devText := widget.NewLabel(fmt.Sprintf("Developer: %s", opts.Developer))

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

	logsLink := widget.NewButton("Logs Folder", func() {
		if opts.OpenFolder == nil {
			return
		}
		if err := opts.OpenFolder(opts.LogsPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), opts.Window)
		}
	})
	logsLink.Importance = widget.LowImportance

	feedbackLabel := widget.NewLabel("Feedback: use the Logs button on the main menu to view logs; send issues with attached logs.")
	feedbackLabel.Wrapping = fyne.TextWrapWord

	xLabel := widget.NewLabel(opts.XLabel)
	xBtn := widget.NewButton("Open", func() {
		if opts.OpenURL == nil {
			return
		}
		if err := opts.OpenURL(opts.XProfileURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open X profile: %w", err), opts.Window)
		}
	})
	xBtn.Importance = widget.LowImportance

	mainContent := container.NewVBox(
		versionText,
		devText,
		widget.NewLabel(""),
		container.NewHBox(xLabel, xBtn),
		feedbackLabel,
	)

	logoColumn := container.NewVBox()
	if vtLogo := loadLogo("VT_Logo.png", 96); vtLogo != nil {
		logoColumn.Add(vtLogo)
	}
	if ltLogo := loadLogo("LT_Logo-26.png", 72); ltLogo != nil {
		logoColumn.Add(ltLogo)
	}

	qrCode, err := generatePixelatedQRCode(opts.DocsURL)
	if err != nil {
		if docURL, parseErr := url.Parse(opts.DocsURL); parseErr == nil {
			logoColumn.Add(widget.NewHyperlink("View Documentation", docURL))
		}
	} else {
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
	dialog.ShowCustom("About & Support", "Close", container.NewMax(sizeShim, body), opts.Window)
}
