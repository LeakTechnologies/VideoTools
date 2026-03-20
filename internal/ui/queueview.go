package ui

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
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

type queueCallbacks struct {
	onBack        func()
	onPause       func(string)
	onResume      func(string)
	onCancel      func(string)
	onRemove      func(string)
	onMoveUp      func(string)
	onMoveDown    func(string)
	onPauseAll    func()
	onResumeAll   func()
	onStart       func()
	onClear       func()
	onClearAll    func()
	onCopyError   func(string)
	onViewLog     func(string)
	onCopyCommand func(string)
	onOpenFolder  func(string)
	onOpenOutput  func(string)
}

type queueItemWidgets struct {
	jobID       string
	status      queue.JobStatus
	container   fyne.CanvasObject
	titleLabel  *widget.Label
	descLabel   *widget.Label
	statusLabel *widget.Label
	progress    *StripedProgress
	buttonBox   *fyne.Container
	statusRect  *canvas.Rectangle
}

type QueueView struct {
	Root       fyne.CanvasObject
	Scroll     *container.Scroll
	jobList    *fyne.Container
	emptyLabel fyne.CanvasObject
	items      map[string]*queueItemWidgets
	callbacks  queueCallbacks
	bgColor    color.Color
	textColor  color.Color
}

func (v *QueueView) StopAnimations() {
	for _, item := range v.items {
		if item != nil && item.progress != nil {
			item.progress.StopAnimation()
		}
	}
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
	onOpenFolder func(string),
	onOpenOutput func(string),
	titleColor, bgColor, textColor color.Color,
) *QueueView {
	t := i18n.T()

	// Header
	titleText := t.QueueTitle
	if titleText == "" {
		titleText = "Queue"
	}
	title := canvas.NewText(strings.ToUpper(titleText), titleColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	backBtn := widget.NewButton(t.ActionBack, onBack)
	backBtn.Importance = widget.LowImportance

	startAllBtn := widget.NewButton(t.ActionQueueStart, onStart)
	startAllBtn.Importance = widget.MediumImportance

	pauseAllBtn := widget.NewButton(t.ActionQueuePauseAll, onPauseAll)
	pauseAllBtn.Importance = widget.LowImportance

	resumeAllBtn := widget.NewButton(t.ActionQueueResumeAll, onResumeAll)
	resumeAllBtn.Importance = widget.LowImportance

	clearBtn := widget.NewButton(t.ActionQueueClearCompleted, onClear)
	clearBtn.Importance = widget.LowImportance

	clearAllBtn := widget.NewButton(t.ActionClearAll, onClearAll)
	clearAllBtn.Importance = widget.DangerImportance

	buttonRow := container.NewHBox(startAllBtn, pauseAllBtn, resumeAllBtn, clearAllBtn, clearBtn)

	header := container.NewBorder(
		nil, nil,
		backBtn,
		buttonRow,
		container.NewCenter(title),
	)

	jobList := container.NewVBox()
	emptyMsg := widget.NewLabel(t.QueueEmpty)
	emptyMsg.Alignment = fyne.TextAlignCenter
	emptyLabel := container.NewCenter(emptyMsg)

	// Use a scroll container anchored to the top to avoid jumpy scroll-to-content behavior.
	scrollable := container.NewScroll(jobList)
	// scrollable.SetMinSize(fyne.NewSize(0, 0)) // Removed for flexible sizing
	scrollable.Offset = fyne.NewPos(0, 0)

	body := container.NewBorder(
		header,
		nil, nil, nil,
		scrollable,
	)

	view := &QueueView{
		Root:       container.NewPadded(body),
		Scroll:     scrollable,
		jobList:    jobList,
		emptyLabel: emptyLabel,
		items:      make(map[string]*queueItemWidgets),
		callbacks: queueCallbacks{
			onBack:        onBack,
			onPause:       onPause,
			onResume:      onResume,
			onCancel:      onCancel,
			onRemove:      onRemove,
			onMoveUp:      onMoveUp,
			onMoveDown:    onMoveDown,
			onPauseAll:    onPauseAll,
			onResumeAll:   onResumeAll,
			onStart:       onStart,
			onClear:       onClear,
			onClearAll:    onClearAll,
			onCopyError:   onCopyError,
			onViewLog:     onViewLog,
			onCopyCommand: onCopyCommand,
			onOpenFolder:  onOpenFolder,
			onOpenOutput:  onOpenOutput,
		},
		bgColor:   bgColor,
		textColor: textColor,
	}
	view.UpdateJobs(jobs)
	return view
}

// buildJobItem creates a single job item in the queue list
func buildJobItem(
	job *queue.Job,
	queuePositions map[string]int,
	callbacks queueCallbacks,
	bgColor, textColor color.Color,
) *queueItemWidgets {
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
	descLabel.Wrapping = fyne.TextTruncate

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
	statusLabel.Wrapping = fyne.TextTruncate

	buttonBox := buildJobButtons(job, callbacks)

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
	// card.SetMinSize(fyne.NewSize(0, 140)) // Removed for flexible sizing

	item := container.NewPadded(
		container.NewMax(card, content),
	)

	// Wrap with draggable to allow drag-to-reorder (up/down by drag direction)
	wrapped := newDraggableJobItem(job.ID, item, func(id string, dir int) {
		if dir < 0 {
			callbacks.onMoveUp(id)
		} else if dir > 0 {
			callbacks.onMoveDown(id)
		}
	})

	return &queueItemWidgets{
		jobID:       job.ID,
		status:      job.Status,
		container:   wrapped,
		titleLabel:  titleLabel,
		descLabel:   descLabel,
		statusLabel: statusLabel,
		progress:    progress,
		buttonBox:   buttonBox,
		statusRect:  statusRect,
	}
}

func buildJobButtons(job *queue.Job, callbacks queueCallbacks) *fyne.Container {
	var buttons []fyne.CanvasObject

	if job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused {
		buttons = append(buttons,
			widget.NewButtonWithIcon("", GetIcon("keyboard_arrow_up"), func() { callbacks.onMoveUp(job.ID) }),
			widget.NewButtonWithIcon("", GetIcon("keyboard_arrow_down"), func() { callbacks.onMoveDown(job.ID) }),
		)
	}

	switch job.Status {
	case queue.JobStatusRunning:
		buttons = append(buttons,
			widget.NewButton("Copy Command", func() { callbacks.onCopyCommand(job.ID) }),
			widget.NewButton("Pause", func() { callbacks.onPause(job.ID) }),
			widget.NewButton("Cancel", func() { callbacks.onCancel(job.ID) }),
		)
	case queue.JobStatusPaused:
		buttons = append(buttons,
			widget.NewButton("Resume", func() { callbacks.onResume(job.ID) }),
			widget.NewButton("Cancel", func() { callbacks.onCancel(job.ID) }),
		)
	case queue.JobStatusPending:
		buttons = append(buttons,
			widget.NewButton("Copy Command", func() { callbacks.onCopyCommand(job.ID) }),
			widget.NewButton("Remove", func() { callbacks.onRemove(job.ID) }),
		)
	case queue.JobStatusCompleted, queue.JobStatusFailed, queue.JobStatusCancelled:
		// Open in Folder + Play/Open stacked, shown whenever there is an output file.
		if job.OutputFile != "" && (callbacks.onOpenFolder != nil || callbacks.onOpenOutput != nil) {
			openLabel := openOutputLabel(job.OutputFile)
			openFolderBtn := widget.NewButton("Open in Folder", func() {
				if callbacks.onOpenFolder != nil {
					callbacks.onOpenFolder(job.ID)
				}
			})
			openFolderBtn.Importance = widget.LowImportance
			openOutputBtn := widget.NewButton(openLabel, func() {
				if callbacks.onOpenOutput != nil {
					callbacks.onOpenOutput(job.ID)
				}
			})
			openOutputBtn.Importance = widget.LowImportance
			buttons = append(buttons, container.NewVBox(openFolderBtn, openOutputBtn))
		}
		if job.Status == queue.JobStatusFailed && strings.TrimSpace(job.Error) != "" && callbacks.onCopyError != nil {
			buttons = append(buttons,
				widget.NewButton("Copy Error", func() { callbacks.onCopyError(job.ID) }),
			)
		}
		if job.LogPath != "" && callbacks.onViewLog != nil {
			buttons = append(buttons,
				widget.NewButton("View Log", func() { callbacks.onViewLog(job.ID) }),
			)
		}
		buttons = append(buttons,
			widget.NewButton("Remove", func() { callbacks.onRemove(job.ID) }),
		)
	}

	return container.NewHBox(buttons...)
}

// openOutputLabel returns an appropriate button label based on the output file type.
func openOutputLabel(outputFile string) string {
	ext := strings.ToLower(filepath.Ext(outputFile))
	switch ext {
	case ".mp4", ".mkv", ".avi", ".mov", ".mpg", ".mpeg", ".ts", ".m2ts", ".wmv", ".flv", ".webm", ".iso":
		return "Play Video"
	case ".jpg", ".jpeg", ".png", ".webp", ".tiff", ".tif", ".bmp", ".gif":
		return "Open Image"
	case ".mp3", ".aac", ".flac", ".ogg", ".wav", ".m4a", ".opus":
		return "Play Audio"
	default:
		return "Open File"
	}
}

func updateJobItem(item *queueItemWidgets, job *queue.Job, queuePositions map[string]int, callbacks queueCallbacks) {
	item.titleLabel.SetText(utils.ShortenMiddle(job.Title, 60))
	item.descLabel.SetText(utils.ShortenMiddle(job.Description, 90))
	item.statusLabel.SetText(getStatusText(job, queuePositions))

	if job.Status == queue.JobStatusCompleted {
		item.progress.SetProgress(1.0)
	} else {
		item.progress.SetProgress(job.Progress / 100.0)
	}

	if job.Status == queue.JobStatusRunning {
		item.progress.SetActivity(job.Progress <= 0.01)
		item.progress.StartAnimation()
	} else {
		item.progress.SetActivity(false)
		item.progress.StopAnimation()
	}

	if item.status != job.Status {
		item.status = job.Status
		if item.statusRect != nil {
			item.statusRect.FillColor = GetStatusColor(job.Status)
			item.statusRect.Refresh()
		}
		item.buttonBox.Objects = buildJobButtons(job, callbacks).Objects
		item.buttonBox.Refresh()
	}
}

func (v *QueueView) UpdateJobs(jobs []*queue.Job) {
	if len(jobs) == 0 {
		v.jobList.Objects = []fyne.CanvasObject{v.emptyLabel}
		v.jobList.Refresh()
		v.Scroll.Refresh()
		return
	}

	queuePositions := make(map[string]int)
	position := 1
	for _, job := range jobs {
		if job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused {
			queuePositions[job.ID] = position
			position++
		}
	}

	ordered := make([]fyne.CanvasObject, 0, len(jobs))
	seen := make(map[string]struct{}, len(jobs))
	// Track whether the list structure (count or order) changed.
	structureChanged := len(jobs) != len(v.jobList.Objects)

	for i, job := range jobs {
		seen[job.ID] = struct{}{}
		item := v.items[job.ID]
		if item == nil {
			item = buildJobItem(job, queuePositions, v.callbacks, v.bgColor, v.textColor)
			v.items[job.ID] = item
			structureChanged = true
		} else {
			updateJobItem(item, job, queuePositions, v.callbacks)
			// Check if position in list changed.
			if !structureChanged && i < len(v.jobList.Objects) && v.jobList.Objects[i] != item.container {
				structureChanged = true
			}
		}
		ordered = append(ordered, item.container)
	}

	for id := range v.items {
		if _, ok := seen[id]; !ok {
			delete(v.items, id)
			structureChanged = true
		}
	}

	// Only update the list widget when structure changed; individual widget refreshes
	// (SetText, SetProgress, Refresh on buttonBox) are handled by updateJobItem above.
	if structureChanged {
		v.jobList.Objects = ordered
		v.jobList.Refresh()
		v.Scroll.Refresh()
	}
}

// UpdateRunningStatus updates elapsed/progress text for running jobs without rebuilding the list.
func (v *QueueView) UpdateRunningStatus(jobs []*queue.Job) {
	if len(jobs) == 0 {
		return
	}
	queuePositions := make(map[string]int)
	position := 1
	for _, job := range jobs {
		if job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused {
			queuePositions[job.ID] = position
			position++
		}
	}

	for _, job := range jobs {
		if job.Status != queue.JobStatusRunning {
			continue
		}
		item := v.items[job.ID]
		if item == nil {
			continue
		}
		updateJobItem(item, job, queuePositions, v.callbacks)
	}
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

// ModuleColor returns rainbow ROYGBIV colors matching main module palette
func ModuleColor(t queue.JobType) color.Color {
	switch t {
	case queue.JobTypeConvert:
		return color.RGBA{R: 111, G: 66, B: 193, A: 255} // Purple (#6F42C1)
	case queue.JobTypeMerge:
		return color.RGBA{R: 46, G: 125, B: 50, A: 255} // Green (#2E7D32)
	case queue.JobTypeTrim:
		return color.RGBA{R: 239, G: 108, B: 0, A: 255} // Orange (#EF6C00)
	case queue.JobTypeFilter:
		return color.RGBA{R: 63, G: 81, B: 181, A: 255} // Blue (#3F51B5)
	case queue.JobTypeUpscale:
		return color.RGBA{R: 194, G: 24, B: 91, A: 255} // Pink (#C2185B)
	case queue.JobTypeAudio:
		return color.RGBA{R: 46, G: 125, B: 50, A: 255} // Green (#2E7D32)
	case queue.JobTypeThumbnail:
		return color.RGBA{R: 63, G: 81, B: 181, A: 255} // Blue (#3F51B5)
	case queue.JobTypeAuthor:
		return color.RGBA{R: 239, G: 108, B: 0, A: 255} // Orange (#EF6C00)
	case queue.JobTypeRip:
		return color.RGBA{R: 46, G: 125, B: 50, A: 255} // Green (#2E7D32)
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
