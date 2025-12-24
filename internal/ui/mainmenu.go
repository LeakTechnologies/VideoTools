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
	ID       string
	Label    string
	Color    color.Color
	Enabled  bool
	Category string
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
func BuildMainMenu(modules []ModuleInfo, onModuleClick func(string), onModuleDrop func(string, []fyne.URI), onQueueClick func(), onLogsClick func(), onBenchmarkClick func(), onBenchmarkHistoryClick func(), onToggleSidebar func(), sidebarVisible bool, sidebar fyne.CanvasObject, titleColor, queueColor, textColor color.Color, queueCompleted, queueTotal int, hasBenchmark bool) fyne.CanvasObject {
	title := canvas.NewText("VIDEOTOOLS", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 18

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
		catLabel := canvas.NewText(cat, textColor)
		catLabel.TextSize = 12
		catLabel.TextStyle = fyne.TextStyle{Bold: true}
		tileSize := fyne.NewSize(170, 75)
		sections = append(sections,
			catLabel,
			container.NewGridWrap(tileSize, categorized[cat]...),
		)
	}

	padding := canvas.NewRectangle(color.Transparent)
	padding.SetMinSize(fyne.NewSize(0, 4))

	// Compact body without scrolling
	body := container.NewVBox(
		header,
		padding,
		container.NewVBox(sections...),
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
	logging.Debug(logging.CatUI, "building tile %s color=%v enabled=%v", mod.ID, mod.Color, mod.Enabled)
	return NewModuleTile(mod.Label, mod.Color, mod.Enabled, tapped, dropped)
}

// buildQueueTile creates the queue status tile
func buildQueueTile(completed, total int, queueColor, textColor color.Color, onClick func()) fyne.CanvasObject {
	rect := canvas.NewRectangle(queueColor)
	rect.CornerRadius = 6
	rect.SetMinSize(fyne.NewSize(120, 40))

	text := canvas.NewText(fmt.Sprintf("QUEUE: %d/%d", completed, total), textColor)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 14

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

// BuildHistorySidebar creates the history sidebar with tabs
func BuildHistorySidebar(
	entries []HistoryEntry,
	activeJobs []HistoryEntry,
	onEntryClick func(HistoryEntry),
	onEntryDelete func(HistoryEntry),
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

	// Header
	title := canvas.NewText("HISTORY", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 18

	header := container.NewVBox(
		container.NewCenter(title),
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
