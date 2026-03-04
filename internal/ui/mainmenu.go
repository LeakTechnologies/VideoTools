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
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// ModuleInfo contains information about a module for display
type ModuleInfo struct {
	ID                  string
	Label               string
	Color               color.Color
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

// BuildMainMenu creates the main menu view with module tiles grouped by category
func BuildMainMenu(titleText string, modules []ModuleInfo, onModuleClick func(string), onModuleDrop func(string, []fyne.URI), onQueueClick func(), onLogsClick func(), onBenchmarkClick func(), onBenchmarkHistoryClick func(), onToggleSidebar func(), sidebarVisible bool, sidebar fyne.CanvasObject, titleColor, queueColor, textColor color.Color, queueCompleted, queueTotal int, hasBenchmark bool) fyne.CanvasObject {
	title := canvas.NewText(titleText, titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 20

	queueTile := buildQueueTile(queueCompleted, queueTotal, queueColor, textColor, onQueueClick)

	sidebarToggleBtn := widget.NewButton("☰", onToggleSidebar)
	sidebarToggleBtn.Importance = widget.LowImportance

	benchmarkBtn := widget.NewButton("Benchmark", onBenchmarkClick)
	// Highlight the benchmark button if no benchmark has been run
	if !hasBenchmark {
		benchmarkBtn.Importance = widget.HighImportance
	} else {
		benchmarkBtn.Importance = widget.LowImportance
	}

	viewResultsBtn := widget.NewButton("Results", onBenchmarkHistoryClick)
	viewResultsBtn.Importance = widget.LowImportance

	// Build header controls dynamically - only show logs button if callback is provided
	headerControls := []fyne.CanvasObject{sidebarToggleBtn}
	if onLogsClick != nil {
		logsBtn := widget.NewButton("Logs", onLogsClick)
		logsBtn.Importance = widget.LowImportance
		headerControls = append(headerControls, logsBtn)
	}
	headerControls = append(headerControls, benchmarkBtn, viewResultsBtn, queueTile)

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

	// Build rows with category labels above tiles
	var rows []fyne.CanvasObject

	// Convert section
	rows = append(rows, makeCatLabel("Convert"))
	rows = append(rows, buildGrid("convert", "merge", "trim", "filters", "audio", "subtitles"))

	// Inspect section
	rows = append(rows, makeCatLabel("Inspect"))
	rows = append(rows, buildGrid("compare", "inspect", "upscale"))

	// Disc section
	rows = append(rows, makeCatLabel("Disc"))
	rows = append(rows, buildGrid("author", "rip", "bluray"))

	// Playback section
	rows = append(rows, makeCatLabel("Playback"))
	rows = append(rows, buildGrid("player", "thumbnail", "settings"))

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
	return NewModuleTile(mod.Label, mod.Color, mod.Enabled, mod.MissingDependencies, tapped, dropped)
}

// buildQueueTile creates the queue status tile
func buildQueueTile(completed, total int, queueColor, textColor color.Color, onClick func()) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 6
	// rect.SetMinSize(fyne.NewSize(120, 40)) // Removed for flexible sizing

	text := canvas.NewText(fmt.Sprintf("QUEUE: %d/%d", completed, total), textColor)
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
	entries []HistoryEntry,
	activeJobs []HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
	onClearAll func(int),
	selectedTab int,
	onTabChanged func(int),
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
	inProgressList := buildHistoryList(activeJobs, onEntryClick, nil, bgColor, textColor) // No delete for active jobs
	completedList := buildHistoryList(completedEntries, onEntryClick, onEntryDelete, bgColor, textColor)
	failedList := buildHistoryList(failedEntries, onEntryClick, onEntryDelete, bgColor, textColor)

	// Tabs - In Progress first for quick visibility
	tabs := container.NewAppTabs(
		container.NewTabItem("In Progress", container.NewVScroll(inProgressList)),
		container.NewTabItem("Completed", container.NewVScroll(completedList)),
		container.NewTabItem("Failed", container.NewVScroll(failedList)),
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

	// Header
	title := canvas.NewText("HISTORY", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 18
	clearBtn := widget.NewButton("Clear All", func() {
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
	clearBtn.Importance = widget.LowImportance

	header := container.NewVBox(
		container.NewBorder(nil, nil, title, clearBtn, nil),
		widget.NewSeparator(),
	)

	return container.NewBorder(header, nil, nil, nil, tabs)
}

func buildHistoryList(
	entries []HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
	bgColor, textColor color.Color,
) *fyne.Container {
	if len(entries) == 0 {
		return container.NewCenter(widget.NewLabel("No entries"))
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
		deleteBtn := widget.NewButton("×", func() {
			onEntryDelete(capturedEntry)
		})
		deleteBtn.Importance = widget.LowImportance
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
