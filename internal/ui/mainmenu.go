package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// ModuleInfo contains information about a module for display
type ModuleInfo struct {
	ID    string
	Label string
	Color color.Color
}

// BuildMainMenu creates the main menu view with module tiles
func BuildMainMenu(modules []ModuleInfo, onModuleClick func(string), titleColor, queueColor, textColor color.Color) fyne.CanvasObject {
	title := canvas.NewText("VIDEOTOOLS", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 28

	queueTile := buildQueueTile(0, 0, queueColor, textColor)

	header := container.New(layout.NewHBoxLayout(),
		title,
		layout.NewSpacer(),
		queueTile,
	)

	var tileObjects []fyne.CanvasObject
	for _, mod := range modules {
		modID := mod.ID // Capture for closure
		tileObjects = append(tileObjects, buildModuleTile(mod, func() {
			onModuleClick(modID)
		}))
	}

	grid := container.NewGridWithColumns(3, tileObjects...)

	padding := canvas.NewRectangle(color.Transparent)
	padding.SetMinSize(fyne.NewSize(0, 14))

	body := container.New(layout.NewVBoxLayout(),
		header,
		padding,
		grid,
	)

	return body
}

// buildModuleTile creates a single module tile
func buildModuleTile(mod ModuleInfo, tapped func()) fyne.CanvasObject {
	logging.Debug(logging.CatUI, "building tile %s color=%v", mod.ID, mod.Color)
	return container.NewPadded(NewModuleTile(mod.Label, mod.Color, tapped))
}

// buildQueueTile creates the queue status tile
func buildQueueTile(done, total int, queueColor, textColor color.Color) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 8
	rect.SetMinSize(fyne.NewSize(160, 60))

	text := canvas.NewText(fmt.Sprintf("QUEUE: %d/%d", done, total), textColor)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 18

	return container.NewMax(rect, container.NewCenter(text))
}
