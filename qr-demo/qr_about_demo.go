package main

import (
	"bytes"
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/skip2/go-qrcode"
)

func generatePixelatedQRCode() (fyne.CanvasObject, error) {
	docURL := "https://github.com/LeakTechnologies/VideoTools/wiki"

	// Generate QR code with large pixels for blocky look (160x160 with 8x8 pixel blocks)
	qrBytes, err := qrcode.Encode(docURL, qrcode.Medium, 160)
	if err != nil {
		return nil, err
	}

	// Convert to Fyne image with pixelated look
	img := canvas.NewImageFromBytes(qrBytes)
	img.FillMode = canvas.ImageFillOriginal // Keep pixelated look
	img.SetMinSize(fyne.NewSize(160, 160))

	return img, nil
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("QR Code Test - About Dialog Demo")

	// Test QR generation
	qrCode, err := generatePixelatedQRCode()
	if err != nil {
		log.Printf("Failed to generate QR code: %v", err)
		fallback := widget.NewLabel("QR generation failed - using fallback")
		myWindow.SetContent(container.NewVBox(fallback))
	} else {
		// Recreate about dialog layout with QR code
		title := canvas.NewText("About & Support", color.Color{} /*textColor*/)
		title.TextSize = 20

		versionText := widget.NewLabel("VideoTools QR Code Demo")
		devText := widget.NewLabel("Developer: Leak Technologies")

		// QR code with label
		qrLabel := widget.NewLabel("Scan for docs")
		qrLabel.Alignment = fyne.TextAlignCenter

		// Logs button
		logsLink := widget.NewButton("Logs Folder", func() {
			fmt.Println("Logs folder clicked")
		})
		logsLink.Importance = widget.LowImportance

		feedbackLabel := widget.NewLabel("Feedback: use Logs button on main menu to view logs; send issues with attached logs.")
		feedbackLabel.Wrapping = fyne.TextWrapWord

		mainContent := container.NewVBox(
			versionText,
			devText,
			widget.NewLabel(""),
			widget.NewLabel("Support Development"),
			widget.NewLabel("QR code demo for docs"),
			feedbackLabel,
		)

		logoColumn := container.NewVBox()
		logoColumn.Add(qrCode)
		logoColumn.Add(qrLabel)
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
		sizeShim := canvas.NewRectangle(color.Transparent{})
		sizeShim.SetMinSize(fyne.NewSize(560, 280))

		content := container.NewMax(sizeShim, body)
		myWindow.SetContent(content)
	}

	myWindow.Resize(fyne.NewSize(600, 400))
	myWindow.CenterOnScreen()
	myWindow.ShowAndRun()
}
