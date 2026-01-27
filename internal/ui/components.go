package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
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

// MonoTheme ensures all text uses a monospace font and swaps hover/selection colors
type MonoTheme struct{}

func (m *MonoTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameSelection:
		// Use default hover color for selection
		return theme.DefaultTheme().Color(theme.ColorNameHover, variant)
	case theme.ColorNameHover:
		// Use default selection color for hover
		return theme.DefaultTheme().Color(theme.ColorNameSelection, variant)
	case theme.ColorNameButton:
		// Use a slightly lighter blue for buttons (92% of full selection color brightness)
		selectionColor := theme.DefaultTheme().Color(theme.ColorNameSelection, variant)
		r, g, b, a := selectionColor.RGBA()
		// Lighten by 8% (multiply by 1.08, capped at 255)
		lightness := 1.08
		newR := uint8(min(int(float64(r>>8)*lightness), 255))
		newG := uint8(min(int(float64(g>>8)*lightness), 255))
		newB := uint8(min(int(float64(b>>8)*lightness), 255))
		return color.RGBA{R: newR, G: newG, B: newB, A: uint8(a >> 8)}
	case theme.ColorNameBackground:
		// Match dropdown background tone for panels/inputs
		return utils.MustHex("#344256")
	case theme.ColorNameInputBackground:
		// Match dropdown background tone for input fields
		return utils.MustHex("#344256")
	case theme.ColorNameInputBorder:
		// Keep input borders visually flat against the background
		return utils.MustHex("#344256")
	case theme.ColorNameFocus:
		// Avoid bright focus outlines on dark input fields
		return utils.MustHex("#344256")
	case theme.ColorNameForeground:
		// Ensure good contrast on dark backgrounds
		return color.White
	}
	return theme.DefaultTheme().Color(name, variant)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *MonoTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Monospace = true
	return theme.DefaultTheme().Font(style)
}

func (m *MonoTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MonoTheme) Size(name fyne.ThemeSizeName) float32 {
	// Make UI elements larger and more readable
	switch name {
	case theme.SizeNamePadding:
		return 6 // Back to default for better precision
	case theme.SizeNameInnerPadding:
		return 8 // Back to default for better precision
	case theme.SizeNameText:
		return 14 // Slightly smaller for a less cramped UI
	case theme.SizeNameHeadingText:
		return 18 // Slightly smaller for a less cramped UI
	case theme.SizeNameSubHeadingText:
		return 15 // Slightly smaller for a less cramped UI
	case theme.SizeNameInputBorder:
		return 0 // Remove input borders for cleaner fields
	}
	return theme.DefaultTheme().Size(name)
}

// ModuleTile is a clickable tile widget for module selection
type ModuleTile struct {
	widget.BaseWidget
	label               string
	color               color.Color
	enabled             bool
	missingDependencies bool
	onTapped            func()
	onDropped           func([]fyne.URI)
	flashing            bool
	draggedOver         bool
}

