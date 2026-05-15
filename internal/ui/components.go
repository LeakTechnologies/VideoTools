package ui

import (
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/fontutil"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
	tooltipwidget "github.com/dweymouth/fyne-tooltip/widget"
)

// ShowTooltips controls whether tooltips are displayed globally.
// Set this from main.go based on user preferences.
var ShowTooltips bool = true

// SetTooltip adds a tooltip to a widget if ShowTooltips is enabled
func SetTooltip(wid fyne.Widget, tooltip string) {
	if !ShowTooltips || tooltip == "" {
		return
	}
	var twe tooltipwidget.ToolTipWidgetExtend
	twe.ExtendToolTipWidget(wid)
	twe.SetToolTip(tooltip)
}

type monoFonts struct {
	regular    []byte
	italic     []byte
	bold       []byte
	boldItalic []byte
}

var monoFontData monoFonts
var aboriginalFontData monoFonts
var fontMode = "mono"
var vcrFontData []byte

// MonoFontPreference controls which monospace font is used ("ibm" or "vcr").
// Set via SetMonoFontPreference() and checked by MonoTheme.Font().
// When changed, call fyne.CurrentApp().Settings().SetTheme(theme) to refresh.
var MonoFontPreference = "ibm"

// FontSizePreference controls UI text size ("large" or "small").
var FontSizePreference = "large"

func SetMonoFontPreference(pref string) {
	MonoFontPreference = pref
}

func GetMonoFontPreference() string {
	return MonoFontPreference
}

func SetFontSizePreference(size string) {
	FontSizePreference = size
}

func GetFontSizePreference() string {
	return FontSizePreference
}

func SetMonoFontData(regular, italic, bold, boldItalic []byte) {
	monoFontData = monoFonts{
		regular:    regular,
		italic:     italic,
		bold:       bold,
		boldItalic: boldItalic,
	}
}

func SetVCRFontData(data []byte) {
	vcrFontData = data
}

func SetAboriginalFontData(regular, italic, bold, boldItalic []byte) {
	aboriginalFontData = monoFonts{
		regular:    regular,
		italic:     italic,
		bold:       bold,
		boldItalic: boldItalic,
	}
}

// SetFontMode switches between font modes ("mono" or "aboriginal").
// In "aboriginal" mode, Aboriginal Sans is registered as an auxiliary font so UCAS syllabics
// render correctly while IBM Plex Mono remains the primary for all Latin/ASCII text.
// Call this from the i18n language-change listener, followed by a theme refresh to flush caches.
func SetFontMode(mode string) {
	fontMode = mode
	if mode == "aboriginal" && aboriginalFontData.regular != nil {
		fontutil.SetAuxiliaryFont(fyne.NewStaticResource("AboriginalSans-Regular.ttf", aboriginalFontData.regular))
	} else {
		fontutil.SetAuxiliaryFont(nil)
	}
	logging.Debug(logging.CatUI, "SetFontMode: mode=%s auxiliary=%v", mode, mode == "aboriginal")
}

var (
	iconCache    = make(map[string]fyne.Resource)
	iconsEmbedFS fs.FS
)

// SetIconsFS initialises the embedded icon filesystem used by GetIcon.
func SetIconsFS(embedFS fs.FS) {
	iconsEmbedFS = embedFS
}

var (
	flagCache    = make(map[string]fyne.Resource)
	flagsEmbedFS fs.FS
)

// SetFlagsFS initialises the embedded flags filesystem used by GetFlag.
func SetFlagsFS(embedFS fs.FS) {
	flagsEmbedFS = embedFS
}

// GetFlag loads a flag SVG resource by filename (e.g. "FLAG_canada.svg").
// Returns nil if the filesystem is not initialised or the file is not found.
func GetFlag(filename string) fyne.Resource {
	if filename == "" {
		return nil
	}
	if cached, ok := flagCache[filename]; ok {
		return cached
	}
	if flagsEmbedFS == nil {
		logging.Info(logging.CatUI, "GetFlag: flagsEmbedFS is nil for %s", filename)
		return nil
	}
	data, err := fs.ReadFile(flagsEmbedFS, filename)
	if err != nil {
		logging.Info(logging.CatUI, "GetFlag: failed to read %s: %v", filename, err)
		return nil
	}
	res := fyne.NewStaticResource(filename, data)
	flagCache[filename] = res
	return res
}

func GetIcon(name string) fyne.Resource {
	if cached, ok := iconCache[name]; ok {
		return cached
	}

	if iconsEmbedFS == nil {
		logging.Info(logging.CatUI, "Icons FS not initialised")
		return theme.ErrorIcon()
	}

	entries, err := fs.ReadDir(iconsEmbedFS, ".")
	if err != nil {
		logging.Info(logging.CatUI, "Failed to read icons directory: %v", err)
		return theme.ErrorIcon()
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), name+"_") && strings.HasSuffix(entry.Name(), ".svg") {
			data, err := fs.ReadFile(iconsEmbedFS, entry.Name())
			if err != nil {
				logging.Info(logging.CatUI, "Failed to load icon %s: %v", name, err)
				return theme.ErrorIcon()
			}
			res := fyne.NewStaticResource(entry.Name(), data)
			iconCache[name] = res
			return res
		}
	}

	logging.Info(logging.CatUI, "Icon not found: %s", name)
	return theme.ErrorIcon()
}

