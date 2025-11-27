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
	titleColor, bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Header
	title := canvas.NewText("JOB QUEUE", titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	backBtn := widget.NewButton("← Back", onBack)
	clearBtn := widget.NewButton("Clear Completed", onClear)

	header := container.NewBorder(
		nil, nil,
		backBtn,
		clearBtn,
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
			jobItems = append(jobItems, buildJobItem(job, onPause, onResume, onCancel, onRemove, bgColor, textColor))
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
	var progressWidget fyne.CanvasObject
	if job.Status == queue.JobStatusRunning {
		progress := widget.NewProgressBar()
		progress.SetValue(job.Progress / 100.0)
		progressWidget = progress
	} else if job.Status == queue.JobStatusCompleted {
		progress := widget.NewProgressBar()
		progress.SetValue(1.0)
		progressWidget = progress
	} else {
		progressWidget = widget.NewLabel("")
	}

	// Status text
	statusText := getStatusText(job)
	statusLabel := widget.NewLabel(statusText)
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Control buttons
	var buttons []fyne.CanvasObject
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
		titleLabel,
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