// NewModuleTile creates a new module tile
func NewModuleTile(label string, col color.Color, enabled bool, missingDeps bool, tapped func(), dropped func([]fyne.URI)) *ModuleTile {
	m := &ModuleTile{
		label:               strings.ToUpper(label),
		color:               col,
		missingDependencies: missingDeps,
		enabled:             enabled,
		onTapped:            tapped,
		onDropped:           dropped,
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

// getContrastColor returns black or white text color based on background brightness
func getContrastColor(bgColor color.Color) color.Color {
	r, g, b, _ := bgColor.RGBA()
	// Convert from 16-bit to 8-bit
	r8 := float64(r >> 8)
	g8 := float64(g >> 8)
	b8 := float64(b >> 8)

	// Calculate relative luminance (WCAG formula)
	luminance := (0.2126*r8 + 0.7152*g8 + 0.0722*b8) / 255.0

	// If bright background, use dark text; if dark background, use light text
	if luminance > 0.5 {
		return color.NRGBA{R: 20, G: 20, B: 20, A: 255} // Dark text
	}
	return TextColor // Light text
}

func (m *ModuleTile) CreateRenderer() fyne.WidgetRenderer {
	tileColor := m.color
	labelColor := TextColor // White text for all modules

	// Orange background for modules missing dependencies
	if m.missingDependencies {
		tileColor = color.NRGBA{R: 255, G: 152, B: 0, A: 255} // Orange
	} else if !m.enabled {
		// Grey background for not implemented modules
		tileColor = color.NRGBA{R: 80, G: 80, B: 80, A: 255}
	}

	bg := canvas.NewRectangle(tileColor)
	bg.CornerRadius = 8
	bg.StrokeColor = GridColor
	bg.StrokeWidth = 1

	txt := canvas.NewText(m.label, labelColor)
	txt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	txt.Alignment = fyne.TextAlignCenter
	txt.TextSize = 28

	// Lock icon for disabled modules
	lockIcon := canvas.NewText("🔒", color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	lockIcon.TextSize = 16
	lockIcon.Alignment = fyne.TextAlignCenter
	if m.enabled {
		lockIcon.Hide()
	}

	// Diagonal stripe overlay for disabled modules
	disabledStripe := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, w, h))

		// Only draw stripes if disabled
		if !m.enabled {
			// Semi-transparent dark stripes
			darkStripe := color.NRGBA{R: 0, G: 0, B: 0, A: 100}
			lightStripe := color.NRGBA{R: 0, G: 0, B: 0, A: 30}

			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					// Thicker diagonal stripes (dividing by 8 instead of 4)
					if ((x + y) / 8 % 2) == 0 {
						img.Set(x, y, darkStripe)
					} else {
						img.Set(x, y, lightStripe)
					}
				}
			}
		}
		// Return transparent image for enabled modules
		return img
	})

	return &moduleTileRenderer{
		tile:           m,
		bg:             bg,
		label:          txt,
		lockIcon:       lockIcon,
		disabledStripe: disabledStripe,
	}
}

func (m *ModuleTile) Tapped(*fyne.PointEvent) {
	if m.enabled && m.onTapped != nil {
		m.onTapped()
	}
}

type moduleTileRenderer struct {
	tile           *ModuleTile
	bg             *canvas.Rectangle
	label          *canvas.Text
	lockIcon       *canvas.Text
	disabledStripe *canvas.Raster
}

func (r *moduleTileRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	// Stripe overlay covers entire tile
	if r.disabledStripe != nil {
		r.disabledStripe.Resize(size)
		r.disabledStripe.Move(fyne.NewPos(0, 0))
	}

	// Center the label by positioning it in the middle
	labelSize := r.label.MinSize()
	r.label.Resize(labelSize)
	x := (size.Width - labelSize.Width) / 2
	y := (size.Height - labelSize.Height) / 2
	r.label.Move(fyne.NewPos(x, y))

	// Position lock icon in top-right corner
	if r.lockIcon != nil {
		lockSize := r.lockIcon.MinSize()
		r.lockIcon.Resize(lockSize)
		lockX := size.Width - lockSize.Width - 4
		lockY := float32(4)
		r.lockIcon.Move(fyne.NewPos(lockX, lockY))
	}
}

func (r *moduleTileRenderer) MinSize() fyne.Size {
	return fyne.NewSize(170, 72)
}

func (r *moduleTileRenderer) Refresh() {
	// Update tile color and text color based on enabled state
	if r.tile.enabled {
		r.bg.FillColor = r.tile.color
		r.label.Color = TextColor // Always white text for enabled modules
		if r.lockIcon != nil {
			r.lockIcon.Hide()
		}
	} else {
		// Dim disabled tiles but preserve hue
		if c, ok := r.tile.color.(color.NRGBA); ok {
			r.bg.FillColor = color.NRGBA{
				R: uint8(float32(c.R) * 0.55),
				G: uint8(float32(c.G) * 0.55),
				B: uint8(float32(c.B) * 0.55),
				A: c.A,
			}
		}
		r.label.Color = color.NRGBA{R: 100, G: 100, B: 100, A: 255}
		if r.lockIcon != nil {
			r.lockIcon.Show()
		}
	}

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
	if r.lockIcon != nil {
		r.lockIcon.Refresh()
	}
	if r.disabledStripe != nil {
		r.disabledStripe.Refresh()
	}
}

func (r *moduleTileRenderer) Destroy() {}