var (
	// GridColor is the color used for grid lines and borders
	GridColor color.Color = BorderDim
	// TextColor is the main text color
	TextColor color.Color = Text
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
		return InputBg
	case theme.ColorNameInputBackground:
		return InputBg
	case theme.ColorNameInputBorder:
		return InputBg
	case theme.ColorNameFocus:
		return InputBg
	case theme.ColorNameForeground:
		return TextOnDark
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
	// VCR OSD Mono is used when MonoFontPreference is "vcr". It has no Bold/Italic variants.
	// IBM Plex Mono is used when preference is "ibm" and has variants.
	if MonoFontPreference == "vcr" && vcrFontData != nil {
		return fyne.NewStaticResource("VCR-OSD-mono.ttf", vcrFontData)
	}

	// IBM Plex Mono is the primary font. Aboriginal Sans is injected as an auxiliary
	// via SetFontMode so UCAS syllabics fall through to it only when Mono lacks the glyph.
	if monoFontData.regular != nil {
		var fontData []byte
		fontName := "IBMPlexMono-Regular.ttf"
		switch {
		case style.Bold && style.Italic:
			fontData = monoFontData.boldItalic
			fontName = "IBMPlexMono-BoldItalic.ttf"
		case style.Bold:
			fontData = monoFontData.bold
			fontName = "IBMPlexMono-Bold.ttf"
		case style.Italic:
			fontData = monoFontData.italic
			fontName = "IBMPlexMono-Italic.ttf"
		default:
			fontData = monoFontData.regular
		}
		if fontData != nil {
			return fyne.NewStaticResource(fontName, fontData)
		}
	}
	return theme.DefaultTheme().Font(style)
}

func (m *MonoTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MonoTheme) Size(name fyne.ThemeSizeName) float32 {
	isLarge := FontSizePreference != "small"
	padding := float32(8)
	innerPadding := float32(10)
	if !isLarge {
		padding = float32(6)
		innerPadding = float32(8)
	}
	switch name {
	case theme.SizeNamePadding:
		return padding
	case theme.SizeNameInnerPadding:
		return innerPadding
	case theme.SizeNameText:
		if isLarge {
			return 16
		}
		return 14
	case theme.SizeNameHeadingText:
		if isLarge {
			return 22
		}
		return 18
	case theme.SizeNameSubHeadingText:
		if isLarge {
			return 18
		}
		return 15
	case theme.SizeNameInputBorder:
		return 0
	}
	return theme.DefaultTheme().Size(name)
}

// ModuleTile is a clickable tile widget for module selection
type ModuleTile struct {
	widget.BaseWidget
	label               string
	color               color.Color
	textColor           color.Color
	enabled             bool
	missingDependencies bool
	onTapped            func()
	onDropped           func([]fyne.URI)
	flashing            bool
	draggedOver         bool
}

// NewModuleTile creates a new module tile
func NewModuleTile(label string, col color.Color, textCol color.Color, enabled bool, missingDeps bool, tapped func(), dropped func([]fyne.URI), tooltip string) *ModuleTile {
	m := &ModuleTile{
		label:               strings.ToUpper(label),
		color:               col,
		textColor:           textCol,
		missingDependencies: missingDeps,
		enabled:             enabled,
		onTapped:            tapped,
		onDropped:           dropped,
	}
	m.ExtendBaseWidget(m)

	// Set tooltip if provided
	if tooltip != "" {
		var twe tooltipwidget.ToolTipWidgetExtend
		twe.ExtendToolTipWidget(m)
		twe.SetToolTip(tooltip)
	}

	return m
}

// DraggedOver implements fyne.DropTarget
func (m *ModuleTile) DraggedOver(pos fyne.Position) {
	logging.Debug(logging.CatUI, "DraggedOver tile=%s enabled=%v pos=%v", m.label, m.enabled, pos)
	if m.enabled {
		m.draggedOver = true
		m.Refresh()
	}
}

// DraggedOut implements fyne.DropTarget
func (m *ModuleTile) DraggedOut() {
	logging.Debug(logging.CatUI, "DraggedOut tile=%s", m.label)
	m.draggedOver = false
	m.Refresh()
}

