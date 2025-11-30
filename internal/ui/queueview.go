package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
)

// BuildQueueView creates the queue viewer UI
func BuildQueueView(
	jobs []*queue.Job,
	onBack func(),
	onPause func(string),
	onResume func(string),
	onCancel func(string),
	onRemove func(string),
	onMoveUp func(string),
	onMoveDown func(string),
	onPauseAll func(),
	onResumeAll func(),
	onStart func(),
	onClear func(),
	onClearAll func(),
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Header
	title := canvas.NewText("JOB QUEUE", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	backBtn := widget.NewButton("← Back", onBack)
	backBtn.Importance = widget.LowImportance

	startAllBtn := widget.NewButton("Start Queue", onStart)
	startAllBtn.Importance = widget.MediumImportance

	pauseAllBtn := widget.NewButton("Pause All", onPauseAll)
	pauseAllBtn.Importance = widget.LowImportance

	resumeAllBtn := widget.NewButton("Resume All", onResumeAll)
	resumeAllBtn.Importance = widget.LowImportance

	clearBtn := widget.NewButton("Clear Completed", onClear)
	clearBtn.Importance = widget.LowImportance

	clearAllBtn := widget.NewButton("Clear All", onClearAll)
	clearAllBtn.Importance = widget.DangerImportance

	buttonRow := container.NewHBox(startAllBtn, pauseAllBtn, resumeAllBtn, clearAllBtn, clearBtn)

	header := container.NewBorder(
		nil, nil,
		backBtn,
		buttonRow,
		container.NewCenter(title),
	)

	// Job list
	var jobItems []fyne.CanvasObject

	if len(jobs) == 0 {
		emptyMsg := widget.NewLabel("No jobs in queue")
		emptyMsg.Alignment = fyne.TextAlignCenter
		jobItems = append(jobItems, container.NewCenter(emptyMsg))
	} else {
		for _, job := range jobs {
			jobItems = append(jobItems, buildJobItem(job, onPause, onResume, onCancel, onRemove, onMoveUp, onMoveDown, bgColor, textColor))
		}
	}

	jobList := container.NewVBox(jobItems...)
	scrollable := container.NewVScroll(jobList)

	body := container.NewBorder(
		header,
		nil, nil, nil,
		scrollable,
	)

	return container.NewPadded(body)
}

// buildJobItem creates a single job item in the queue list
func buildJobItem(
	job *queue.Job,
	onPause func(string),
	onResume func(string),
	onCancel func(string),
	onRemove func(string),
	onMoveUp func(string),
	onMoveDown func(string),
	bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Status color
	statusColor := getStatusColor(job.Status)

	// Status indicator
	statusRect := canvas.NewRectangle(statusColor)
	statusRect.SetMinSize(fyne.NewSize(6, 0))

	// Title and description
	titleLabel := widget.NewLabel(job.Title)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	descLabel := widget.NewLabel(job.Description)
	descLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Progress bar (for running jobs)
	progress := widget.NewProgressBar()
	progress.SetValue(job.Progress / 100.0)
	if job.Status == queue.JobStatusCompleted {
		progress.SetValue(1.0)
	}
	progressWidget := progress

	// Module badge
	badge := buildModuleBadge(job.Type)

	// Status text
	statusText := getStatusText(job)
	statusLabel := widget.NewLabel(statusText)
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Control buttons
	var buttons []fyne.CanvasObject
	// Reorder arrows for pending/paused jobs
	if job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused {
		buttons = append(buttons,
			widget.NewButton("↑", func() { onMoveUp(job.ID) }),
			widget.NewButton("↓", func() { onMoveDown(job.ID) }),
		)
	}

	switch job.Status {
	case queue.JobStatusRunning:
		buttons = append(buttons,
			widget.NewButton("Pause", func() { onPause(job.ID) }),
			widget.NewButton("Cancel", func() { onCancel(job.ID) }),
		)
	case queue.JobStatusPaused:
		buttons = append(buttons,
			widget.NewButton("Resume", func() { onResume(job.ID) }),
			widget.NewButton("Cancel", func() { onCancel(job.ID) }),
		)
	case queue.JobStatusPending:
		buttons = append(buttons,
			widget.NewButton("Cancel", func() { onCancel(job.ID) }),
		)
	case queue.JobStatusCompleted, queue.JobStatusFailed, queue.JobStatusCancelled:
		buttons = append(buttons,
			widget.NewButton("Remove", func() { onRemove(job.ID) }),
		)
	}

	buttonBox := container.NewHBox(buttons...)

	// Info section
	infoBox := container.NewVBox(
		container.NewHBox(titleLabel, layout.NewSpacer(), badge),
		descLabel,
		progressWidget,
		statusLabel,
	)

	// Main content
	content := container.NewBorder(
		nil, nil,
		statusRect,
		buttonBox,
		infoBox,
	)

	// Card background
	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, content),
	)

	// Wrap with draggable to allow drag-to-reorder (up/down by drag direction)
	return newDraggableJobItem(job.ID, item, func(id string, dir int) {
		if dir < 0 {
			onMoveUp(id)
		} else if dir > 0 {
			onMoveDown(id)
		}
	})
}