func (r *moduleTileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.disabledStripe, r.label, r.lockIcon}
}

// TintedBar creates a colored bar container
func TintedBar(col color.Color, body fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(col)
	// rect.SetMinSize(fyne.NewSize(0, 48)) // Removed for flexible sizing
	padded := container.NewPadded(body)
	return container.NewMax(rect, padded)
}

// NewRatioRow lays out two objects with a fixed width ratio for the left item.
func NewRatioRow(left, right fyne.CanvasObject, leftRatio float32) *fyne.Container {
	return NewRatioRowWithGap(left, right, leftRatio, 0)
}

// NewRatioRowWithGap lays out two objects with a fixed width ratio and a gap between them.
func NewRatioRowWithGap(left, right fyne.CanvasObject, leftRatio float32, gap float32) *fyne.Container {
	return container.New(&ratioRowLayout{leftRatio: leftRatio, gap: gap}, left, right)
}

type ratioRowLayout struct {
	leftRatio float32
	gap       float32
}

func (r *ratioRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	ratio := clampRatio(r.leftRatio)
	gap := float32(0)
	if r.gap > 0 {
		gap = r.gap
	}
	availableWidth := size.Width - gap
	if availableWidth < 0 {
		availableWidth = 0
	}
	leftWidth := availableWidth * ratio
	rightWidth := availableWidth - leftWidth

	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(leftWidth, size.Height))
	objects[1].Move(fyne.NewPos(leftWidth+gap, 0))
	objects[1].Resize(fyne.NewSize(rightWidth, size.Height))
}

func (r *ratioRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	leftMin := objects[0].MinSize()
	rightMin := objects[1].MinSize()
	height := leftMin.Height
	if rightMin.Height > height {
		height = rightMin.Height
	}
	return fyne.NewSize(leftMin.Width+rightMin.Width, height)
}

func clampRatio(ratio float32) float32 {
	if ratio < 0.1 {
		return 0.1
	}
	if ratio > 0.9 {
		return 0.9
	}
	return ratio
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

// FastVScroll creates a vertical scroll container with faster scroll speed
type FastVScroll struct {
	widget.BaseWidget
	scroll *container.Scroll
}

// NewFastVScroll creates a new fast-scrolling vertical scroll container
func NewFastVScroll(content fyne.CanvasObject) *FastVScroll {
	f := &FastVScroll{
		scroll: container.NewVScroll(content),
	}
	f.scroll.SetMinSize(fyne.NewSize(0, 0))
	f.ExtendBaseWidget(f)
	return f
}

func (f *FastVScroll) CreateRenderer() fyne.WidgetRenderer {
	return &fastScrollRenderer{scroll: f.scroll}
}

func (f *FastVScroll) Scrolled(ev *fyne.ScrollEvent) {
	// Increase scroll speed moderately without overshooting content bounds.
	f.ScrollBy(ev.Scrolled.DY * 4.0)
}

// ScrollBy scrolls the content by a delta in pixels (positive = down).
func (f *FastVScroll) ScrollBy(delta float32) {
	if f == nil || f.scroll == nil || f.scroll.Content == nil {
		return
	}
	content := f.scroll.Content
	max := content.Size().Height - f.scroll.Size().Height
	if max <= 0 {
		max = content.MinSize().Height - f.scroll.Size().Height
	}
	if max < 0 {
		max = 0
	}
	newY := f.scroll.Offset.Y + delta
	if newY < 0 {
		newY = 0
	} else if newY > max {
		newY = max
	}
	f.scroll.ScrollToOffset(fyne.NewPos(f.scroll.Offset.X, newY))
}

// PageStep returns a reasonable scroll step based on the current viewport.
func (f *FastVScroll) PageStep() float32 {
	if f == nil || f.scroll == nil {
		return 0
	}
	height := f.scroll.Size().Height
	if height <= 0 {
		height = f.scroll.MinSize().Height
	}
	if height <= 0 {
		height = 240
	}
	return height * 0.85
}

type fastScrollRenderer struct {
	scroll *container.Scroll
}

func (r *fastScrollRenderer) Layout(size fyne.Size) {
	r.scroll.Resize(size)
}

func (r *fastScrollRenderer) MinSize() fyne.Size {
	return r.scroll.MinSize()
}

func (r *fastScrollRenderer) Refresh() {
	r.scroll.Refresh()
}

func (r *fastScrollRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.scroll}
}

