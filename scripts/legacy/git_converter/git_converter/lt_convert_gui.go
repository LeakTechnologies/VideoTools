package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("LT-Convert GUI")
	w.Resize(fyne.NewSize(900, 700))

	// Title section
	title := widget.NewLabelWithStyle("LT-CONVERT", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Simple button layout that works
	content := container.NewVBox(
		title,
		widget.NewSeparator(),
		widget.NewLabel("VideoTools Style Interface v1.0"),
		widget.NewSeparator(),
		widget.NewLabel("Ready for bash conversion"),
		widget.NewSeparator(),
		widget.NewButton("🚀 LAUNCH BASH", func() {
			// Close GUI and run bash version
			w.Close()
		}),
		widget.NewButton("❌ QUIT", func() {
			a.Quit()
		}),
	)

	w.SetContent(content)
	w.ShowAndRun()
}