// Dropped implements fyne.DropTarget
func (m *ModuleTile) Dropped(pos fyne.Position, items []fyne.URI) {
	logging.Debug(logging.CatUI, "Dropped on tile=%s enabled=%v items=%v", m.label, m.enabled, items)
	m.draggedOver = false

	if m.enabled && m.onDropped != nil {
		logging.Debug(logging.CatUI, "Calling onDropped callback for %s", m.label)
		m.flashing = true
		m.Refresh()
		time.AfterFunc(300*time.Millisecond, func() {
			m.flashing = false
			m.Refresh()
		})
		m.onDropped(items)
	} else {
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
	// Use the same colour logic as Refresh() so the initial paint is consistent
	// with every subsequent repaint. Disabled/unavailable tiles show a 65%-dimmed
	// version of the module colour (preserving hue) rather than a fixed orange/grey.
	tileColor := m.color
	labelColor := m.textColor
	if labelColor == nil {
		labelColor = TextColor
	}
	if !m.enabled {
		if c, ok := m.color.(color.NRGBA); ok {
			tileColor = color.NRGBA{
				R: uint8(float32(c.R) * 0.65),
				G: uint8(float32(c.G) * 0.65),
				B: uint8(float32(c.B) * 0.65),
				A: c.A,
			}
		}
		labelColor = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
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
	lockIcon := widget.NewIcon(GetIcon("lock"))
	if m.enabled {
		lockIcon.Hide()
	}

	// Diagonal stripe overlay for disabled modules
	disabledStripe := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewRGBA(image.Rect(0, 0, w, h))

		// Only draw stripes if disabled
		if !m.enabled {
			// Semi-transparent dark stripes
			darkStripe := color.NRGBA{R: 0, G: 0, B: 0, A: 70}
			lightStripe := color.NRGBA{R: 0, G: 0, B: 0, A: 20}

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
	lockIcon       *widget.Icon
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
				R: uint8(float32(c.R) * 0.65),
				G: uint8(float32(c.G) * 0.65),
				B: uint8(float32(c.B) * 0.65),
				A: c.A,
			}
		}
		r.label.Color = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
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

// DarkTextButton creates a clickable text element with black text, suitable
// for use on bright backgrounds such as VT Green bars.
func DarkTextButton(label string, onTapped func()) fyne.CanvasObject {
	txt := canvas.NewText(label, color.Black)
	txt.TextStyle = fyne.TextStyle{Bold: true}
	return NewTappable(txt, onTapped)
}

// DarkTextLabel creates a canvas text label with black text, suitable for
// use on bright backgrounds such as VT Green bars.
func DarkTextLabel(label string) *canvas.Text {
	return canvas.NewText(label, color.Black)
}

// TintedBar creates a colored bar container
func TintedBar(col color.Color, body fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(col)
	rect.SetMinSize(fyne.NewSize(0, 48))
	padded := container.NewPadded(body)
	return container.NewMax(rect, padded)
}

// PillButton renders a pill-shaped button matching the roadmap's visual style:
// dark background, coloured border, centred text, hover lightens border, active inverts.
type PillButton struct {
	widget.DisableableWidget
	Label     string
	BorderCol color.Color
	OnTapped  func()
	hovered   bool
	Active    bool
}

func NewPillButton(label string, borderCol color.Color, onTapped func()) *PillButton {
	p := &PillButton{
		Label:     label,
		BorderCol: borderCol,
		OnTapped:  onTapped,
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *PillButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(nil)
	bg.CornerRadius = 12
	bg.StrokeWidth = 1.5
	txt := canvas.NewText(p.Label, nil)
	txt.Alignment = fyne.TextAlignCenter
	return &pillButtonRenderer{pill: p, bg: bg, txt: txt}
}

func (p *PillButton) MouseIn(*desktop.MouseEvent) {
	if p.Disabled() {
		return
	}
	p.hovered = true
	p.Refresh()
}

func (p *PillButton) MouseOut() {
	p.hovered = false
	p.Refresh()
}

func (p *PillButton) MouseMoved(*desktop.MouseEvent) {}

func (p *PillButton) SetText(label string) {
	p.Label = label
	p.Refresh()
}

func (p *PillButton) Tapped(*fyne.PointEvent) {
	if p.Disabled() {
		return
	}
	if p.OnTapped != nil {
		p.OnTapped()
	}
}

type pillButtonRenderer struct {
	pill *PillButton
	bg   *canvas.Rectangle
	txt  *canvas.Text
}

func (r *pillButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.txt.Resize(size)
}

func (r *pillButtonRenderer) MinSize() fyne.Size {
	return r.txt.MinSize().Add(fyne.NewSize(24, 12))
}

func (r *pillButtonRenderer) Refresh() {
	p := r.pill
	switch {
	case p.Disabled():
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
		r.txt.Color = TextMuted
	case p.Active:
		r.bg.FillColor = p.BorderCol
		r.bg.StrokeColor = p.BorderCol
		r.txt.Color = BgDark
	case p.hovered:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
		r.txt.Color = TextOnDark
	default:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = p.BorderCol
		r.txt.Color = TextOnDark
	}
	r.txt.Text = p.Label
	r.bg.Refresh()
	r.txt.Refresh()
}

func (r *pillButtonRenderer) Destroy() {}

func (r *pillButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.txt}
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
	content       fyne.CanvasObject
	onDropped     func([]fyne.URI)
	onDraggedOver func()
	onDraggedOut  func()
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

// SetOnDrag registers optional callbacks for drag-enter and drag-leave events.
func (d *Droppable) SetOnDrag(over func(), out func()) {
	d.onDraggedOver = over
	d.onDraggedOut = out
}

// CreateRenderer creates the renderer for the droppable
func (d *Droppable) CreateRenderer() fyne.WidgetRenderer {
	return &droppableRenderer{
		droppable: d,
		content:   d.content,
	}
}

// DraggedOver implements fyne.DropTarget
func (d *Droppable) DraggedOver(pos fyne.Position) {
	_ = pos
	if d.onDraggedOver != nil {
		d.onDraggedOver()
	}
}

// DraggedOut implements fyne.DropTarget
func (d *Droppable) DraggedOut() {
	if d.onDraggedOut != nil {
		d.onDraggedOut()
	}
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

// scrollClip is a minimal clipping widget that does NOT implement fyne.Draggable.
// This is intentional: container.Scroll implements fyne.Draggable but discards all
// drag events on desktop (mobile-only guard), causing them to be consumed silently
// before the outer FastVScroll can receive them. By using scrollClip instead, drag
// events propagate up to FastVScroll which handles them correctly.
//
// scrollClip DOES implement fyne.Scrollable (forwarding to parent FastVScroll) so
// that mouse-wheel events are captured even when the cursor is over child widgets
// like Entry or Select that have no internal scroll of their own.
type scrollClip struct {
	widget.BaseWidget
	content fyne.CanvasObject
	offsetY float32
	parent  *FastVScroll
}

func newScrollClip(content fyne.CanvasObject, parent *FastVScroll) *scrollClip {
	s := &scrollClip{content: content, parent: parent}
	s.ExtendBaseWidget(s)
	return s
}

// Scrolled forwards wheel events to the parent FastVScroll, ensuring that
// mouse-wheel scroll works even when the cursor is over a non-scrollable child
// (Entry, Select, Label, etc.).
func (s *scrollClip) Scrolled(ev *fyne.ScrollEvent) {
	if s.parent != nil {
		s.parent.Scrolled(ev)
	}
}

func (s *scrollClip) setOffset(y float32) {
	s.offsetY = y
	s.Refresh()
}

func (s *scrollClip) CreateRenderer() fyne.WidgetRenderer {
	return &scrollClipRenderer{clip: s}
}

type scrollClipRenderer struct {
	clip *scrollClip
}

func (r *scrollClipRenderer) Layout(size fyne.Size) {
	contentMin := r.clip.content.MinSize()
	r.clip.content.Resize(fyne.NewSize(size.Width, fyne.Max(contentMin.Height, size.Height)))
	r.clip.content.Move(fyne.NewPos(0, -r.clip.offsetY))
}

func (r *scrollClipRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *scrollClipRenderer) Refresh() {
	r.Layout(r.clip.Size())
	canvas.Refresh(r.clip.content)
}

func (r *scrollClipRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.clip.content}
}

func (r *scrollClipRenderer) Destroy() {}

// IsClip marks scrollClip as a GL scissor region so content is clipped to its bounds.
func (r *scrollClipRenderer) IsClip() {}

// FastVScroll creates a vertical scroll container with faster scroll speed.
// It supports mouse-wheel scrolling (Scrolled) and click-and-drag scrolling
// (MouseDown/Dragged/MouseUp).
type FastVScroll struct {
	widget.BaseWidget
	clip         *scrollClip
	dragging     bool
	dragStartY   float32 // canvas Y at drag start
	dragStartOff float32 // scroll offset at drag start
	forcedMinH   float32 // set via SetMinSize
}

// NewFastVScroll creates a new fast-scrolling vertical scroll container
func NewFastVScroll(content fyne.CanvasObject) *FastVScroll {
	f := &FastVScroll{}
	f.clip = newScrollClip(content, f)
	f.ExtendBaseWidget(f)
	return f
}

func (f *FastVScroll) CreateRenderer() fyne.WidgetRenderer {
	return &fastScrollRenderer{clip: f.clip}
}

func (f *FastVScroll) Scrolled(ev *fyne.ScrollEvent) {
	// Adaptive scroll speed based on viewport height for smoother feel across resolutions.
	height := f.Size().Height
	if height <= 0 {
		height = f.MinSize().Height
	}
	if height <= 0 {
		height = 480
	}
	scale := float32(1.2 + math.Min(float64(height)/500.0, 1.8)) // ~1.2x to ~3.0x
	f.ScrollBy(-ev.Scrolled.DY * scale)
}

// ScrollBy scrolls the content by a delta in pixels (positive = down).
func (f *FastVScroll) ScrollBy(delta float32) {
	if f == nil || f.clip == nil || f.clip.content == nil {
		return
	}
	content := f.clip.content
	max := content.Size().Height - f.Size().Height
	if max <= 0 {
		max = content.MinSize().Height - f.Size().Height
	}
	if max < 0 {
		max = 0
	}
	newY := f.clip.offsetY + delta
	if newY < 0 {
		newY = 0
	} else if newY > max {
		newY = max
	}
	f.clip.setOffset(newY)
}

// MouseDown records the drag anchor when the primary mouse button is pressed.
func (f *FastVScroll) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		f.dragging = true
		f.dragStartY = ev.Position.Y
		f.dragStartOff = f.clip.offsetY
	}
}

// MouseUp ends any in-progress drag.
func (f *FastVScroll) MouseUp(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		f.dragging = false
	}
}