func (r *fastScrollRenderer) Destroy() {}

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
	contentSize := d.content.Size()
	if contentSize.Height == 0 {
		contentSize = d.content.MinSize()
	}

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
	contentSize := d.content.Size()
	if contentSize.Height == 0 {
		contentSize = d.content.MinSize()
	}

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
	// Increase scroll speed modestly while clamping to content bounds.
	contentSize := d.content.Size()
	if contentSize.Height == 0 {
		contentSize = d.content.MinSize()
	}
	max := contentSize.Height - d.scroll.Size().Height
	if max < 0 {
		max = 0
	}
	newY := d.scroll.Offset.Y + (ev.Scrolled.DY * 2.0)
	if newY < 0 {
		newY = 0
	} else if newY > max {
		newY = max
	}
	d.scroll.ScrollToOffset(fyne.NewPos(d.scroll.Offset.X, newY))
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
	c.updateStats(func() {
		c.running = running
		c.pending = pending
		c.completed = completed
		c.failed = failed
		c.cancelled = cancelled
		c.progress = progress
		c.jobTitle = jobTitle
	})
}

// UpdateStatsWithDetails updates the stats display with detailed conversion info
func (c *ConversionStatsBar) UpdateStatsWithDetails(running, pending, completed, failed, cancelled int, progress, fps, speed float64, eta, jobTitle string) {
	c.updateStats(func() {
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
	})
}

func (c *ConversionStatsBar) updateStats(update func()) {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		update()
		c.Refresh()
		return
	}
	app.Driver().DoFromGoroutine(func() {
		update()
		c.Refresh()
	}, false)
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

	// Position progress bar on right side
	barWidth := float32(120)
	barHeight := float32(20)
	barX := size.Width - barWidth - padding
	barY := (size.Height - barHeight) / 2

	r.progressBar.Resize(fyne.NewSize(barWidth, barHeight))
	r.progressBar.Move(fyne.NewPos(barX, barY))

	// Position text on left
	r.statusText.Move(fyne.NewPos(padding, (size.Height-textSize.Height)/2))
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

// FFmpegCommandWidget displays an FFmpeg command with copy button
type FFmpegCommandWidget struct {
	widget.BaseWidget
	command      string
	commandLabel *widget.Label
	copyButton   *widget.Button
	window       fyne.Window
}

// NewFFmpegCommandWidget creates a new FFmpeg command display widget
func NewFFmpegCommandWidget(command string, window fyne.Window) *FFmpegCommandWidget {
	w := &FFmpegCommandWidget{
		command: command,
		window:  window,
	}
	w.ExtendBaseWidget(w)

	w.commandLabel = widget.NewLabel(command)
	w.commandLabel.Wrapping = fyne.TextWrapBreak
	w.commandLabel.TextStyle = fyne.TextStyle{Monospace: true}

	w.copyButton = widget.NewButton("Copy Command", func() {
		window.Clipboard().SetContent(w.command)
		dialog.ShowInformation("Copied", "FFmpeg command copied to clipboard", window)
	})
	w.copyButton.Importance = widget.LowImportance

	return w
}

// SetCommand updates the displayed command
func (w *FFmpegCommandWidget) SetCommand(command string) {
	w.command = command
	w.commandLabel.SetText(command)
	w.Refresh()
}

// CreateRenderer creates the widget renderer
func (w *FFmpegCommandWidget) CreateRenderer() fyne.WidgetRenderer {
	scroll := container.NewVScroll(w.commandLabel)
	// scroll.SetMinSize(fyne.NewSize(0, 80)) // Removed for flexible sizing

	content := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), w.copyButton),
		nil, nil,
		scroll,
	)

	return widget.NewSimpleRenderer(content)
}

// GetStatusColor returns the color for a job status
func GetStatusColor(status queue.JobStatus) color.Color {
	switch status {
	case queue.JobStatusCompleted:
		return utils.MustHex("#4CAF50") // Green
	case queue.JobStatusFailed:
		return utils.MustHex("#F44336") // Red
	case queue.JobStatusCancelled:
		return utils.MustHex("#FF9800") // Orange
	default:
		return utils.MustHex("#808080") // Gray
	}
}

