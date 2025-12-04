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
	ID      string
	Label   string
	Color   color.Color
	Enabled bool
}

// BuildMainMenu creates the main menu view with module tiles
func BuildMainMenu(modules []ModuleInfo, onModuleClick func(string), onModuleDrop func(string, []fyne.URI), onQueueClick func(), titleColor, queueColor, textColor color.Color, queueCompleted, queueTotal int) fyne.CanvasObject {
	title := canvas.NewText("VIDEOTOOLS", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 28

	queueTile := buildQueueTile(queueCompleted, queueTotal, queueColor, textColor, onQueueClick)

	header := container.New(layout.NewHBoxLayout(),
		title,
		layout.NewSpacer(),
		queueTile,
	)

	var tileObjects []fyne.CanvasObject
	for i := range modules {
		mod := modules[i]   // Create new variable for this iteration
		modID := mod.ID     // Capture for closure
		var tapFunc func()
		var dropFunc func([]fyne.URI)
		if mod.Enabled {
			// Create new closure with properly captured modID
			id := modID // Explicit capture
			tapFunc = func() {
				onModuleClick(id)
			}
			dropFunc = func(items []fyne.URI) {
				fmt.Printf("[MAINMENU] dropFunc called for module=%s itemCount=%d\n", id, len(items))
				logging.Debug(logging.CatUI, "MainMenu dropFunc called for module=%s itemCount=%d", id, len(items))
				onModuleDrop(id, items)
			}
		}
		fmt.Printf("[MAINMENU] Creating tile for module=%s enabled=%v hasDropFunc=%v\n", modID, mod.Enabled, dropFunc != nil)
		logging.Debug(logging.CatUI, "Creating tile for module=%s enabled=%v hasDropFunc=%v", modID, mod.Enabled, dropFunc != nil)
		tileObjects = append(tileObjects, buildModuleTile(mod, tapFunc, dropFunc))
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
func buildModuleTile(mod ModuleInfo, tapped func(), dropped func([]fyne.URI)) fyne.CanvasObject {
	logging.Debug(logging.CatUI, "building tile %s color=%v enabled=%v", mod.ID, mod.Color, mod.Enabled)
	return container.NewPadded(NewModuleTile(mod.Label, mod.Color, mod.Enabled, tapped, dropped))
}

// buildQueueTile creates the queue status tile
func buildQueueTile(completed, total int, queueColor, textColor color.Color, onClick func()) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 8
	rect.SetMinSize(fyne.NewSize(160, 60))

	text := canvas.NewText(fmt.Sprintf("QUEUE: %d/%d", completed, total), textColor)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 18

	tile := container.NewMax(rect, container.NewCenter(text))

	// Make it tappable
	tappable := NewTappable(tile, onClick)
	return tappable
}
