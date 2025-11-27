package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	onClear func(),
	onClearAll func(),
	onProcess func(),
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Header
	title := canvas.NewText("JOB QUEUE", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	backBtn := widget.NewButton("← Back", onBack)
	backBtn.Importance = widget.LowImportance

	clearBtn := widget.NewButton("Clear Completed", onClear)
	clearBtn.Importance = widget.LowImportance

	clearAllBtn := widget.NewButton("Clear All", onClearAll)
	clearAllBtn.Importance = widget.DangerImportance

	processBtn := widget.NewButton("▶ Process Queue", onProcess)
	processBtn.Importance = widget.HighImportance

	// Only show process button if there are pending jobs
	if len(jobs) == 0 {
		processBtn.Disable()
	}
	var hasPending bool
	for _, job := range jobs {
		if job.Status == queue.JobStatusPending {
			hasPending = true
			break
		}
	}
	if !hasPending {
		processBtn.Disable()
	}

	header := container.NewBorder(
		nil, nil,
		backBtn,
		container.NewHBox(clearBtn, clearAllBtn, processBtn),
		container.NewCenter(title),
	)

	// Count stats
	pending := 0
	running := 0
	failed := 0
	completed := 0
	for _, job := range jobs {
		switch job.Status {
		case queue.JobStatusPending:
			pending++
		case queue.JobStatusRunning:
			running++
		case queue.JobStatusCompleted:
			completed++
		case queue.JobStatusFailed, queue.JobStatusCancelled:
			failed++
		}
	}

	// Stats display with better formatting
	var statsText string
	if len(jobs) == 0 {
		statsText = "Queue is empty"
	} else {
		statsText = fmt.Sprintf("  Total: %d  |  Running: %d  |  Pending: %d  |  Completed: %d  |  Failed: %d  ",
			len(jobs), running, pending, completed, failed)
	}
	statsLabel := widget.NewLabel(statsText)
	statsLabel.Alignment = fyne.TextAlignCenter

	// Job list
	var jobItems []fyne.CanvasObject

	if len(jobs) == 0 {
		emptyMsg := widget.NewLabel("Drop videos on modules to add conversion jobs")
		emptyMsg.Alignment = fyne.TextAlignCenter
		jobItems = append(jobItems, container.NewCenter(emptyMsg))
	} else {
		for _, job := range jobs {
			jobItems = append(jobItems, buildJobItem(job, onPause, onResume, onCancel, onRemove, bgColor, textColor))
		}
	}

	jobList := container.NewVBox(jobItems...)
	scrollable := container.NewVScroll(jobList)

	// Create body with header, stats, and scrollable list
	body := container.NewBorder(
		header,
		statsLabel, nil, nil,
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
	bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Status color
	statusColor := getStatusColor(job.Status)

	// Status indicator bar
	statusRect := canvas.NewRectangle(statusColor)
	statusRect.SetMinSize(fyne.NewSize(4, 0))

	// Title with modified indicator
	titleText := job.Title
	if job.HasModifiedSettings && job.Status == queue.JobStatusPending {
		titleText = titleText + " ⚙ (custom settings)"
	}
	titleLabel := widget.NewLabel(titleText)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Description/output path
	descLabel := widget.NewLabel(job.Description)
	descLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Progress bar (for running/completed jobs)
	var progressWidget fyne.CanvasObject
	if job.Status == queue.JobStatusRunning || job.Status == queue.JobStatusCompleted {
		progress := widget.NewProgressBar()
		if job.Status == queue.JobStatusRunning {
			progress.SetValue(job.Progress / 100.0)
		} else {
			progress.SetValue(1.0)
		}
		progressWidget = progress
	} else {
		progressWidget = widget.NewLabel("")
	}

	// Status text
	statusText := getStatusText(job)
	statusLabel := widget.NewLabel(statusText)
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Control buttons with status-appropriate styling
	var buttons []fyne.CanvasObject
	switch job.Status {
	case queue.JobStatusRunning:
		pauseBtn := widget.NewButton("⏸ Pause", func() { onPause(job.ID) })
		pauseBtn.Importance = widget.MediumImportance
		cancelBtn := widget.NewButton("⊗ Cancel", func() { onCancel(job.ID) })
		cancelBtn.Importance = widget.DangerImportance
		buttons = append(buttons, pauseBtn, cancelBtn)
	case queue.JobStatusPaused:
		resumeBtn := widget.NewButton("▶ Resume", func() { onResume(job.ID) })
		resumeBtn.Importance = widget.MediumImportance
		cancelBtn := widget.NewButton("⊗ Cancel", func() { onCancel(job.ID) })
		cancelBtn.Importance = widget.DangerImportance
		buttons = append(buttons, resumeBtn, cancelBtn)
	case queue.JobStatusPending:
		cancelBtn := widget.NewButton("⊗ Cancel", func() { onCancel(job.ID) })
		cancelBtn.Importance = widget.DangerImportance
		buttons = append(buttons, cancelBtn)
	case queue.JobStatusCompleted:
		removeBtn := widget.NewButton("✓ Remove", func() { onRemove(job.ID) })
		removeBtn.Importance = widget.LowImportance
		buttons = append(buttons, removeBtn)
	case queue.JobStatusFailed:
		removeBtn := widget.NewButton("✗ Remove", func() { onRemove(job.ID) })
		removeBtn.Importance = widget.LowImportance
		buttons = append(buttons, removeBtn)
	case queue.JobStatusCancelled:
		removeBtn := widget.NewButton("⊗ Remove", func() { onRemove(job.ID) })
		removeBtn.Importance = widget.LowImportance
		buttons = append(buttons, removeBtn)
	}

	// Layout buttons in a responsive way
	buttonBox := container.NewHBox(buttons...)

	// Info section
	infoBox := container.NewVBox(
		titleLabel,
		descLabel,
		progressWidget,
		statusLabel,
	)

	// Main content with borders
	content := container.NewBorder(
		nil, nil,
		statusRect,
		buttonBox,
		infoBox,
	)

	// Card background with padding
	card := canvas.NewRectangle(bgColor)
	card.CornerRadius = 4

	item := container.NewPadded(
		container.NewMax(card, content),
	)

	return item
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