// BuildModuleBadge creates a small colored badge for the job type
func BuildModuleBadge(jobType queue.JobType) fyne.CanvasObject {
	var badgeColor color.Color
	var badgeText string

	switch jobType {
	case queue.JobTypeConvert:
		badgeColor = utils.MustHex("#673AB7") // Deep Purple
		badgeText = "CONVERT"
	case queue.JobTypeMerge:
		badgeColor = utils.MustHex("#4CAF50") // Green
		badgeText = "MERGE"
	case queue.JobTypeTrim:
		badgeColor = utils.MustHex("#FFEB3B") // Yellow
		badgeText = "TRIM"
	case queue.JobTypeFilter:
		badgeColor = utils.MustHex("#00BCD4") // Cyan
		badgeText = "FILTER"
	case queue.JobTypeUpscale:
		badgeColor = utils.MustHex("#9C27B0") // Purple
		badgeText = "UPSCALE"
	case queue.JobTypeAudio:
		badgeColor = utils.MustHex("#FFC107") // Amber
		badgeText = "AUDIO"
	case queue.JobTypeThumbnail:
		badgeColor = utils.MustHex("#00ACC1") // Dark Cyan
		badgeText = "THUMBNAIL"
	case queue.JobTypeSnippet:
		badgeColor = utils.MustHex("#00BCD4") // Cyan (same as Convert)
		badgeText = "SNIPPET"
	case queue.JobTypeAuthor:
		badgeColor = utils.MustHex("#FF5722") // Deep Orange
		badgeText = "AUTHOR"
	case queue.JobTypeRip:
		badgeColor = utils.MustHex("#FF9800") // Orange
		badgeText = "RIP"
	default:
		badgeColor = utils.MustHex("#808080")
		badgeText = "OTHER"
	}

	rect := canvas.NewRectangle(badgeColor)
	rect.CornerRadius = 3
	rect.SetMinSize(fyne.NewSize(70, 20))

	text := canvas.NewText(badgeText, color.White)
	text.Alignment = fyne.TextAlignCenter
	text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	text.TextSize = 10

	return container.NewMax(rect, container.NewCenter(text))
}

// SectionHeader creates a color-coded section header for better visual separation
// Helps fix usability issue where settings sections blend together
func SectionHeader(title string, accentColor color.Color) fyne.CanvasObject {
	// Left accent bar (Memphis geometric style)
	accent := canvas.NewRectangle(accentColor)
	accent.SetMinSize(fyne.NewSize(4, 20))

	// Title text
	label := widget.NewLabel(title)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Importance = widget.HighImportance

	// Combine accent bar + title with padding
	content := container.NewBorder(
		nil, nil,
		accent,
		nil,
		container.NewPadded(label),
	)

	return content
}

// SectionSpacer creates vertical spacing between sections for better readability
func SectionSpacer() fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	// spacer.SetMinSize(fyne.NewSize(0, 12)) // Removed for flexible sizing
	return spacer
}

// ColoredDivider creates a thin horizontal divider with accent color
func ColoredDivider(accentColor color.Color) fyne.CanvasObject {
	divider := canvas.NewRectangle(accentColor)
	// divider.SetMinSize(fyne.NewSize(0, 2)) // Removed for flexible sizing
	return divider
}

// NewColorCodedSelectContainer wraps a Select widget with a colored left border
// The colored border visually indicates the category/type of the selection
// Returns a container with the border and a pointer to the border rectangle for color updates
func NewColorCodedSelectContainer(selectWidget *widget.Select, accentColor color.Color) (*fyne.Container, *canvas.Rectangle) {
	// Create colored left border rectangle
	border := canvas.NewRectangle(accentColor)
	// border.SetMinSize(fyne.NewSize(4, 44)) // Removed for flexible sizing

	// Return container with [ColoredBorder][Select] and the border for future updates
	container := container.NewBorder(nil, nil, border, nil, selectWidget)
	return container, border
}