// Dragged translates a drag delta into a scroll offset change so the user
// can click-and-drag the content like a mobile/touch interface.
func (f *FastVScroll) Dragged(ev *fyne.DragEvent) {
	if !f.dragging {
		return
	}
	delta := f.dragStartY - ev.Position.Y // positive = dragged upward = scroll down
	content := f.clip.content
	if content == nil {
		return
	}
	max := content.Size().Height - f.Size().Height
	if max <= 0 {
		max = content.MinSize().Height - f.Size().Height
	}
	if max < 0 {
		max = 0
	}
	newY := f.dragStartOff + delta
	if newY < 0 {
		newY = 0
	} else if newY > max {
		newY = max
	}
	f.clip.setOffset(newY)
}

// DragEnd satisfies fyne.Draggable.
func (f *FastVScroll) DragEnd() {
	f.dragging = false
}

// SetMinSize overrides the minimum height reported to Fyne's layout system.
// Use this to ensure the scroll container occupies a minimum amount of space.
func (f *FastVScroll) SetMinSize(size fyne.Size) {
	f.forcedMinH = size.Height
	f.Refresh()
}

// ScrollToTop scrolls the content to the top.
func (f *FastVScroll) ScrollToTop() {
	if f == nil || f.clip == nil {
		return
	}
	f.clip.setOffset(0)
}

