package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	onStopPreview    func()
	onBack           func()
	onPause          func(string)
	onResume         func(string)
	onCancel         func(string)
	onRemove         func(string)
	onMoveUp         func(string)
	onMoveDown       func(string)
	onPauseAll       func()
	onResumeAll      func()
	onStart          func()
	onClear          func()
	onClearAll       func()
	onCancelAll      func()
	onRetry          func(string)
	onCopyError      func(string)
	onViewLog        func(string)
	onCopyCommand    func(string)
	onOpenFolder     func(string)
	onOpenOutput     func(string)
	onBurnISO        func(string)
	onOpenInModule   func(string, string) // jobID, module name
	onScheduleModule func(string, string) // jobID, module name - for pending jobs
	Window           fyne.Window          // needed for context menu positioning
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

	statusBadgeLabel *canvas.Text

	// Live output panel (shown when a job is running)
	logSection  *fyne.Container
	logJobLabel *widget.Label
	logEntry    *widget.Label
	logScroll   *container.Scroll
	logPath     string
	logReading  bool // true while a read goroutine is in flight
	logMu       sync.Mutex
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
	onCancelAll func(),
	onRetry func(string),
	onCopyError func(string),
	onViewLog func(string),
	onCopyCommand func(string),
onOpenFolder func(string),
	onOpenOutput func(string),
	onBurnISO func(string),
	onOpenInModule func(string, string),
onScheduleModule func(string, string),
	Window fyne.Window,
	titleColor, bgColor, textColor color.Color,
) *QueueView {
	t := i18n.T()

	// Determine if there are active jobs (running or pending)
	hasActiveJobs := false
	for _, job := range jobs {
		if job.Status == queue.JobStatusRunning || job.Status == queue.JobStatusPending {
			hasActiveJobs = true
			break
		}
	}

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

	cancelAllBtn := widget.NewButton(t.ActionQueueCancelAll, onCancelAll)
	cancelAllBtn.Importance = widget.DangerImportance

	// Only show Cancel All button when there are active jobs
	var buttonRow *fyne.Container
	if hasActiveJobs {
		buttonRow = container.NewHBox(startAllBtn, pauseAllBtn, resumeAllBtn, cancelAllBtn, clearAllBtn, clearBtn)
	} else {
		buttonRow = container.NewHBox(startAllBtn, pauseAllBtn, resumeAllBtn, clearAllBtn, clearBtn)
	}

	// Status badge for queue (shows active/completed counts)
	statusBadge := canvas.NewText("", textColor)
	statusBadge.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	statusBadge.TextSize = 11

	// Header with TintedBar (matches other modules)
	headerTitle := container.NewHBox(
		backBtn,
		layout.NewSpacer(),
		statusBadge,
		buttonRow,
	)
	topBar := TintedBar(bgColor, headerTitle)

	jobList := container.NewVBox()
	emptyMsg := widget.NewLabel(t.QueueEmpty)
	emptyMsg.Alignment = fyne.TextAlignCenter
	emptyLabel := container.NewCenter(emptyMsg)

	// Use a scroll container anchored to the top to avoid jumpy scroll-to-content behavior.
	scrollable := container.NewScroll(jobList)
	// scrollable.SetMinSize(fyne.NewSize(0, 0)) // Removed for flexible sizing
	scrollable.Offset = fyne.NewPos(0, 0)

	// Live output panel — shown while a job is running
	logJobLabel := widget.NewLabel("")
	logJobLabel.TextStyle = fyne.TextStyle{Bold: true}
	logJobLabel.Truncation = fyne.TextTruncateEllipsis

	logEntry := widget.NewLabel("")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.TextStyle = fyne.TextStyle{Monospace: true}

	logScroll := container.NewVScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 160))

	logBg := canvas.NewRectangle(color.NRGBA{R: 0x0a, G: 0x0d, B: 0x18, A: 0xff})

	logHeaderLabel := canvas.NewText("LIVE OUTPUT", titleColor)
	logHeaderLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	logHeaderLabel.TextSize = 11

	logHeader := container.NewHBox(logHeaderLabel, logJobLabel)

	// Live output content area
	logContent := container.NewMax(logBg, logScroll)

	// Build the inner log section with header and content
	innerLogSection := container.NewBorder(
		container.NewPadded(logHeader),
		nil, nil, nil,
		container.NewPadded(logContent),
	)

	// Wrap with 4px VT green outline using Border layout
	// Top/bottom/left/right borders are 4px green rectangles
	vtGreen := color.NRGBA{R: 0x4c, G: 0xe8, B: 0x70, A: 0xff}
	topBorder := canvas.NewRectangle(vtGreen)
	topBorder.SetMinSize(fyne.NewSize(0, 4))
	bottomBorder := canvas.NewRectangle(vtGreen)
	bottomBorder.SetMinSize(fyne.NewSize(0, 4))
	leftBorder := canvas.NewRectangle(vtGreen)
	leftBorder.SetMinSize(fyne.NewSize(4, 0))
	rightBorder := canvas.NewRectangle(vtGreen)
	rightBorder.SetMinSize(fyne.NewSize(4, 0))

	logSection := container.NewBorder(
		topBorder,
		bottomBorder,
		leftBorder,
		rightBorder,
		innerLogSection,
	)
	// Live output now visible by default (was hidden before)
