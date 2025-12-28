package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// StripedProgress renders a progress bar with a tinted stripe pattern.
type StripedProgress struct {
	widget.BaseWidget
	progress float64
	color    color.Color
	bg       color.Color
	offset   float64
	activity bool
	animMu   sync.Mutex
	animStop chan struct{}
}

// NewStripedProgress creates a new striped progress bar with the given color
func NewStripedProgress(col color.Color) *StripedProgress {
	sp := &StripedProgress{
		progress: 0,
		color:    col,
		bg:       color.RGBA{R: 34, G: 38, B: 48, A: 255}, // dark neutral
	}
	sp.ExtendBaseWidget(sp)
	return sp
}

// SetProgress updates the progress value (0.0 to 1.0)
func (s *StripedProgress) SetProgress(p float64) {
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	s.progress = p
	s.Refresh()
}

// SetActivity toggles the full-width animated background when progress is near zero.
func (s *StripedProgress) SetActivity(active bool) {
	s.activity = active
	s.Refresh()
}

// StartAnimation starts the stripe animation.
func (s *StripedProgress) StartAnimation() {
	s.animMu.Lock()
	if s.animStop != nil {
		s.animMu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.animStop = stop
	s.animMu.Unlock()

	ticker := time.NewTicker(80 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				app := fyne.CurrentApp()
				if app == nil {
					continue
				}
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.Refresh()
				}, false)
			case <-stop:
				return
			}
		}
	}()
}

// StopAnimation stops the stripe animation.
func (s *StripedProgress) StopAnimation() {
	s.animMu.Lock()
	if s.animStop == nil {
		s.animMu.Unlock()
		return
	}
	close(s.animStop)
	s.animStop = nil
	s.animMu.Unlock()
}

func (s *StripedProgress) CreateRenderer() fyne.WidgetRenderer {
	bgRect := canvas.NewRectangle(s.bg)
	fillRect := canvas.NewRectangle(applyAlpha(s.color, 200))
	stripes := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		lightAlpha := uint8(80)
		darkAlpha := uint8(220)
		if s.activity && s.progress <= 0 {
			lightAlpha = 40
			darkAlpha = 90
		}
		light := applyAlpha(s.color, lightAlpha)
		dark := applyAlpha(s.color, darkAlpha)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				// animate diagonal stripes using offset
				if (((x + y) + int(s.offset)) / 4 % 2) == 0 {
					img.Set(x, y, light)
				} else {
					img.Set(x, y, dark)
				}
			}
		}
		return img
	})

	objects := []fyne.CanvasObject{bgRect, fillRect, stripes}

	r := &stripedProgressRenderer{
		bar:     s,
		bg:      bgRect,
		fill:    fillRect,
		stripes: stripes,
		objects: objects,
	}
	return r
}

type stripedProgressRenderer struct {
	bar     *StripedProgress
	bg      *canvas.Rectangle
	fill    *canvas.Rectangle
	stripes *canvas.Raster
	objects []fyne.CanvasObject
}

func (r *stripedProgressRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	fillWidth := size.Width * float32(r.bar.progress)
	stripeWidth := fillWidth
	if r.bar.activity && r.bar.progress <= 0 {
		stripeWidth = size.Width
	}
	fillSize := fyne.NewSize(fillWidth, size.Height)
	stripeSize := fyne.NewSize(stripeWidth, size.Height)

	r.fill.Resize(fillSize)
	r.fill.Move(fyne.NewPos(0, 0))

	r.stripes.Resize(stripeSize)
	r.stripes.Move(fyne.NewPos(0, 0))
}

func (r *stripedProgressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 20)
}

func (r *stripedProgressRenderer) Refresh() {
	// Only animate stripes when animation is active
	r.bar.animMu.Lock()
	shouldAnimate := r.bar.animStop != nil
	r.bar.animMu.Unlock()

	if shouldAnimate {
		r.bar.offset += 2
	}
	r.Layout(r.bg.Size())
	canvas.Refresh(r.bg)
	canvas.Refresh(r.stripes)
}