// ColoredSelect is a custom select widget with color-coded dropdown items
type ColoredSelect struct {
	widget.BaseWidget
	options     []string
	selected    string
	colorMap    map[string]color.Color
	onChanged   func(string)
	popup       *widget.PopUp
	window      fyne.Window
	placeHolder string
	disabled    bool
}

// NewColoredSelect creates a new colored select widget
// colorMap should contain a color for each option
func NewColoredSelect(options []string, colorMap map[string]color.Color, onChange func(string), window fyne.Window) *ColoredSelect {
	cs := &ColoredSelect{
		options:   options,
		colorMap:  colorMap,
		onChanged: onChange,
		window:    window,
	}
	if len(options) > 0 {
		cs.selected = options[0]
	}
	cs.ExtendBaseWidget(cs)
	return cs
}

// SetPlaceHolder sets the placeholder text when nothing is selected
func (cs *ColoredSelect) SetPlaceHolder(text string) {
	cs.placeHolder = text
}

// SetSelected sets the currently selected option and triggers onChange callback if tapped by user
func (cs *ColoredSelect) SetSelected(option string) {
	cs.selected = option
	cs.Refresh()
}

// SetSelectedSilent sets the currently selected option WITHOUT triggering onChange callback
// Use this when synchronizing multiple widgets to avoid callback loops
func (cs *ColoredSelect) SetSelectedSilent(option string) {
	cs.selected = option
	cs.Refresh()
}

// UpdateOptions updates the available options and their colors
func (cs *ColoredSelect) UpdateOptions(options []string, colorMap map[string]color.Color) {
	cs.options = options
	cs.colorMap = colorMap
	// If current selection is not in new options, select first option
	found := false
	for _, opt := range options {
		if opt == cs.selected {
			found = true
			break
		}
	}
	if !found && len(options) > 0 {
		cs.selected = options[0]
	}
	cs.Refresh()
}

// Selected returns the currently selected option
func (cs *ColoredSelect) Selected() string {
	return cs.selected
}

// Enable enables the widget
func (cs *ColoredSelect) Enable() {
	cs.disabled = false
	cs.Refresh()
}

// Disable disables the widget
func (cs *ColoredSelect) Disable() {
	cs.disabled = true
	cs.Refresh()
}

// CreateRenderer creates the renderer for the colored select
func (cs *ColoredSelect) CreateRenderer() fyne.WidgetRenderer {
	displayText := cs.selected
	if displayText == "" && cs.placeHolder != "" {
		displayText = cs.placeHolder
	}

	bg := canvas.NewRectangle(selectBackgroundColor())
	bg.CornerRadius = 8
	bar := canvas.NewRectangle(selectAccentColor(cs.selected, cs.colorMap))
	bar.SetMinSize(fyne.NewSize(6, 28))
	bar.CornerRadius = 8
	bar.TopRightCornerRadius = 0
	bar.BottomRightCornerRadius = 0

	label := canvas.NewText(displayText, selectTextColor())
	label.Alignment = fyne.TextAlignLeading
	label.TextSize = 16

	caret := canvas.NewText("▼", selectTextColor())
	caret.TextSize = 12

	content := container.NewBorder(nil, nil, bar, nil,
		container.NewPadded(container.NewBorder(nil, nil, nil, caret, label)))

	bg.SetMinSize(fyne.NewSize(0, 36))

	tappable := NewTappable(container.NewMax(bg, content), func() {
		if !cs.disabled {
			cs.showPopup()
		}
	})

	return &coloredSelectRenderer{
		select_:  cs,
		bg:       bg,
		bar:      bar,
		label:    label,
		caret:    caret,
		tappable: tappable,
	}
}

