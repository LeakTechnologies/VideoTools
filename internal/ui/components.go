package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

var (
	// GridColor is the color used for grid lines and borders
	GridColor color.Color
	// TextColor is the main text color
	TextColor color.Color
)

// SetColors sets the UI colors
func SetColors(grid, text color.Color) {
	GridColor = grid
	TextColor = text
}

// MonoTheme ensures all text uses a monospace font
type MonoTheme struct{}

func (m *MonoTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m *MonoTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Monospace = true
	return theme.DefaultTheme().Font(style)
}

func (m *MonoTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MonoTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// ModuleTile is a clickable tile widget for module selection
type ModuleTile struct {
	widget.BaseWidget
	label       string
	color       color.Color
	enabled     bool
	onTapped    func()
	onDropped   func([]fyne.URI)
	flashing    bool
	draggedOver bool
}

// NewModuleTile creates a new module tile
func NewModuleTile(label string, col color.Color, enabled bool, tapped func(), dropped func([]fyne.URI)) *ModuleTile {
	m := &ModuleTile{
		label:     strings.ToUpper(label),
		color:     col,
		enabled:   enabled,
		onTapped:  tapped,
		onDropped: dropped,
	}
	m.ExtendBaseWidget(m)
	return m
}

// DraggedOver implements desktop.Droppable interface
func (m *ModuleTile) DraggedOver(pos fyne.Position) {
	logging.Debug(logging.CatUI, "DraggedOver tile=%s enabled=%v pos=%v", m.label, m.enabled, pos)
	if m.enabled {
		m.draggedOver = true
		m.Refresh()
	}
}

// DraggedOut is called when drag leaves the tile
func (m *ModuleTile) DraggedOut() {
	logging.Debug(logging.CatUI, "DraggedOut tile=%s", m.label)
	m.draggedOver = false
	m.Refresh()
}

// Dropped implements desktop.Droppable interface
func (m *ModuleTile) Dropped(pos fyne.Position, items []fyne.URI) {
	fmt.Printf("[DROPTILE] Dropped on tile=%s enabled=%v itemCount=%d\n", m.label, m.enabled, len(items))
	logging.Debug(logging.CatUI, "Dropped on tile=%s enabled=%v items=%v", m.label, m.enabled, items)
	// Reset dragged over state
	m.draggedOver = false

	if m.enabled && m.onDropped != nil {
		fmt.Printf("[DROPTILE] Calling callback for %s\n", m.label)
		logging.Debug(logging.CatUI, "Calling onDropped callback for %s", m.label)
		// Trigger flash animation
		m.flashing = true
		m.Refresh()
		// Reset flash after 300ms
		time.AfterFunc(300*time.Millisecond, func() {
			m.flashing = false
			m.Refresh()
		})
		m.onDropped(items)
	} else {
		fmt.Printf("[DROPTILE] Drop IGNORED on %s: enabled=%v hasCallback=%v\n", m.label, m.enabled, m.onDropped != nil)
		logging.Debug(logging.CatUI, "Drop ignored: enabled=%v hasCallback=%v", m.enabled, m.onDropped != nil)
	}
}

func (m *ModuleTile) CreateRenderer() fyne.WidgetRenderer {
	tileColor := m.color
	labelColor := TextColor

	// Dim disabled tiles
	if !m.enabled {
		// Reduce opacity by mixing with dark background
		if c, ok := m.color.(color.NRGBA); ok {
			tileColor = color.NRGBA{R: c.R / 3, G: c.G / 3, B: c.B / 3, A: c.A}
		}
		if c, ok := TextColor.(color.NRGBA); ok {
			labelColor = color.NRGBA{R: c.R / 2, G: c.G / 2, B: c.B / 2, A: c.A}
		}
	}

	bg := canvas.NewRectangle(tileColor)
	bg.CornerRadius = 8
	bg.StrokeColor = GridColor
	bg.StrokeWidth = 1

	txt := canvas.NewText(m.label, labelColor)
	txt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	txt.Alignment = fyne.TextAlignCenter
	txt.TextSize = 20

	return &moduleTileRenderer{
		tile:  m,
		bg:    bg,
		label: txt,
	}
}

func (m *ModuleTile) Tapped(*fyne.PointEvent) {
	if m.enabled && m.onTapped != nil {
		m.onTapped()
	}
}

type moduleTileRenderer struct {
	tile  *ModuleTile
	bg    *canvas.Rectangle
	label *canvas.Text
}

func (r *moduleTileRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	// Center the label by positioning it in the middle
	labelSize := r.label.MinSize()
	r.label.Resize(labelSize)
	x := (size.Width - labelSize.Width) / 2
	y := (size.Height - labelSize.Height) / 2
	r.label.Move(fyne.NewPos(x, y))
}

func (r *moduleTileRenderer) MinSize() fyne.Size {
	return fyne.NewSize(220, 110)
}

func (r *moduleTileRenderer) Refresh() {
	r.bg.FillColor = r.tile.color

	// Apply visual feedback based on state
	if r.tile.flashing {
		// Flash animation - white outline
		r.bg.StrokeColor = color.White
		r.bg.StrokeWidth = 3
	} else if r.tile.draggedOver {
		// Dragging over - cyan/blue outline to indicate drop zone
		r.bg.StrokeColor = color.NRGBA{R: 0, G: 200, B: 255, A: 255}
		r.bg.StrokeWidth = 3
	} else {
		// Normal state
		r.bg.StrokeColor = GridColor
		r.bg.StrokeWidth = 1
	}

	r.bg.Refresh()
	r.label.Text = r.tile.label
	r.label.Refresh()
}

func (r *moduleTileRenderer) Destroy() {}

func (r *moduleTileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label}
}