// ScrollToBottom scrolls the content to the bottom.
func (f *FastVScroll) ScrollToBottom() {
	if f == nil || f.clip == nil || f.clip.content == nil {
		return
	}
	max := f.clip.content.Size().Height - f.Size().Height
	if max < 0 {
		max = 0
	}
	f.clip.setOffset(max)
}

// PageStep returns a reasonable scroll step based on the current viewport.
func (f *FastVScroll) PageStep() float32 {
	if f == nil || f.clip == nil {
		return 0
	}
	height := f.Size().Height
	if height <= 0 {
		height = f.MinSize().Height
	}
	if height <= 0 {
		height = 240
	}
	return height * 0.85
}

type fastScrollRenderer struct {
	clip *scrollClip
}

func (r *fastScrollRenderer) Layout(size fyne.Size) {
	r.clip.Resize(size)
}

func (r *fastScrollRenderer) MinSize() fyne.Size {
	if r.clip.parent != nil && r.clip.parent.forcedMinH > 0 {
		return fyne.NewSize(0, r.clip.parent.forcedMinH)
	}
	return fyne.NewSize(0, 0)
}

func (r *fastScrollRenderer) Refresh() {
	r.clip.Refresh()
}

func (r *fastScrollRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.clip}
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
	elapsed   string
	remaining string
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
func (c *ConversionStatsBar) UpdateStatsWithDetails(running, pending, completed, failed, cancelled int, progress, fps, speed float64, eta, elapsed, remaining, jobTitle string) {
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
		c.elapsed = elapsed
		c.remaining = remaining
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

		// Show elapsed and remaining time
		if r.bar.elapsed != "" {
			statusStr += " • " + r.bar.elapsed
		}
		if r.bar.remaining != "" {
			statusStr += " • " + r.bar.remaining
		}

		if r.bar.pending > 0 {
			statusStr += " • " + formatCount(r.bar.pending, "pending")
		}

		r.statusText.Text = statusStr
		r.statusText.Color = color.NRGBA{R: 100, G: 220, B: 100, A: 255} // Green

		// Update progress bar (show even at 0%)
		r.progressBar.SetValue(r.bar.progress / 100.0)
		r.progressBar.Show()
	} else if r.bar.pending > 0 {
		r.statusText.Text = formatCount(r.bar.pending, "queued")
		r.statusText.Color = color.NRGBA{R: 255, G: 200, B: 100, A: 255} // Yellow
		r.progressBar.Hide()
	} else if r.bar.completed > 0 || r.bar.failed > 0 || r.bar.cancelled > 0 {
		statusStr := ""
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
		r.statusText.Text = i18n.T().StatusNoActiveJobs
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
	label := NewSectionLabel(title)

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

// --- Text primitives ---

// NewTitleLabel creates a page/module title (Monospace, Bold, 24pt).
// Use for top-level view titles like "Convert", "Audio", "Settings".
func NewTitleLabel(text string, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.TextSize = 24
	return t
}

// NewSectionLabel creates a bold section heading.
// Use for grouping controls within a view (e.g. "Output Settings", "Source").
func NewSectionLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

// NewWrappingLabel creates body text with word wrapping enabled.
// Use for instructions, descriptions, and multi-line status messages.
func NewWrappingLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// NewHintLabel creates italic hint/secondary text.
// Use for explanations, context help, and non-critical information.
func NewHintLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Italic: true}
	return l
}

// NewMonoLabel creates monospace technical text.
// Use for file hashes, paths, FFmpeg output, and other machine-readable data.
func NewMonoLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Monospace: true}
	return l
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
	options         []string
	selected        string
	colorMap        map[string]color.Color
	onChanged       func(string)
	popup           *widget.PopUp
	window          fyne.Window
	placeHolder     string
	disabled        bool
	disabledOptions map[string]bool
	tooltip         string
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