// getStatusColor returns the color for a job status
func getStatusColor(status queue.JobStatus) color.Color {
	switch status {
	case queue.JobStatusPending:
		return color.RGBA{R: 150, G: 150, B: 150, A: 255} // Gray
	case queue.JobStatusRunning:
		return color.RGBA{R: 68, G: 136, B: 255, A: 255} // Blue
	case queue.JobStatusPaused:
		return color.RGBA{R: 255, G: 193, B: 7, A: 255} // Yellow
	case queue.JobStatusCompleted:
		return color.RGBA{R: 76, G: 232, B: 112, A: 255} // Green
	case queue.JobStatusFailed:
		return color.RGBA{R: 255, G: 68, B: 68, A: 255} // Red
	case queue.JobStatusCancelled:
		return color.RGBA{R: 255, G: 136, B: 68, A: 255} // Orange
	default:
		return color.Gray{Y: 128}
	}
}

// getStatusText returns a human-readable status string
func getStatusText(job *queue.Job) string {
	switch job.Status {
	case queue.JobStatusPending:
		return fmt.Sprintf("Status: Pending | Priority: %d", job.Priority)
	case queue.JobStatusRunning:
		elapsed := ""
		if job.StartedAt != nil {
			elapsed = fmt.Sprintf(" | Elapsed: %s", time.Since(*job.StartedAt).Round(time.Second))
		}
		return fmt.Sprintf("Status: Running | Progress: %.1f%%%s", job.Progress, elapsed)
	case queue.JobStatusPaused:
		return "Status: Paused"
	case queue.JobStatusCompleted:
		duration := ""
		if job.StartedAt != nil && job.CompletedAt != nil {
			duration = fmt.Sprintf(" | Duration: %s", job.CompletedAt.Sub(*job.StartedAt).Round(time.Second))
		}
		return fmt.Sprintf("Status: Completed%s", duration)
	case queue.JobStatusFailed:
		return fmt.Sprintf("Status: Failed | Error: %s", job.Error)
	case queue.JobStatusCancelled:
		return "Status: Cancelled"
	default:
		return fmt.Sprintf("Status: %s", job.Status)
	}
}

// buildModuleBadge renders a small colored pill to show which module created the job.
func buildModuleBadge(t queue.JobType) fyne.CanvasObject {
	label := widget.NewLabel(string(t))
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter

	bg := canvas.NewRectangle(moduleColor(t))
	bg.CornerRadius = 6
	bg.SetMinSize(fyne.NewSize(label.MinSize().Width+12, label.MinSize().Height+6))

	return container.NewMax(bg, container.NewCenter(label))
}

// moduleColor maps job types to distinct colors for quick visual scanning.
func moduleColor(t queue.JobType) color.Color {
	switch t {
	case queue.JobTypeConvert:
		return color.RGBA{R: 76, G: 232, B: 112, A: 255} // green
	case queue.JobTypeMerge:
		return color.RGBA{R: 68, G: 136, B: 255, A: 255} // blue
	case queue.JobTypeTrim:
		return color.RGBA{R: 255, G: 193, B: 7, A: 255} // amber
	case queue.JobTypeFilter:
		return color.RGBA{R: 160, G: 86, B: 255, A: 255} // purple
	case queue.JobTypeUpscale:
		return color.RGBA{R: 255, G: 138, B: 101, A: 255} // coral
	case queue.JobTypeAudio:
		return color.RGBA{R: 255, G: 215, B: 64, A: 255} // gold
	case queue.JobTypeThumb:
		return color.RGBA{R: 102, G: 217, B: 239, A: 255} // teal
	default:
		return color.Gray{Y: 180}
	}
}

// draggableJobItem allows simple drag up/down to reorder one slot at a time.
type draggableJobItem struct {
	widget.BaseWidget
	jobID     string
	content   fyne.CanvasObject
	onReorder func(string, int) // id, direction (-1 up, +1 down)
	accumY    float32
}

func newDraggableJobItem(id string, content fyne.CanvasObject, onReorder func(string, int)) *draggableJobItem {
	d := &draggableJobItem{
		jobID:     id,
		content:   content,
		onReorder: onReorder,
	}
	d.ExtendBaseWidget(d)
	return d
}

func (d *draggableJobItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(d.content)
}

func (d *draggableJobItem) Dragged(ev *fyne.DragEvent) {
	// fyne.Delta is a struct with dx, dy fields
	d.accumY += ev.Dragged.DY
}

func (d *draggableJobItem) DragEnd() {
	const threshold float32 = 25
	if d.accumY <= -threshold {
		d.onReorder(d.jobID, -1)
	} else if d.accumY >= threshold {
		d.onReorder(d.jobID, 1)
	}
	d.accumY = 0
}
