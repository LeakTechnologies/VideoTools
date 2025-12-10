package ui

import (
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// ModuleInfo contains information about a module for display
type ModuleInfo struct {
	ID       string
	Label    string
	Color    color.Color
	Enabled  bool
	Category string
}

// BuildMainMenu creates the main menu view with module tiles grouped by category
func BuildMainMenu(modules []ModuleInfo, onModuleClick func(string), onModuleDrop func(string, []fyne.URI), onQueueClick func(), onLogsClick func(), titleColor, queueColor, textColor color.Color, queueCompleted, queueTotal int) fyne.CanvasObject {
	title := canvas.NewText("VIDEOTOOLS", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 28

	queueTile := buildQueueTile(queueCompleted, queueTotal, queueColor, textColor, onQueueClick)
	logsBtn := widget.NewButton("Logs", onLogsClick)
	logsBtn.Importance = widget.LowImportance

	header := container.New(layout.NewHBoxLayout(), title, layout.NewSpacer(), logsBtn, queueTile)

	categorized := map[string][]fyne.CanvasObject{}
	for i := range modules {
		mod := modules[i] // Create new variable for this iteration
		modID := mod.ID   // Capture for closure
		cat := mod.Category
		if cat == "" {
			cat = "General"
		}
		var tapFunc func()
		var dropFunc func([]fyne.URI)
		if mod.Enabled {
			// Create new closure with properly captured modID
			id := modID // Explicit capture
			tapFunc = func() {
				onModuleClick(id)
			}
			dropFunc = func(items []fyne.URI) {
				logging.Debug(logging.CatUI, "MainMenu dropFunc called for module=%s itemCount=%d", id, len(items))
				onModuleDrop(id, items)
			}
		}
		logging.Debug(logging.CatUI, "Creating tile for module=%s enabled=%v hasDropFunc=%v", modID, mod.Enabled, dropFunc != nil)
		categorized[cat] = append(categorized[cat], buildModuleTile(mod, tapFunc, dropFunc))
	}

	var sections []fyne.CanvasObject
	for _, cat := range sortedKeys(categorized) {
		sections = append(sections,
			canvas.NewText(cat, textColor),
			container.NewGridWithColumns(3, categorized[cat]...),
		)
	}

	padding := canvas.NewRectangle(color.Transparent)
	padding.SetMinSize(fyne.NewSize(0, 14))

	body := container.New(layout.NewVBoxLayout(),
		header,
		padding,
		container.NewVBox(sections...),
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

// sortedKeys returns sorted keys for stable category ordering
func sortedKeys(m map[string][]fyne.CanvasObject) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