// NewColoredSelectWithTooltip creates a new colored select widget with tooltip
func NewColoredSelectWithTooltip(options []string, colorMap map[string]color.Color, onChange func(string), window fyne.Window, tooltip string) *ColoredSelect {
	cs := NewColoredSelect(options, colorMap, onChange, window)
	if ShowTooltips && tooltip != "" {
		var twe tooltipwidget.ToolTipWidgetExtend
		twe.ExtendToolTipWidget(cs)
		twe.SetToolTip(tooltip)
	}
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

// DisableOption marks a specific option as disabled (greyed out but visible)
func (cs *ColoredSelect) DisableOption(option string) {
	if cs.disabledOptions == nil {
		cs.disabledOptions = make(map[string]bool)
	}
	cs.disabledOptions[option] = true
	cs.Refresh()
}

// EnableOption re-enables a previously disabled option
func (cs *ColoredSelect) EnableOption(option string) {
	if cs.disabledOptions != nil {
		delete(cs.disabledOptions, option)
	}
	cs.Refresh()
}

// EnableAllOptions re-enables all options
func (cs *ColoredSelect) EnableAllOptions() {
	cs.disabledOptions = nil
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

	caret := canvas.NewText("▾", selectTextColor())
	caret.TextSize = 14

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

		// Check if this option is disabled
		isDisabled := cs.disabledOptions != nil && cs.disabledOptions[opt]

		// Get color for this option
		itemColor := cs.colorMap[opt]
		if itemColor == nil {
			itemColor = color.NRGBA{R: 80, G: 80, B: 80, A: 255} // Default gray
		}
		// Grey out disabled options
		if isDisabled {
			itemColor = color.NRGBA{R: 100, G: 100, B: 100, A: 128}
		}

		// Create colored indicator bar
		colorBar := canvas.NewRectangle(itemColor)
		colorBar.SetMinSize(fyne.NewSize(4, 24))

		// Create label using canvas text for color control
		textColor := selectTextColor()
		if isDisabled {
			textColor = color.NRGBA{R: 150, G: 150, B: 150, A: 255}
		}
		label := canvas.NewText(opt, textColor)
		label.Alignment = fyne.TextAlignLeading
		label.TextSize = 16
		// Highlight if currently selected
		if opt == cs.selected {
			label.TextStyle = fyne.TextStyle{Bold: true}
		}

		// Create tappable item with proper padding
		itemContent := container.NewBorder(nil, nil, colorBar, nil,
			container.NewPadded(label)) // Single padding for precision

		if isDisabled {
			// Disabled options are not tappable - just add the label
			items[i] = itemContent
		} else {
			// Create tappable item with proper padding
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

// DraggableListItem allows simple drag up/down to reorder one slot at a time.
type DraggableListItem struct {
	widget.BaseWidget
	itemID    string
	content   fyne.CanvasObject
	onReorder func(string, int) // id, direction (-1 up, +1 down)
	accumY    float32
}

// NewDraggableListItem creates a new draggable list item for reorderable lists.
// The onReorder callback receives the item ID and direction (-1 for up, +1 for down).
func NewDraggableListItem(id string, content fyne.CanvasObject, onReorder func(string, int)) *DraggableListItem {
	d := &DraggableListItem{
		itemID:    id,
		content:   content,
		onReorder: onReorder,
	}
	d.ExtendBaseWidget(d)
	return d
}

func (d *DraggableListItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(d.content)
}

func (d *DraggableListItem) Dragged(ev *fyne.DragEvent) {
	d.accumY += ev.Dragged.DY
}

func (d *DraggableListItem) DragEnd() {
	const threshold float32 = 25
	if d.accumY <= -threshold {
		d.onReorder(d.itemID, -1)
	} else if d.accumY >= threshold {
		d.onReorder(d.itemID, 1)
	}
	d.accumY = 0
}

type TrimTimeline struct {
	widget.BaseWidget
	Duration         float64 // Total duration in seconds
	InPoint          float64 // In point in seconds (0 to Duration)
	OutPoint         float64 // Out point in seconds (0 to Duration)
	CurrentPos       float64 // Current playback position
	OnInPointChange  func(float64)
	OnOutPointChange func(float64)
	OnPositionChange func(float64)

	draggingIn  bool
	draggingOut bool
	draggingPos bool
	dragStartX  float32
	handleWidth float32
}

func NewTrimTimeline(duration float64) *TrimTimeline {
	t := &TrimTimeline{
		Duration:    duration,
		InPoint:     0,
		OutPoint:    duration,
		CurrentPos:  0,
		handleWidth: 16,
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *TrimTimeline) MinSize() fyne.Size {
	return fyne.NewSize(400, 60)
}

func (t *TrimTimeline) SetDuration(dur float64) {
	t.Duration = dur
	if t.OutPoint > dur {
		t.OutPoint = dur
	}
	if t.InPoint > t.OutPoint {
		t.InPoint = 0
	}
	t.Refresh()
}

func (t *TrimTimeline) SetInPoint(inPt float64) {
	if inPt < 0 {
		inPt = 0
	}
	if inPt > t.Duration {
		inPt = t.Duration
	}
	if inPt > t.OutPoint {
		inPt = t.OutPoint
	}
	t.InPoint = inPt
	if t.OnInPointChange != nil {
		t.OnInPointChange(inPt)
	}
	t.Refresh()
}

func (t *TrimTimeline) SetOutPoint(outPt float64) {
	if outPt < 0 {
		outPt = 0
	}
	if outPt > t.Duration {
		outPt = t.Duration
	}
	if outPt < t.InPoint {
		outPt = t.InPoint
	}
	t.OutPoint = outPt
	if t.OnOutPointChange != nil {
		t.OnOutPointChange(outPt)
	}
	t.Refresh()
}

func (t *TrimTimeline) SetPosition(pos float64) {
	if pos < 0 {
		pos = 0
	}
	if pos > t.Duration {
		pos = t.Duration
	}
	t.CurrentPos = pos
	t.Refresh()
}

func (t *TrimTimeline) CreateRenderer() fyne.WidgetRenderer {
	return &trimTimelineRenderer{timeline: t}
}

type trimTimelineRenderer struct {
	timeline *TrimTimeline
}

func (r *trimTimelineRenderer) Destroy() {}

func (r *trimTimelineRenderer) Layout(size fyne.Size) {
	// Handled by Refresh
}

func (r *trimTimelineRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 60)
}

func (r *trimTimelineRenderer) Refresh() {
	// Handled in objects
}

func (r *trimTimelineRenderer) Objects() []fyne.CanvasObject {
	t := r.timeline
	handleW := t.handleWidth
	barHeight := float32(40)
	handleHeight := float32(50)

	size := t.Size()
	if size.Width < 100 {
		size = fyne.NewSize(400, 60)
	}

	usableWidth := size.Width - (handleW * 2)
	if usableWidth < 50 {
		usableWidth = 50
	}

	// Calculate positions
	inX := handleW + (float32(t.InPoint/t.Duration) * usableWidth)
	outX := float32(t.OutPoint/t.Duration)*usableWidth + handleW - handleW

	// Background bar (full duration)
	bg := canvas.NewRectangle(utils.MustHex("#2A2F45"))
	bg.SetMinSize(fyne.NewSize(size.Width, barHeight))
	bg.Move(fyne.NewPos(0, (size.Height-barHeight)/2))

	// Selected region (between in and out points)
	selectedRect := canvas.NewRectangle(utils.MustHex("#4A90D9"))
	selectedWidth := outX - inX
	if selectedWidth < 0 {
		selectedWidth = 0
	}
	selectedRect.SetMinSize(fyne.NewSize(selectedWidth, barHeight-4))
	selectedRect.Move(fyne.NewPos(inX+2, (size.Height-barHeight)/2+2))

	// Left handle (in-point) - draggable
	inHandle := canvas.NewRectangle(utils.MustHex("#22C55E"))
	inHandle.SetMinSize(fyne.NewSize(handleW, handleHeight))
	inHandle.CornerRadius = 4
	// Position centered vertically, overlapping the bar
	inHandle.Move(fyne.NewPos(inX-handleW/2, (size.Height-handleHeight)/2))

	// Right handle (out-point) - draggable
	outHandle := canvas.NewRectangle(utils.MustHex("#EF4444"))
	outHandle.SetMinSize(fyne.NewSize(handleW, handleHeight))
	outHandle.CornerRadius = 4
	outHandle.Move(fyne.NewPos(inX+selectedWidth-handleW/2, (size.Height-handleHeight)/2))

	// Current position indicator (blue line)
	posX := handleW + (float32(t.CurrentPos/t.Duration) * usableWidth)
	posIndicator := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	posIndicator.SetMinSize(fyne.NewSize(2, handleHeight))
	posIndicator.Move(fyne.NewPos(posX-1, (size.Height-handleHeight)/2))

	// Time labels
	inLabel := canvas.NewText(formatTimelineTime(t.InPoint), color.White)
	inLabel.TextSize = 11
	inLabel.Move(fyne.NewPos(inX-handleW/2, 0))

	outLabel := canvas.NewText(formatTimelineTime(t.OutPoint), color.White)
	outLabel.TextSize = 11
	outLabel.Move(fyne.NewPos(inX+selectedWidth-handleW/2-30, 0))

	return []fyne.CanvasObject{bg, selectedRect, inHandle, outHandle, posIndicator, inLabel, outLabel}
}

func formatTimelineTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms)
	}
	return fmt.Sprintf("%d:%02d.%03d", m, s, ms)
}