func (r *stripedProgressRenderer) BackgroundColor() color.Color { return color.Transparent }
func (r *stripedProgressRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *stripedProgressRenderer) Destroy()                     { r.bar.StopAnimation() }

func applyAlpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

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
	onCopyError func(string),
	onViewLog func(string),
	onCopyCommand func(string),
	titleColor, bgColor, textColor color.Color,
) (fyne.CanvasObject, *container.Scroll) {
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
		// Calculate queue positions for pending/paused jobs
		queuePositions := make(map[string]int)
		position := 1
		for _, job := range jobs {
			if job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused {
				queuePositions[job.ID] = position
				position++
			}
		}

		for _, job := range jobs {
			jobItems = append(jobItems, buildJobItem(job, queuePositions, onPause, onResume, onCancel, onRemove, onMoveUp, onMoveDown, onCopyError, onViewLog, onCopyCommand, bgColor, textColor))
		}
	}

	jobList := container.NewVBox(jobItems...)
	// Use a scroll container anchored to the top to avoid jumpy scroll-to-content behavior.
	scrollable := container.NewScroll(jobList)
	scrollable.SetMinSize(fyne.NewSize(0, 0))
	scrollable.Offset = fyne.NewPos(0, 0)

	body := container.NewBorder(
		header,
		nil, nil, nil,
		scrollable,
	)

	return container.NewPadded(body), scrollable
}

// buildJobItem creates a single job item in the queue list
func buildJobItem(
	job *queue.Job,
	queuePositions map[string]int,
	onPause func(string),
	onResume func(string),
	onCancel func(string),
	onRemove func(string),
	onMoveUp func(string),
	onMoveDown func(string),
	onCopyError func(string),
	onViewLog func(string),
	onCopyCommand func(string),
	bgColor, textColor color.Color,
) fyne.CanvasObject {
	// Status color
	statusColor := GetStatusColor(job.Status)

	// Status indicator
	statusRect := canvas.NewRectangle(statusColor)
	statusRect.SetMinSize(fyne.NewSize(6, 0))

	// Title and description
	titleText := utils.ShortenMiddle(job.Title, 60)
	descText := utils.ShortenMiddle(job.Description, 90)

	titleLabel := widget.NewLabel(titleText)
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	descLabel := widget.NewLabel(descText)
	descLabel.TextStyle = fyne.TextStyle{Italic: true}
	descLabel.Wrapping = fyne.TextWrapWord

	// Progress bar (for running jobs)
	progress := NewStripedProgress(ModuleColor(job.Type))
	progress.SetProgress(job.Progress / 100.0)
	if job.Status == queue.JobStatusCompleted {
		progress.SetProgress(1.0)
	}
	if job.Status == queue.JobStatusRunning {
		progress.SetActivity(job.Progress <= 0.01)
		progress.StartAnimation()
	} else {
		progress.SetActivity(false)
		progress.StopAnimation()
	}
	progressWidget := progress

	// Module badge
	badge := BuildModuleBadge(job.Type)

	// Status text
	statusText := getStatusText(job, queuePositions)
	statusLabel := widget.NewLabel(statusText)
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}
	statusLabel.Wrapping = fyne.TextWrapWord

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
			widget.NewButton("Copy Command", func() { onCopyCommand(job.ID) }),
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
			widget.NewButton("Copy Command", func() { onCopyCommand(job.ID) }),
			widget.NewButton("Remove", func() { onRemove(job.ID) }),
		)
	case queue.JobStatusCompleted, queue.JobStatusFailed, queue.JobStatusCancelled:
		if job.Status == queue.JobStatusFailed && strings.TrimSpace(job.Error) != "" && onCopyError != nil {
			buttons = append(buttons,
				widget.NewButton("Copy Error", func() { onCopyError(job.ID) }),
			)
		}
		if job.LogPath != "" && onViewLog != nil {
			buttons = append(buttons,
				widget.NewButton("View Log", func() { onViewLog(job.ID) }),
			)
		}
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