// logSection.Hide() - removed to show live output panel

	// Bottom TintedBar (matches other modules like benchmark)
	bottomBar := TintedBar(vtGreen, layout.NewSpacer())

	// Use BorderLayout: top bar (TintedBar), bottom bar (TintedBar), content fills middle
	// Live output (logSection) is pinned at bottom of content area
	bodyWithBars := container.NewBorder(
		topBar,
		bottomBar,
		nil, nil,
		container.NewBorder(
			nil,
			logSection,
			nil, nil,
			scrollable,
		),
	)

	view := &QueueView{
		Root:        bodyWithBars,
		Scroll:      scrollable,
		jobList:     jobList,
		emptyLabel:  emptyLabel,
		items:       make(map[string]*queueItemWidgets),
		statusBadgeLabel: statusBadge,
		logSection:  logSection,
		logJobLabel: logJobLabel,
		logEntry:    logEntry,
		logScroll:   logScroll,
		callbacks:  queueCallbacks{
			onPause:    onPause,
			onResume:   onResume,
			onCancel:   onCancel,
			onRemove:   onRemove,
			onMoveUp:   onMoveUp,
			onMoveDown: onMoveDown,
			onPauseAll: onPauseAll,
			onResumeAll: onResumeAll,
			onStart:     onStart,
			onClear:     onClear,
			onClearAll:  onClearAll,
			onCancelAll: onCancelAll,
			onRetry:     onRetry,
			onCopyError: onCopyError,
			onViewLog:   onViewLog,
			onCopyCommand: onCopyCommand,
			onOpenFolder: onOpenFolder,
			onOpenOutput: onOpenOutput,
			onBurnISO:    onBurnISO,
			onOpenInModule: onOpenInModule,
			onScheduleModule: onScheduleModule,
		},
	}
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

	// Thumbnail image with 3px colored outline matching module color
	var thumbnailWidget fyne.CanvasObject
	if job.ThumbnailPath != "" {
		if img, err := fyne.LoadResourceFromPath(job.ThumbnailPath); err == nil {
			thumbImg := canvas.NewImageFromResource(img)
			thumbImg.FillMode = canvas.ImageFillContain
			thumbImg.SetMinSize(fyne.NewSize(120, 68)) // 16:9 aspect ratio
			// 3px outline using module color
			moduleColor := ModuleColor(job.Type)
			outlineBg := canvas.NewRectangle(moduleColor)
			outlineBg.SetMinSize(fyne.NewSize(126, 74)) // 120+6 x 68+6 = 3px each side
			thumbnailWidget = container.NewMax(outlineBg, container.NewPadded(thumbImg))
		}
	}

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

	// Main content with optional thumbnail on the left
	var content fyne.CanvasObject
	if thumbnailWidget != nil {
		content = container.NewBorder(
			nil, nil,
			statusRect,
			buttonBox,
			container.NewHBox(thumbnailWidget, layout.NewSpacer(), infoBox),
		)
	} else {
		content = container.NewBorder(
			nil, nil,
			statusRect,
			buttonBox,
			infoBox,
		)
	}

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

	// Add right-click context menu
	wrapped.onTappedSecondary = func(ev *fyne.PointEvent) {
		buildQueueItemContextMenu(job, callbacks, ev)
	}

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
		// For author jobs, show Burn DVD button if ISO output
		if job.Type == queue.JobTypeAuthor && job.Status == queue.JobStatusCompleted {
			outputExt := strings.ToLower(filepath.Ext(job.OutputFile))
			if outputExt == ".iso" && callbacks.onBurnISO != nil {
				burnBtn := widget.NewButton("Burn DVD", func() {
					callbacks.onBurnISO(job.ID)
				})
				burnBtn.Importance = widget.MediumImportance
				buttons = append(buttons, burnBtn)
			}
		}

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
		if (job.Status == queue.JobStatusFailed || job.Status == queue.JobStatusCancelled) && callbacks.onRetry != nil && isRetryableJobType(job.Type) {
			retryBtn := widget.NewButton("Retry", func() { callbacks.onRetry(job.ID) })
			retryBtn.Importance = widget.MediumImportance
			buttons = append(buttons, retryBtn)
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

// isRetryableJobType returns true for processing jobs that produce output and can be safely retried.
func isRetryableJobType(t queue.JobType) bool {
	switch t {
	case queue.JobTypePlayer, queue.JobTypeInspect, queue.JobTypeCompare, queue.JobTypeBenchmark:
		return false
	default:
		return true
	}
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
		v.Root.Refresh()
	}

	// Update status badge with job counts
	activeCount, completedCount, failedCount := 0, 0, 0
	for _, job := range jobs {
		switch job.Status {
		case queue.JobStatusRunning, queue.JobStatusPending, queue.JobStatusPaused:
			activeCount++
		case queue.JobStatusCompleted:
			completedCount++
		case queue.JobStatusFailed, queue.JobStatusCancelled:
			failedCount++
		}
	}

	badgeText := ""
	if activeCount > 0 {
		badgeText += i18n.T().QueueInProgress + ":" + strconv.Itoa(activeCount)
	}
	if completedCount > 0 {
		if badgeText != "" {
			badgeText += " "
		}
		badgeText += i18n.T().QueueCompleted + ":" + strconv.Itoa(completedCount)
	}
	if failedCount > 0 {
		if badgeText != "" {
			badgeText += " "
		}
		badgeText += i18n.T().QueueFailed + ":" + strconv.Itoa(failedCount)
	}

	v.statusBadgeLabel.Text = badgeText
	v.statusBadgeLabel.Refresh()
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

	var runningJob *queue.Job
	for _, job := range jobs {
		if job.Status != queue.JobStatusRunning {
			continue
		}
		item := v.items[job.ID]
		if item == nil {
			continue
		}
		updateJobItem(item, job, queuePositions, v.callbacks)
		runningJob = job
	}

	if runningJob != nil && runningJob.LogPath != "" {
		v.showLiveLog(runningJob.Title, runningJob.LogPath)
	} else {
		v.hideLiveLog()
	}
}