func (t *TrimTimeline) Tapped(ev *fyne.PointEvent) {
	t.handleTap(ev.Position)
}

func (t *TrimTimeline) handleTap(pos fyne.Position) {
	size := t.Size()
	handleW := t.handleWidth
	usableWidth := size.Width - (handleW * 2)
	if usableWidth < 50 {
		usableWidth = 50
	}

	inX := handleW + (float32(t.InPoint/t.Duration) * usableWidth)
	outX := float32(t.OutPoint/t.Duration)*usableWidth + handleW - handleW

	// Check if tap is near in handle
	if pos.X >= inX-handleW && pos.X <= inX+handleW {
		t.SetInPoint(t.CurrentPos)
		return
	}
	// Check if tap is near out handle
	if pos.X >= outX-handleW && pos.X <= outX+handleW {
		t.SetOutPoint(t.CurrentPos)
		return
	}
	// Otherwise, set position to tap point
	newPos := float64((pos.X-handleW)/usableWidth) * t.Duration
	if newPos < 0 {
		newPos = 0
	}
	if newPos > t.Duration {
		newPos = t.Duration
	}
	t.SetPosition(newPos)
	if t.OnPositionChange != nil {
		t.OnPositionChange(newPos)
	}
}

func (t *TrimTimeline) Dragged(ev *fyne.DragEvent) {
	size := t.Size()
	handleW := t.handleWidth
	usableWidth := size.Width - (handleW * 2)
	if usableWidth < 50 {
		usableWidth = 50
	}

	newPos := float64((ev.Position.X-handleW)/usableWidth) * t.Duration
	if newPos < 0 {
		newPos = 0
	}
	if newPos > t.Duration {
		newPos = t.Duration
	}

	// Determine which handle is being dragged based on initial click
	// For simplicity, we'll use a state flag set on DragStart
	if t.draggingIn {
		if newPos > t.OutPoint {
			newPos = t.OutPoint
		}
		t.InPoint = newPos
		if t.OnInPointChange != nil {
			t.OnInPointChange(newPos)
		}
	} else if t.draggingOut {
		if newPos < t.InPoint {
			newPos = t.InPoint
		}
		t.OutPoint = newPos
		if t.OnOutPointChange != nil {
			t.OnOutPointChange(newPos)
		}
	} else if t.draggingPos {
		t.CurrentPos = newPos
		if t.OnPositionChange != nil {
			t.OnPositionChange(newPos)
		}
	}

	t.Refresh()
}