// showPopup displays the dropdown list with colored items
func (cs *ColoredSelect) showPopup() {
	if cs.popup != nil {
		cs.popup.Hide()
		cs.popup = nil
		return
	}

	// Create list items with colors
	items := make([]fyne.CanvasObject, len(cs.options))
	for i, option := range cs.options {
		opt := option // Capture for closure

		// Get color for this option
		itemColor := cs.colorMap[opt]
		if itemColor == nil {
			itemColor = color.NRGBA{R: 80, G: 80, B: 80, A: 255} // Default gray
		}

		// Create colored indicator bar
		colorBar := canvas.NewRectangle(itemColor)
		colorBar.SetMinSize(fyne.NewSize(4, 24))

		// Create label
		label := widget.NewLabel(opt)

		// Highlight if currently selected
		if opt == cs.selected {
			label.TextStyle = fyne.TextStyle{Bold: true}
		}

		// Create tappable item with proper padding
		itemContent := container.NewBorder(nil, nil, colorBar, nil,
			container.NewPadded(label)) // Single padding for precision

		tappableItem := NewTappable(itemContent, func() {
			cs.selected = opt
			if cs.onChanged != nil {
				cs.onChanged(opt)
			}
			// Hide popup after a short delay to allow the selection to be processed
			time.AfterFunc(50*time.Millisecond, func() {
				fyne.Do(func() {
					if cs.popup != nil {
						cs.popup.Hide()
						cs.popup = nil
						cs.Refresh()
					}
				})
			})
		})

		items[i] = tappableItem
	}

	// Create scrollable list with proper spacing
	list := container.NewVBox(items...)
	scroll := container.NewVScroll(list)
	dropWidth := cs.Size().Width
	if dropWidth <= 0 {
		dropWidth = cs.MinSize().Width
	}
	if dropWidth < 200 {
		dropWidth = 200
	}
	visibleItems := min(len(cs.options), 6)
	popupHeight := float32(visibleItems) * 36
	if popupHeight < 144 {
		popupHeight = 144
	}
	scroll.SetMinSize(fyne.NewSize(dropWidth, popupHeight))

	// Create popup
	cs.popup = widget.NewPopUp(scroll, cs.window.Canvas())
	cs.popup.Resize(fyne.NewSize(dropWidth, popupHeight))

	// Position popup below the select widget
	popupPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(cs)
	popupPos.Y += cs.Size().Height
	cs.popup.ShowAtPosition(popupPos)
}

// Tapped implements the Tappable interface
func (cs *ColoredSelect) Tapped(*fyne.PointEvent) {
	if !cs.disabled {
		cs.showPopup()
	}
}

type coloredSelectRenderer struct {
	select_  *ColoredSelect
	bg       *canvas.Rectangle
	bar      *canvas.Rectangle
	label    *canvas.Text
	caret    *canvas.Text
	tappable *Tappable
}

func (r *coloredSelectRenderer) Layout(size fyne.Size) {
	r.tappable.Resize(size)
}

func (r *coloredSelectRenderer) MinSize() fyne.Size {
	return r.tappable.MinSize()
}

func (r *coloredSelectRenderer) Refresh() {
	displayText := r.select_.selected
	if displayText == "" && r.select_.placeHolder != "" {
		displayText = r.select_.placeHolder
	}

	if r.select_.disabled {
		r.bg.FillColor = color.NRGBA{R: 42, G: 46, B: 54, A: 255}
		r.label.Color = color.NRGBA{R: 140, G: 150, B: 160, A: 255}
		r.caret.Color = color.NRGBA{R: 140, G: 150, B: 160, A: 255}
	} else {
		r.bg.FillColor = selectBackgroundColor()
		r.label.Color = selectTextColor()
		r.caret.Color = selectTextColor()
	}

	r.bar.FillColor = selectAccentColor(r.select_.selected, r.select_.colorMap)
	r.label.Text = displayText

	r.bg.Refresh()
	r.bar.Refresh()
	r.label.Refresh()
	r.caret.Refresh()
	r.tappable.Refresh()
}

func (r *coloredSelectRenderer) Destroy() {}

func (r *coloredSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.tappable}
}

func selectBackgroundColor() color.Color {
	return color.NRGBA{R: 52, G: 66, B: 86, A: 255}
}

func selectTextColor() color.Color {
	return color.NRGBA{R: 230, G: 236, B: 245, A: 255}
}

func selectAccentColor(selected string, colorMap map[string]color.Color) color.Color {
	if selected == "" {
		return color.NRGBA{R: 90, G: 90, B: 90, A: 255}
	}
	if colorMap != nil {
		if col := colorMap[selected]; col != nil {
			return col
		}
	}
	return color.NRGBA{R: 90, G: 90, B: 90, A: 255}
}