// TintedBar creates a colored bar container
func TintedBar(col color.Color, body fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(col)
	rect.SetMinSize(fyne.NewSize(0, 48))
	padded := container.NewPadded(body)
	return container.NewMax(rect, padded)
}

// Tappable wraps any canvas object and makes it tappable
type Tappable struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	onTapped func()
}

// NewTappable creates a new tappable wrapper
func NewTappable(content fyne.CanvasObject, onTapped func()) *Tappable {
	t := &Tappable{
		content:  content,
		onTapped: onTapped,
	}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer creates the renderer for the tappable
func (t *Tappable) CreateRenderer() fyne.WidgetRenderer {
	return &tappableRenderer{
		tappable: t,
		content:  t.content,
	}
}

// Tapped handles tap events
func (t *Tappable) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

type tappableRenderer struct {
	tappable *Tappable
	content  fyne.CanvasObject
}

func (r *tappableRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *tappableRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *tappableRenderer) Refresh() {
	r.content.Refresh()
}

func (r *tappableRenderer) Destroy() {}

func (r *tappableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

// Droppable wraps any canvas object and makes it a drop target (files/URIs)
type Droppable struct {
	widget.BaseWidget
	content   fyne.CanvasObject
	onDropped func([]fyne.URI)
}

// NewDroppable creates a new droppable wrapper
func NewDroppable(content fyne.CanvasObject, onDropped func([]fyne.URI)) *Droppable {
	d := &Droppable{
		content:   content,
		onDropped: onDropped,
	}
	d.ExtendBaseWidget(d)
	return d
}

// CreateRenderer creates the renderer for the droppable
func (d *Droppable) CreateRenderer() fyne.WidgetRenderer {
	return &droppableRenderer{
		droppable: d,
		content:   d.content,
	}
}

// DraggedOver highlights when drag is over (optional)
func (d *Droppable) DraggedOver(pos fyne.Position) {
	_ = pos
}

// DraggedOut clears highlight (optional)
func (d *Droppable) DraggedOut() {
}

// Dropped handles drop events
func (d *Droppable) Dropped(_ fyne.Position, items []fyne.URI) {
	if d.onDropped != nil && len(items) > 0 {
		d.onDropped(items)
	}
}

type droppableRenderer struct {
	droppable *Droppable
	content   fyne.CanvasObject
}

func (r *droppableRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *droppableRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *droppableRenderer) Refresh() {
	r.content.Refresh()
}

func (r *droppableRenderer) Destroy() {}

func (r *droppableRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

// DraggableVScroll creates a vertical scroll container with draggable track
type DraggableVScroll struct {
	widget.BaseWidget
	content fyne.CanvasObject
	scroll  *container.Scroll
}

// NewDraggableVScroll creates a new draggable vertical scroll container
func NewDraggableVScroll(content fyne.CanvasObject) *DraggableVScroll {
	d := &DraggableVScroll{
		content: content,
		scroll:  container.NewVScroll(content),
	}
	d.ExtendBaseWidget(d)
	return d
}

// CreateRenderer creates the renderer for the draggable scroll
func (d *DraggableVScroll) CreateRenderer() fyne.WidgetRenderer {
	return &draggableScrollRenderer{
		scroll: d.scroll,
	}
}

// Dragged handles drag events on the scrollbar track
func (d *DraggableVScroll) Dragged(ev *fyne.DragEvent) {
	// Calculate the scroll position based on drag position
	size := d.scroll.Size()
	contentSize := d.content.MinSize()

	if contentSize.Height <= size.Height {
		return // No scrolling needed
	}

	// Calculate scroll ratio (0.0 to 1.0)
	ratio := ev.Position.Y / size.Height
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	// Calculate target offset
	maxOffset := contentSize.Height - size.Height
	targetOffset := ratio * maxOffset

	// Apply scroll offset
	d.scroll.Offset = fyne.NewPos(0, targetOffset)
	d.scroll.Refresh()
}

// DragEnd handles the end of a drag event
func (d *DraggableVScroll) DragEnd() {
	// Nothing needed
}

// Tapped handles tap events on the scrollbar track
func (d *DraggableVScroll) Tapped(ev *fyne.PointEvent) {
	// Jump to tapped position
	size := d.scroll.Size()
	contentSize := d.content.MinSize()

	if contentSize.Height <= size.Height {
		return
	}

	ratio := ev.Position.Y / size.Height
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	maxOffset := contentSize.Height - size.Height
	targetOffset := ratio * maxOffset

	d.scroll.Offset = fyne.NewPos(0, targetOffset)
	d.scroll.Refresh()
}

// Scrolled handles scroll events (mouse wheel)
func (d *DraggableVScroll) Scrolled(ev *fyne.ScrollEvent) {
	d.scroll.Scrolled(ev)
}

type draggableScrollRenderer struct {
	scroll *container.Scroll
}

func (r *draggableScrollRenderer) Layout(size fyne.Size) {
	r.scroll.Resize(size)
}

func (r *draggableScrollRenderer) MinSize() fyne.Size {
	return r.scroll.MinSize()
}

func (r *draggableScrollRenderer) Refresh() {
	r.scroll.Refresh()
}

func (r *draggableScrollRenderer) Destroy() {}

func (r *draggableScrollRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.scroll}
}

// ConversionStatsBar shows current conversion status with live updates
type ConversionStatsBar struct {
	widget.BaseWidget
	running   int
	pending   int
	completed int
	failed    int
	cancelled int
	progress  float64
	jobTitle  string
	fps       float64
	speed     float64
	eta       string
	onTapped  func()
}

// NewConversionStatsBar creates a new conversion stats bar
func NewConversionStatsBar(onTapped func()) *ConversionStatsBar {
	c := &ConversionStatsBar{
		onTapped: onTapped,
	}
	c.ExtendBaseWidget(c)
	return c
}

// UpdateStats updates the stats display
func (c *ConversionStatsBar) UpdateStats(running, pending, completed, failed, cancelled int, progress float64, jobTitle string) {
	c.running = running
	c.pending = pending
	c.completed = completed
	c.failed = failed
	c.cancelled = cancelled
	c.progress = progress
	c.jobTitle = jobTitle
	c.Refresh()
}

// UpdateStatsWithDetails updates the stats display with detailed conversion info
func (c *ConversionStatsBar) UpdateStatsWithDetails(running, pending, completed, failed, cancelled int, progress, fps, speed float64, eta, jobTitle string) {
	c.running = running
	c.pending = pending
	c.completed = completed
	c.failed = failed
	c.cancelled = cancelled
	c.progress = progress
	c.fps = fps
	c.speed = speed
	c.eta = eta
	c.jobTitle = jobTitle
	c.Refresh()
}

// CreateRenderer creates the renderer for the stats bar
func (c *ConversionStatsBar) CreateRenderer() fyne.WidgetRenderer {
	// Transparent background so the parent tinted bar color shows through
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 0
	bg.StrokeWidth = 0

	statusText := canvas.NewText("", color.NRGBA{R: 230, G: 236, B: 245, A: 255})
	statusText.TextStyle = fyne.TextStyle{Monospace: true}
	statusText.TextSize = 11

	progressBar := widget.NewProgressBar()

	return &conversionStatsRenderer{
		bar:         c,
		bg:          bg,
		statusText:  statusText,
		progressBar: progressBar,
	}
}

// Tapped handles tap events
func (c *ConversionStatsBar) Tapped(*fyne.PointEvent) {
	if c.onTapped != nil {
		c.onTapped()
	}
}

// Enable full-width tap target across the bar
func (c *ConversionStatsBar) MouseIn(*desktop.MouseEvent)    {}
func (c *ConversionStatsBar) MouseMoved(*desktop.MouseEvent) {}
func (c *ConversionStatsBar) MouseOut()                      {}

type conversionStatsRenderer struct {
	bar         *ConversionStatsBar
	bg          *canvas.Rectangle
	statusText  *canvas.Text
	progressBar *widget.ProgressBar
}

func (r *conversionStatsRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)

	// Layout text and progress bar
	textSize := r.statusText.MinSize()
	padding := float32(10)

	// If there's a running job, show progress bar
	if r.bar.running > 0 && r.bar.progress > 0 {
		// Show progress bar on right side
		barWidth := float32(120)
		barHeight := float32(20)
		barX := size.Width - barWidth - padding
		barY := (size.Height - barHeight) / 2

		r.progressBar.Resize(fyne.NewSize(barWidth, barHeight))
		r.progressBar.Move(fyne.NewPos(barX, barY))
		r.progressBar.Show()

		// Position text on left
		r.statusText.Move(fyne.NewPos(padding, (size.Height-textSize.Height)/2))
	} else {
		// No progress bar, center text
		r.progressBar.Hide()
		r.statusText.Move(fyne.NewPos(padding, (size.Height-textSize.Height)/2))
	}
}

func (r *conversionStatsRenderer) MinSize() fyne.Size {
	// Only constrain height, allow width to flex
	return fyne.NewSize(0, 36)
}

func (r *conversionStatsRenderer) Refresh() {
	// Update status text
	if r.bar.running > 0 {
		statusStr := ""
		if r.bar.jobTitle != "" {
			// Truncate job title if too long
			title := r.bar.jobTitle
			if len(title) > 30 {
				title = title[:27] + "..."
			}
			statusStr = title
		} else {
			statusStr = "Processing"
		}

		// Always show progress percentage when running (even if 0%)
		statusStr += " • " + formatProgress(r.bar.progress)

		// Show FPS if available
		if r.bar.fps > 0 {
			statusStr += fmt.Sprintf(" • %.0f fps", r.bar.fps)
		}

		// Show speed if available
		if r.bar.speed > 0 {
			statusStr += fmt.Sprintf(" • %.2fx", r.bar.speed)
		}

		// Show ETA if available
		if r.bar.eta != "" {
			statusStr += " • ETA " + r.bar.eta
		}

		if r.bar.pending > 0 {
			statusStr += " • " + formatCount(r.bar.pending, "pending")
		}

		r.statusText.Text = "▶ " + statusStr
		r.statusText.Color = color.NRGBA{R: 100, G: 220, B: 100, A: 255} // Green

		// Update progress bar (show even at 0%)
		r.progressBar.SetValue(r.bar.progress / 100.0)
		r.progressBar.Show()
	} else if r.bar.pending > 0 {
		r.statusText.Text = "⏸ " + formatCount(r.bar.pending, "queued")
		r.statusText.Color = color.NRGBA{R: 255, G: 200, B: 100, A: 255} // Yellow
		r.progressBar.Hide()
	} else if r.bar.completed > 0 || r.bar.failed > 0 || r.bar.cancelled > 0 {
		statusStr := "✓ "
		parts := []string{}
		if r.bar.completed > 0 {
			parts = append(parts, formatCount(r.bar.completed, "completed"))
		}
		if r.bar.failed > 0 {
			parts = append(parts, formatCount(r.bar.failed, "failed"))
		}
		if r.bar.cancelled > 0 {
			parts = append(parts, formatCount(r.bar.cancelled, "cancelled"))
		}
		statusStr += strings.Join(parts, " • ")
		r.statusText.Text = statusStr
		r.statusText.Color = color.NRGBA{R: 150, G: 150, B: 150, A: 255} // Gray
		r.progressBar.Hide()
	} else {
		r.statusText.Text = "○ No active jobs"
		r.statusText.Color = color.NRGBA{R: 100, G: 100, B: 100, A: 255} // Dim gray
		r.progressBar.Hide()
	}

	r.statusText.Refresh()
	r.progressBar.Refresh()
	r.bg.Refresh()
}

func (r *conversionStatsRenderer) Destroy() {}

func (r *conversionStatsRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.statusText, r.progressBar}
}

// Helper functions for formatting
func formatProgress(progress float64) string {
	return fmt.Sprintf("%.1f%%", progress)
}

func formatCount(count int, label string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", label)
	}
	return fmt.Sprintf("%d %s", count, label)
}