// showLiveLog reads the tail of the job's log file and updates the live output panel.
// If a read is already in flight, this call is skipped to avoid pile-up.
func (v *QueueView) showLiveLog(title, logPath string) {
	v.logMu.Lock()
	if v.logReading {
		v.logMu.Unlock()
		return
	}
	v.logPath = logPath
	v.logReading = true
	v.logMu.Unlock()

	go func() {
		content := readLogTail(logPath, 80)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.logMu.Lock()
			v.logReading = false
			v.logMu.Unlock()
			if v.logSection == nil {
				return
			}
			v.logSection.Show()
			shortTitle := "  —  " + utils.ShortenMiddle(title, 50)
			v.logJobLabel.SetText(shortTitle)
			v.logEntry.SetText(content)
			// Scroll to bottom and refresh layout
			v.logScroll.ScrollToBottom()
			v.logScroll.Refresh()
			v.Root.Refresh()
		}, false)
	}()
}

// hideLiveLog hides the live output panel.
func (v *QueueView) hideLiveLog() {
	if v.logSection != nil {
		v.logSection.Hide()
		v.Root.Refresh()
	}
}

// readLogTail reads the last n lines from the given file path.
// Returns empty string if the file cannot be read.
func readLogTail(path string, lines int) string {
	const maxRead = 256 * 1024 // 256 KB cap
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	start := size - maxRead
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}

	// Split into lines and take the last n
	parts := bytes.Split(data, []byte("\n"))
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.TrimRight(string(bytes.Join(parts, []byte("\n"))), "\n")
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
		remaining := ""
		if job.StartedAt != nil {
			elapsed = fmt.Sprintf(" | Elapsed: %s", time.Since(*job.StartedAt).Round(time.Second))
			if job.Progress > 0 && job.Progress < 100 {
				elapsedSec := time.Since(*job.StartedAt).Seconds()
				remainingSec := elapsedSec*(100/job.Progress) - elapsedSec
				remaining = fmt.Sprintf(" | Remaining: %s", time.Duration(remainingSec*float64(time.Second)).Round(time.Second))
			}
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

		return fmt.Sprintf("Status: Running | Progress: %.1f%%%s%s%s", job.Progress, elapsed, remaining, extras)
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
		return color.RGBA{R: 43, G: 156, B: 28, A: 255} // Green (#2B9C1C) - matching upscale module
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
	jobID             string
	content           fyne.CanvasObject
	onReorder         func(string, int) // id, direction (-1 up, +1 down)
	onTappedSecondary func(*fyne.PointEvent)
	accumY            float32
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

func (d *draggableJobItem) TappedSecondary(ev *fyne.PointEvent) {
	if d.onTappedSecondary != nil {
		d.onTappedSecondary(ev)
	}
}

func buildQueueItemContextMenu(job *queue.Job, callbacks queueCallbacks, ev *fyne.PointEvent) {
	t := i18n.T()
	menu := fyne.NewMenu("")

	// Add Open item only if callback is available
	if callbacks.onOpenOutput != nil {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label:  t.FileManagerOpenInspect,
			Action: func() { callbacks.onOpenOutput(job.ID) },
		})
	}

	if job.Status == queue.JobStatusCompleted && callbacks.onOpenInModule != nil {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label:  t.FileManagerOpenConvert,
			Action: func() { callbacks.onOpenInModule(job.ID, "convert") },
		})
	}

	if (job.Status == queue.JobStatusPending || job.Status == queue.JobStatusPaused) && callbacks.onScheduleModule != nil {
		menu.Items = append(menu.Items, &fyne.MenuItem{
			Label:  "Schedule: Convert on completion",
			Action: func() { callbacks.onScheduleModule(job.ID, "convert") },
		})
	}

	// Don't show empty menu
	if len(menu.Items) == 0 {
		return
	}

	// Defensive: ensure Window and Canvas are valid
	if callbacks.Window == nil {
		return
	}
	canvas := callbacks.Window.Canvas()
	if canvas == nil {
		return
	}

	pop := widget.NewPopUpMenu(menu, canvas)
	pop.ShowAtPosition(ev.Position)
}