func (t *TrimTimeline) DragEnd() {
	t.draggingIn = false
	t.draggingOut = false
	t.draggingPos = false
}

// ToastSeverity controls the accent colour of a toast notification.
type ToastSeverity int

const (
	ToastInfo    ToastSeverity = iota // neutral blue-grey
	ToastWarning                      // amber
	ToastError                        // red
)

// ShowToast displays a non-blocking notification bar at the bottom of the
// window canvas. It auto-dismisses after ~3.5 seconds. Safe to call from
// any goroutine.
func ShowToast(win fyne.Window, message string, severity ToastSeverity) {
	if win == nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		showToastOnMain(win, message, severity)
	}, false)
}

func showToastOnMain(win fyne.Window, message string, severity ToastSeverity) {
	var bg color.Color
	switch severity {
	case ToastWarning:
		bg = color.NRGBA{R: 180, G: 110, B: 0, A: 230}
	case ToastError:
		bg = color.NRGBA{R: 160, G: 30, B: 30, A: 230}
	default:
		bg = color.NRGBA{R: 40, G: 50, B: 70, A: 230}
	}

	rect := canvas.NewRectangle(bg)
	rect.CornerRadius = 8

	lbl := canvas.NewText(message, color.White)
	lbl.TextSize = 13
	lbl.TextStyle = fyne.TextStyle{Monospace: true}
	lbl.Alignment = fyne.TextAlignCenter

	content := container.NewMax(rect, container.NewPadded(container.NewCenter(lbl)))

	pop := widget.NewPopUp(content, win.Canvas())

	// Size and position: full-width bar pinned 16px above the bottom edge.
	winSize := win.Canvas().Size()
	toastW := winSize.Width - 32
	if toastW < 200 {
		toastW = 200
	}
	toastH := float32(44)
	pop.Resize(fyne.NewSize(toastW, toastH))
	pop.Move(fyne.NewPos(16, winSize.Height-toastH-16))
	pop.Show()

	// Auto-dismiss.
	time.AfterFunc(3500*time.Millisecond, func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(pop.Hide, false)
	})
}

// BuildPlayerContainer is the canonical way to embed a player widget in any
// module view.  It wraps widget in a Max container backed by a consistently
// styled rectangle: dark fill (#0F1529), 8-px corner radius, 1-px GridColor
// border.  Pass fyne.NewSize(0, 0) when no explicit minimum size is needed.
//
// Callers that manage their own playback UI (inspect, convert) should call
// widget.(*media.VideoPlayer).DisableBuiltinControls() before or after this;
// modules that rely on the built-in control bar (trim, subtitles) should leave
// controls enabled.
func BuildPlayerContainer(widget fyne.CanvasObject, minSize fyne.Size) fyne.CanvasObject {
	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.CornerRadius = 8
	bg.StrokeColor = GridColor
	bg.StrokeWidth = 1
	if minSize.Width > 0 || minSize.Height > 0 {
		bg.SetMinSize(minSize)
	}
	return container.NewMax(bg, widget)
}