// getStatusText returns a human-readable status string
func getStatusText(job *queue.Job, queuePositions map[string]int) string {
	switch job.Status {
	case queue.JobStatusPending:
		// Display position in queue (1 = first to run, 2 = second, etc.)
		if pos, ok := queuePositions[job.ID]; ok {
			return fmt.Sprintf("Status: Pending | Queue Position: %d", pos)
		}
		return "Status: Pending"
	case queue.JobStatusRunning:
		elapsed := ""
		if job.StartedAt != nil {
			elapsed = fmt.Sprintf(" | Elapsed: %s", time.Since(*job.StartedAt).Round(time.Second))
		}

		// Add FPS and speed info if available in Config
		var extras string
		if job.Config != nil {
			if fps, ok := job.Config["fps"].(float64); ok && fps > 0 {
				extras += fmt.Sprintf(" | %.0f fps", fps)
			}
			if speed, ok := job.Config["speed"].(float64); ok && speed > 0 {
				extras += fmt.Sprintf(" | %.2fx", speed)
			}
			if etaDuration, ok := job.Config["eta"].(time.Duration); ok && etaDuration > 0 {
				extras += fmt.Sprintf(" | ETA %s", etaDuration.Round(time.Second))
			}
		}

		return fmt.Sprintf("Status: Running | Progress: %.1f%%%s%s", job.Progress, elapsed, extras)
	case queue.JobStatusPaused:
		// Display position in queue for paused jobs too
		if pos, ok := queuePositions[job.ID]; ok {
			return fmt.Sprintf("Status: Paused | Queue Position: %d", pos)
		}
		return "Status: Paused"
	case queue.JobStatusCompleted:
		duration := ""
		if job.StartedAt != nil && job.CompletedAt != nil {
			duration = fmt.Sprintf(" | Duration: %s", job.CompletedAt.Sub(*job.StartedAt).Round(time.Second))
		}
		return fmt.Sprintf("Status: Completed%s", duration)
	case queue.JobStatusFailed:
		// Truncate error to prevent UI overflow
		errMsg := job.Error
		maxLen := 150
		if len(errMsg) > maxLen {
			errMsg = errMsg[:maxLen] + "… (see Copy Error button for full message)"
		}
		return fmt.Sprintf("Status: Failed | Error: %s", errMsg)
	case queue.JobStatusCancelled:
		return "Status: Cancelled"
	default:
		return fmt.Sprintf("Status: %s", job.Status)
	}
}

// moduleColor maps job types to distinct colors matching the main module colors
// ModuleColor returns the color for a given job type
func ModuleColor(t queue.JobType) color.Color {
	switch t {
	case queue.JobTypeConvert:
		return color.RGBA{R: 139, G: 68, B: 255, A: 255} // Violet (#8B44FF)
	case queue.JobTypeMerge:
		return color.RGBA{R: 68, G: 136, B: 255, A: 255} // Blue (#4488FF)
	case queue.JobTypeTrim:
		return color.RGBA{R: 68, G: 221, B: 255, A: 255} // Cyan (#44DDFF)
	case queue.JobTypeFilter:
		return color.RGBA{R: 68, G: 255, B: 136, A: 255} // Green (#44FF88)
	case queue.JobTypeUpscale:
		return color.RGBA{R: 170, G: 255, B: 68, A: 255} // Yellow-Green (#AAFF44)
	case queue.JobTypeAudio:
		return color.RGBA{R: 255, G: 215, B: 68, A: 255} // Yellow (#FFD744)
	case queue.JobTypeThumb:
		return color.RGBA{R: 255, G: 136, B: 68, A: 255} // Orange (#FF8844)
	case queue.JobTypeAuthor:
		return color.RGBA{R: 255, G: 170, B: 68, A: 255} // Orange (#FFAA44)
	case queue.JobTypeRip:
		return color.RGBA{R: 255, G: 153, B: 68, A: 255} // Orange (#FF9944)
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
