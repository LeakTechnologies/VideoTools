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
	QRURL       string // URL embedded in the QR code (e.g. releases page)
	WebsiteURL  string // leaktechnologies.dev link shown in main content
	XProfileURL string
	XLabel      string
	// Translatable labels — populated by caller from i18n.T()
	TitleLabel      string // "About / Support"
	LogsFolderLabel string // "Logs Folder"
	ScanDocsLabel   string // "Dev Builds" (label under QR code)
	FeedbackLabel   string // feedback instructions text
	CloseLabel      string // "Close"
	OpenLabel       string // "Open" (for X/social link button)

	// LogoResource is the VT logo shown at the top of the right column.
	// If nil, falls back to loading VT_Logotype1.png from the filesystem.
	LogoResource fyne.Resource
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
	titleStr := opts.TitleLabel
	if titleStr == "" {
		titleStr = "About / Support"
	}
	title := canvas.NewText(titleStr, opts.TextColor)
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

	logsFolderStr := opts.LogsFolderLabel
	if logsFolderStr == "" {
		logsFolderStr = "Logs Folder"
	}
	logsLink := widget.NewButton(logsFolderStr, func() {
		if opts.OpenFolder == nil {
			return
		}
		if err := opts.OpenFolder(opts.LogsPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), opts.Window)
		}
	})
	logsLink.Importance = widget.LowImportance

	feedbackStr := opts.FeedbackLabel
	if feedbackStr == "" {
		feedbackStr = "Feedback: use the Logs button on the main menu to view logs; send issues with attached logs."
	}
	feedbackLabel := widget.NewLabel(feedbackStr)
	feedbackLabel.Wrapping = fyne.TextWrapWord

	openStr := opts.OpenLabel
	if openStr == "" {
		openStr = "Open"
	}
	xLabel := widget.NewLabel(opts.XLabel)
	xBtn := widget.NewButton(openStr, func() {
		if opts.OpenURL == nil {
			return
		}
		if err := opts.OpenURL(opts.XProfileURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open X profile: %w", err), opts.Window)
		}
	})
	xBtn.Importance = widget.LowImportance

	mainContentItems := []fyne.CanvasObject{
		versionText,
		devText,
		widget.NewLabel(""),
		container.NewHBox(xLabel, xBtn),
	}
	if opts.WebsiteURL != "" {
		if u, err := url.Parse(opts.WebsiteURL); err == nil {
			mainContentItems = append(mainContentItems, widget.NewHyperlink("leaktechnologies.dev", u))
		}
	}
	mainContentItems = append(mainContentItems, feedbackLabel)
	mainContent := container.NewVBox(mainContentItems...)

	logoColumn := container.NewVBox()
	if opts.LogoResource != nil {
		vtLogo := canvas.NewImageFromResource(opts.LogoResource)
		vtLogo.FillMode = canvas.ImageFillContain
		vtLogo.SetMinSize(fyne.NewSize(200, 80))
		logoColumn.Add(vtLogo)
	} else if vtLogo := loadLogo("VT_Logotype1.png", 96); vtLogo != nil {
		logoColumn.Add(vtLogo)
	}
	if ltLogo := loadLogo("LT_Logo-26.png", 72); ltLogo != nil {
		logoColumn.Add(ltLogo)
	}

	qrCode, err := generatePixelatedQRCode(opts.QRURL)
	if err != nil {
		if releasesURL, parseErr := url.Parse(opts.QRURL); parseErr == nil {
			logoColumn.Add(widget.NewHyperlink("Dev Builds", releasesURL))
		}
	} else {
		scanStr := opts.ScanDocsLabel
		if scanStr == "" {
			scanStr = "Scan for docs"
		}
		qrLabel := widget.NewLabel(scanStr)
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
	closeStr := opts.CloseLabel
	if closeStr == "" {
		closeStr = "Close"
	}
	dialog.ShowCustom(titleStr, closeStr, container.NewMax(sizeShim, body), opts.Window)
}
