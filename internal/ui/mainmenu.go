package ui

import (
	"fmt"
	"image/color"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// ModuleInfo contains information about a module for display
type ModuleInfo struct {
	ID                  string
	Label               string
	Color               color.Color
	TextColor           color.Color
	Enabled             bool
	Category            string
	MissingDependencies bool // true if disabled due to missing dependencies
}

// HistoryEntry represents a completed job in the history
type HistoryEntry struct {
	ID          string
	Type        queue.JobType
	Status      queue.JobStatus
	Title       string
	InputFile   string
	OutputFile  string
	LogPath     string
	Config      map[string]interface{}
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string
	FFmpegCmd   string
	Progress    float64 // 0.0 to 1.0 for in-progress jobs
}

// MenuLabels holds all translatable strings used by BuildMainMenu.
// Populated by the caller from i18n.T() so this package stays language-agnostic.
type MenuLabels struct {
	// Header buttons
	Logs            string
	Files           string // "Files" dropdown button
	FilesOpen       string // "Open Files..."
	FilesRecent     string // "Recent Files"
	FilesOpenFolder string // "Open Output Folder"
	FilesAddMore    string // "Add More Files"
	FilesGoTo       string // "Go to %s"

	Window fyne.Window // needed for context menu positioning

	// Queue tile prefix ("QUEUE")
	Queue string

	// Section headings
	CategoryConvert     string
	CategoryInspect     string
	CategoryDisc        string
	CategoryPlayback    string
	CategoryAdvanced    string
	CategoryScreenshots string
	CategorySettings    string

	// History sidebar
	HistoryTitle      string
	HistoryInProgress string
	HistoryCompleted  string
	HistoryFailed     string
	HistoryClearAll   string
	HistoryNoEntries  string
}

// FilesDropdownData holds data needed to build the files dropdown menu.
// This enables context-aware actions based on the current module.
type FilesDropdownData struct {
	CurrentModule string       // Current active module ID (e.g., "convert", "author")
	RecentFiles   []RecentFile // Recently opened files
	OnFileClick   func(path, module string) // Callback when a recent file is clicked
	OnOpenFolder  func()       // Callback to open current output folder
	OnOpenMore    func()       // Callback to add more files to current module
}

// RecentFile represents a recently opened file for the dropdown
type RecentFile struct {
	Path        string
	DisplayName string
	Module      string // Which module it was opened in
}

// BuildMainMenu creates the main menu view with module tiles grouped by category.
// pipelineStep: "" = off, "step1" = waiting for first job, "step2" = waiting for second job.
// onPipelineToggle: called when the && button is clicked; nil hides the button.
func BuildMainMenu(titleText string, labels MenuLabels, modules []ModuleInfo, onModuleClick func(string), onModuleDrop func(string, []fyne.URI), onQueueClick func(), onLogsClick func(), onToggleSidebar func(), filesDropdownData *FilesDropdownData, sidebarVisible bool, sidebar fyne.CanvasObject, titleColor, queueColor, textColor color.Color, queueCompleted, queueTotal int, pipelineStep string, onPipelineToggle func()) fyne.CanvasObject {
	title := canvas.NewText(titleText, titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 20

	queueTile := buildQueueTile(labels.Queue, queueCompleted, queueTotal, queueColor, textColor, onQueueClick)

	historyBtn := MakePillButton(labels.HistoryTitle, titleColor, onToggleSidebar)
	historyBtn.Active = sidebarVisible

	filesDropdown := buildFilesDropdown(labels, filesDropdownData, textColor, labels.Window)

	// Build header controls — only show logs button if callback is provided
	headerControls := []fyne.CanvasObject{historyBtn, filesDropdown}
	if onLogsClick != nil {
		logsBtn := MakePillButton(labels.Logs, titleColor, onLogsClick)
		headerControls = append(headerControls, logsBtn)
	}
	if onPipelineToggle != nil {
		pipelineLabel := "&&"
		pipelineActive := false
		switch pipelineStep {
		case "step1":
			pipelineLabel = "[ && ]"
			pipelineActive = true
		case "step2":
			pipelineLabel = "A → ?"
			pipelineActive = true
		}
		pipelineBtn := MakePillButton(pipelineLabel, titleColor, onPipelineToggle)
		pipelineBtn.Active = pipelineActive
		headerControls = append(headerControls, pipelineBtn)
	}
	headerControls = append(headerControls, queueTile)

	// Compact header - title on left, controls on right
	header := container.NewBorder(
		nil, nil,
		title,
		container.NewHBox(headerControls...),
		nil,
	)

	// Create module map for quick lookup
	moduleMap := make(map[string]ModuleInfo)
	for _, mod := range modules {
		moduleMap[mod.ID] = mod
	}

	// Helper to build a tile
	buildTile := func(modID string) fyne.CanvasObject {
		mod, exists := moduleMap[modID]
		if !exists {
			return layout.NewSpacer()
		}

		var tapFunc func()
		var dropFunc func([]fyne.URI)
		if mod.Enabled {
			id := modID
			tapFunc = func() { onModuleClick(id) }
			dropFunc = func(items []fyne.URI) {
				logging.Debug(logging.CatUI, "MainMenu dropFunc called for module=%s itemCount=%d", id, len(items))
				onModuleDrop(id, items)
			}
		}
		return buildModuleTile(mod, tapFunc, dropFunc)
	}

	// Helper to create category label
	makeCatLabel := func(text string) *canvas.Text {
		label := canvas.NewText(text, textColor)
		label.TextSize = 12
		label.Alignment = fyne.TextAlignLeading
		return label
	}

	const tileColumns = 3
	buildGrid := func(ids ...string) fyne.CanvasObject {
		tiles := make([]fyne.CanvasObject, 0, len(ids))
		for _, id := range ids {
			tiles = append(tiles, buildTile(id))
		}
		return container.NewGridWithColumns(tileColumns, tiles...)
	}

	// hasAny returns true if at least one of the given IDs is present in moduleMap.
	hasAny := func(ids ...string) bool {
		for _, id := range ids {
			if _, ok := moduleMap[id]; ok {
				return true
			}
		}
		return false
	}

	// Build rows with category labels above tiles
	var rows []fyne.CanvasObject

	// addSection appends a category label + grid only when at least one tile is visible.
	addSection := func(label string, ids ...string) {
		if !hasAny(ids...) {
			return
		}
		rows = append(rows, makeCatLabel(label))
		rows = append(rows, buildGrid(ids...))
	}

	addSection(labels.CategoryConvert, "convert", "merge", "trim", "filters", "audio", "subtitles")
	addSection(labels.CategoryInspect, "compare", "inspect", "upscale")
	addSection(labels.CategoryDisc, "author", "rip", "burn")
	addSection(labels.CategoryPlayback, "player", "thumbnail", "settings")

	gridBox := container.NewVBox(rows...)

	body := container.NewBorder(
		header,
		nil, nil, nil,
		gridBox,
	)

	// Wrap with HSplit if sidebar is visible
	if sidebarVisible && sidebar != nil {
		split := container.NewHSplit(sidebar, body)
		split.Offset = 0.2
		return split
	}

	return body
}

// buildModuleTile creates a single module tile
func buildModuleTile(mod ModuleInfo, tapped func(), dropped func([]fyne.URI)) fyne.CanvasObject {
	logging.Debug(logging.CatUI, "building tile %s color=%v enabled=%v missingDeps=%v", mod.ID, mod.Color, mod.Enabled, mod.MissingDependencies)
	return NewModuleTile(mod.Label, mod.Color, mod.TextColor, mod.Enabled, mod.MissingDependencies, tapped, dropped, mod.Label)
}

// buildQueueTile creates the queue status tile
func buildQueueTile(label string, completed, total int, queueColor, textColor color.Color, onClick func()) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 6

	text := canvas.NewText(fmt.Sprintf("%s: %d/%d", label, completed, total), textColor)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 14

	tile := container.NewMax(rect, container.NewPadded(container.NewCenter(text)))

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

// BuildHistorySidebar creates the history sidebar with tabs
func BuildHistorySidebar(
	labels MenuLabels,
	entries []HistoryEntry,
	activeJobs []HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
	onClearAll func(int),
	selectedTab int,
	onTabChanged func(int),
	onToggleSidebar func(),
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Filter by status
	var completedEntries, failedEntries []HistoryEntry
	for _, entry := range entries {
		if entry.Status == queue.JobStatusCompleted {
			completedEntries = append(completedEntries, entry)
		} else {
			failedEntries = append(failedEntries, entry)
		}
	}

	// Build lists
	inProgressList := buildHistoryList(labels.HistoryNoEntries, activeJobs, onEntryClick, nil, bgColor, textColor)
	completedList := buildHistoryList(labels.HistoryNoEntries, completedEntries, onEntryClick, onEntryDelete, bgColor, textColor)
	failedList := buildHistoryList(labels.HistoryNoEntries, failedEntries, onEntryClick, onEntryDelete, bgColor, textColor)

	// Tabs - In Progress first for quick visibility
	tabs := container.NewAppTabs(
		container.NewTabItem(labels.HistoryInProgress, container.NewVScroll(inProgressList)),
		container.NewTabItem(labels.HistoryCompleted, container.NewVScroll(completedList)),
		container.NewTabItem(labels.HistoryFailed, container.NewVScroll(failedList)),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	if selectedTab >= 0 && selectedTab < len(tabs.Items) {
		tabs.SelectIndex(selectedTab)
	}
	tabs.OnSelected = func(item *container.TabItem) {
		if onTabChanged == nil {
			return
		}
		for idx, tab := range tabs.Items {
			if tab == item {
				onTabChanged(idx)
				return
			}
		}
	}

	// Header — clicking title dismisses sidebar
	title := canvas.NewText(labels.HistoryTitle, titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 18
	titleTappable := NewTappable(title, func() {
		if onToggleSidebar != nil {
			onToggleSidebar()
		}
	})
	clearBtn := MakePillButton(labels.HistoryClearAll, titleColor, func() {
		if onClearAll == nil {
			return
		}
		idx := 0
		for i, tab := range tabs.Items {
			if tab == tabs.Selected() {
				idx = i
				break
			}
		}
		onClearAll(idx)
	})

	header := container.NewVBox(
		container.NewBorder(nil, nil, titleTappable, clearBtn, nil),
		widget.NewSeparator(),
	)

	return container.NewBorder(header, nil, nil, nil, tabs)
}

func buildHistoryList(
	emptyLabel string,
	entries []HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
	bgColor, textColor color.Color,
) *fyne.Container {
	if len(entries) == 0 {
		return container.NewCenter(widget.NewLabel(emptyLabel))
	}

	var items []fyne.CanvasObject
	for _, entry := range entries {
		items = append(items, buildHistoryItem(entry, onEntryClick, onEntryDelete, bgColor, textColor))
	}
	return container.NewVBox(items...)
}

func buildHistoryItem(
	entry HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
	bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Badge
	badge := BuildModuleBadge(entry.Type)

	// Capture entry for closures
	capturedEntry := entry

	// Build header row with badge and optional delete button
	headerItems := []fyne.CanvasObject{badge, layout.NewSpacer()}
	if onEntryDelete != nil {
		// Delete button - small "×" button (only for completed/failed)
		deleteBtn := MakePillButton("×", BorderDim, func() {
			onEntryDelete(capturedEntry)
		})
		headerItems = append(headerItems, deleteBtn)
	}

	// Title
	titleLabel := widget.NewLabel(utils.ShortenMiddle(entry.Title, 25))
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Timestamp or status info
	var timeStr string
	if entry.Status == queue.JobStatusRunning || entry.Status == queue.JobStatusPending {
		// For in-progress jobs, show status
		if entry.Status == queue.JobStatusRunning {
			timeStr = "Running..."
		} else {
			timeStr = "Pending"
		}
	} else {
		// For completed/failed jobs, show timestamp
		if entry.CompletedAt != nil {
			timeStr = entry.CompletedAt.Format("Jan 2, 15:04")
		} else {
			timeStr = "Unknown"
		}
	}
	timeLabel := widget.NewLabel(timeStr)
	timeLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Progress bar for in-progress jobs
	contentItems := []fyne.CanvasObject{
		container.NewHBox(headerItems...),
		titleLabel,
		timeLabel,
	}

	if entry.Status == queue.JobStatusRunning || entry.Status == queue.JobStatusPending {
		// Add progress bar for active jobs
		moduleCol := ModuleColor(entry.Type)
		progressBar := NewStripedProgress(moduleCol)
		progressBar.SetProgress(entry.Progress)
		contentItems = append(contentItems, progressBar)
	}

	// Status color bar
	statusColor := GetStatusColor(entry.Status)
	statusRect := canvas.NewRectangle(statusColor)
	statusRect.SetMinSize(fyne.NewSize(4, 0))

	content := container.NewBorder(
		nil, nil, statusRect, nil,
		container.NewVBox(contentItems...),
	)

	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(container.NewMax(card, content))

	return NewTappable(item, func() { onEntryClick(capturedEntry) })
}

// buildFilesDropdown creates a dropdown menu with context-aware options
func buildFilesDropdown(labels MenuLabels, data *FilesDropdownData, textColor color.Color, win fyne.Window) fyne.CanvasObject {
	btn := MakePillButton(labels.Files, BorderDim, func() {
		menu := fyne.NewMenu("")

		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label: labels.FilesOpen,
			Action: func() {
				if data != nil && data.OnOpenMore != nil {
					data.OnOpenMore()
				}
			},
		})

		if data != nil && data.OnOpenFolder != nil {
			menu.Items = append(menu.Items, &fyne.MenuItem{
				Label:  labels.FilesOpenFolder,
				Action: data.OnOpenFolder,
			})
		}

		if data != nil && len(data.RecentFiles) > 0 {
			menu.Items = append(menu.Items, fyne.NewMenuItemSeparator())
			menu.Items = append(menu.Items, &fyne.MenuItem{
				Label:    labels.FilesRecent,
				Disabled: true,
			})
			for _, file := range data.RecentFiles {
				captured := file
				moduleLabel := fmt.Sprintf(labels.FilesGoTo, moduleLabelForID(captured.Module))
				menu.Items = append(menu.Items, &fyne.MenuItem{
					Label: fmt.Sprintf("  %s (%s)", captured.DisplayName, moduleLabel),
					Action: func() {
						if data.OnFileClick != nil {
							data.OnFileClick(captured.Path, captured.Module)
						}
					},
				})
			}
		}

		pop := widget.NewPopUpMenu(menu, win.Canvas())
		btn := win.Canvas().Focused()
		if btn != nil {
			pos := btn.(*widget.Button).Size()
			pop.ShowAtPosition(fyne.NewPos(pos.Width, pos.Height))
		}
	})
	return btn
}

// moduleLabelForID returns the display label for a module ID
func moduleLabelForID(moduleID string) string {
	switch moduleID {
	case "convert":
		return "Convert"
	case "author":
		return "Author"
	case "audio":
		return "Audio"
	case "subtitles":
		return "Subtitles"
	case "inspect":
		return "Inspect"
	case "player":
		return "Player"
	default:
		return moduleID
	}
}
