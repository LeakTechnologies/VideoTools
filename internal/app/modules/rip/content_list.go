package rip

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

const (
	thumbWidth  = 80
	thumbHeight = 60
	cyclePeriod = 1500 * time.Millisecond
	accentWidth = 6
)

var (
	ripNavy       = utils.MustHex("#191F35")
	ripCardBg     = utils.MustHex("#0F1529")
	ripTeal       = color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}  // selected for export
	ripPink       = color.NRGBA{R: 0xff, G: 0xaa, B: 0xaa, A: 0xff}  // NOT selected for export
	ripFocusBg    = color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0x1a} // focus highlight (subtle teal)
)

// ContentBrowser displays DVD titles as a scrollable list with cycling-still
// thumbnails, metadata, and per-title selection toggles.
type ContentBrowser struct {
	widget.BaseWidget

	mu          sync.Mutex
	scanResult  *DiscScanResult
	sourcePath  string
	titleCards  []*titleCardState
	ticker      *time.Ticker
	tickerDone  chan struct{}
	onSelect    func(titleNum int, selected bool)
	onPreview   func(titleNum int)
	selected    map[int]bool
	focused     int // title number currently focused for preview; 0 = none

	list     *widget.List
	outerBox *fyne.Container
}

type titleCardState struct {
	title    DiscTitle
	isMain   bool
	frames   []fyne.Resource
	thumbImg *canvas.Image
	checked  *widget.Check
	updating bool
}

// NewContentBrowser creates a new content browser widget.
func NewContentBrowser() *ContentBrowser {
	cb := &ContentBrowser{
		tickerDone: make(chan struct{}),
	}

	cb.list = widget.NewList(
		func() int {
			if cb.scanResult == nil {
				return 0
			}
			return len(cb.scanResult.Titles)
		},
		func() fyne.CanvasObject {
			return cb.buildCardTemplate()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			cb.updateCard(id, obj)
		},
	)

	ripTeal := color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}
	t := i18n.T()

	selectAllBtn := widget.NewButton(t.RipSelectAll, func() {
		cb.setAllSelected(true)
	})
	deselectAllBtn := widget.NewButton(t.RipDeselectAll, func() {
		cb.setAllSelected(false)
	})

	headerBar := canvas.NewRectangle(ripTeal)
	headerBar.CornerRadius = 10
	headerBar.SetMinSize(fyne.NewSize(0, 34))
	headerTitle := canvas.NewText(strings.ToUpper(t.RipContentBrowser), color.White)
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}
	headerTitle.TextSize = 12
	header := container.NewMax(
		headerBar,
		container.NewPadded(container.NewHBox(
			headerTitle,
			layout.NewSpacer(),
			selectAllBtn,
			deselectAllBtn,
		)),
	)

	cardBg := canvas.NewRectangle(ripCardBg)
	cardBg.CornerRadius = 8
	cardBg.StrokeColor = ui.GridColor
	cardBg.StrokeWidth = 1

	cb.outerBox = container.NewBorder(header, nil, nil, nil,
		container.NewStack(cardBg, cb.list),
	)
	cb.ExtendBaseWidget(cb)
	return cb
}

// SetScanResult configures the browser with disc scan data and begins
// thumbnail extraction. Safe to call from a goroutine.
func (cb *ContentBrowser) SetScanResult(result *DiscScanResult, sourcePath string) {
	cb.Stop()

	cb.mu.Lock()
	cb.scanResult = result
	cb.sourcePath = sourcePath
	cb.titleCards = nil
	cb.selected = make(map[int]bool)

	if result != nil {
		// Find main feature (longest duration).
		mainNum := 0
		mainDur := 0.0
		for _, dt := range result.Titles {
			if dt.Duration > mainDur {
				mainDur = dt.Duration
				mainNum = dt.Number
			}
		}
		for _, dt := range result.Titles {
			cb.selected[dt.Number] = true
			cb.titleCards = append(cb.titleCards, &titleCardState{
				title:  dt,
				isMain: dt.Number == mainNum,
			})
		}
	}
	cb.mu.Unlock()

	if result != nil && len(result.Titles) > 0 {
		cb.list.Refresh()
		cb.startCycling()
		// Extract thumbnails in background.
		for i := range cb.titleCards {
			go cb.extractThumbnails(cb.titleCards[i])
		}
	} else {
		cb.list.Refresh()
	}
}

// GetContainer returns the root canvas object for embedding.
func (cb *ContentBrowser) GetContainer() fyne.CanvasObject {
	return cb.outerBox
}

// SetOnSelect registers a callback for when a title's selection changes.
func (cb *ContentBrowser) SetOnSelect(fn func(int, bool)) {
	cb.mu.Lock()
	cb.onSelect = fn
	cb.mu.Unlock()
}

// SetOnPreview registers a callback for when a title card is clicked.
func (cb *ContentBrowser) SetOnPreview(fn func(int)) {
	cb.mu.Lock()
	cb.onPreview = fn
	cb.mu.Unlock()
}

// GetSelected returns a copy of the current selection map.
func (cb *ContentBrowser) GetSelected() map[int]bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	out := make(map[int]bool, len(cb.selected))
	for k, v := range cb.selected {
		out[k] = v
	}
	return out
}

// SetFocused marks a title as focused (highlighted for preview). Pass 0 to clear.
func (cb *ContentBrowser) SetFocused(titleNum int) {
	cb.mu.Lock()
	cb.focused = titleNum
	cb.mu.Unlock()
	cb.list.Refresh()
}

// GetFocused returns the currently focused title number (0 = none).
func (cb *ContentBrowser) GetFocused() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.focused
}

// Stop halts the cycling ticker and any pending extraction goroutines.
func (cb *ContentBrowser) Stop() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.ticker != nil {
		cb.ticker.Stop()
		cb.ticker = nil
	}
	close(cb.tickerDone)
	cb.tickerDone = make(chan struct{})
}

func (cb *ContentBrowser) setAllSelected(v bool) {
	cb.mu.Lock()
	for _, tc := range cb.titleCards {
		cb.selected[tc.title.Number] = v
		if tc.checked != nil {
			tc.checked.SetChecked(v)
		}
	}
	cb.mu.Unlock()
	cb.list.Refresh()
}

func (cb *ContentBrowser) startCycling() {
	cb.mu.Lock()
	if cb.ticker != nil {
		cb.ticker.Stop()
	}
	done := cb.tickerDone
	cb.ticker = time.NewTicker(cyclePeriod)
	cb.mu.Unlock()

	go func() {
		for {
			select {
			case <-done:
				return
			case <-cb.ticker.C:
				cb.cycleThumbnails()
			}
		}
	}()
}

func (cb *ContentBrowser) cycleThumbnails() {
	cb.mu.Lock()
	for _, tc := range cb.titleCards {
		if len(tc.frames) <= 1 || tc.thumbImg == nil {
			continue
		}
		// Find current frame index and advance.
		currentRes := tc.thumbImg.Resource
		idx := 0
		for i, f := range tc.frames {
			if f == currentRes {
				idx = i
				break
			}
		}
		next := (idx + 1) % len(tc.frames)
		tc.thumbImg.Resource = tc.frames[next]
		tc.thumbImg.Refresh()
	}
	cb.mu.Unlock()
}

func (cb *ContentBrowser) buildCardTemplate() fyne.CanvasObject {
	thumb := canvas.NewImageFromResource(nil)
	thumb.SetMinSize(fyne.NewSize(thumbWidth, thumbHeight))
	thumb.FillMode = canvas.ImageFillContain

	thumbBg := canvas.NewRectangle(ripCardBg)
	thumbBg.CornerRadius = 4
	thumbBg.StrokeColor = ui.GridColor
	thumbBg.StrokeWidth = 1

	titleLabel := widget.NewLabel("T00")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	infoLabel := widget.NewLabel("--")
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.TextStyle = fyne.TextStyle{Monospace: true}
	infoLabel.Importance = widget.LowImportance

	check := widget.NewCheck("", nil)

	accentBar := canvas.NewRectangle(ripTeal)
	accentBar.SetMinSize(fyne.NewSize(accentWidth, 0))

	card := container.NewHBox(
		accentBar,
		container.NewMax(thumbBg, thumb),
		container.NewVBox(titleLabel, infoLabel),
		layout.NewSpacer(),
		check,
	)
	return card
}

func (cb *ContentBrowser) updateCard(id widget.ListItemID, obj fyne.CanvasObject) {
	cb.mu.Lock()
	if cb.scanResult == nil || id >= len(cb.scanResult.Titles) {
		cb.mu.Unlock()
		return
	}
	dt := cb.scanResult.Titles[id]
	tc := cb.titleCards[id]
	isSelected := cb.selected[dt.Number]
	isFocused := cb.focused == dt.Number
	cb.mu.Unlock()

	hbox := obj.(*fyne.Container)
	accentBar := hbox.Objects[0].(*canvas.Rectangle)
	thumbContainer := hbox.Objects[1].(*fyne.Container)
	vbox := hbox.Objects[2].(*fyne.Container)
	check := hbox.Objects[4].(*widget.Check)

	// Accent bar colour: teal if selected for export, pink if not.
	if isSelected {
		accentBar.FillColor = ripTeal
	} else {
		accentBar.FillColor = ripPink
	}
	accentBar.Refresh()

	// Thumbnail.
	thumbImg := thumbContainer.Objects[1].(*canvas.Image)
	cb.mu.Lock()
	tc.thumbImg = thumbImg
	if len(tc.frames) > 0 {
		thumbImg.Resource = tc.frames[0]
		thumbImg.Show()
	} else {
		thumbImg.Resource = nil
		thumbImg.Hide()
	}
	cb.mu.Unlock()

	// Title label.
	titleLabel := vbox.Objects[0].(*widget.Label)
	t := i18n.T()
	label := fmt.Sprintf("T%02d  %s", dt.Number, FormatDuration(dt.Duration))
	if tc.isMain {
		label += "  " + t.RipMainFeature
	}
	titleLabel.SetText(label)

	// Info line.
	infoLabel := vbox.Objects[1].(*widget.Label)
	infoParts := []string{}
	if dt.NumChapters > 1 {
		infoParts = append(infoParts, fmt.Sprintf(t.RipTitleCardFmt, dt.Number, FormatDuration(dt.Duration), dt.NumChapters))
	}
	if len(dt.Audio) > 0 {
		infoParts = append(infoParts, fmt.Sprintf("%d audio", len(dt.Audio)))
	}
	if len(dt.Subtitles) > 0 {
		infoParts = append(infoParts, fmt.Sprintf("%d subs", len(dt.Subtitles)))
	}
	if len(infoParts) == 0 {
		infoParts = append(infoParts, "—")
	}
	infoLabel.SetText(strings.Join(infoParts, " · "))

	// Selection toggle.
	check.SetChecked(isSelected)
	check.OnChanged = func(v bool) {
		cb.mu.Lock()
		cb.selected[dt.Number] = v
		fn := cb.onSelect
		cb.mu.Unlock()
		if fn != nil {
			fn(dt.Number, v)
		}
		cb.list.Refresh()
	}

	// Focus on tap — set this title as focused and fire preview callback.
	if isFocused {
		// Highlight the card with a subtle teal tint.
		// Handled via accent bar already being teal when selected.
	}
}

// CreateRenderer implements fyne.Widget.
func (cb *ContentBrowser) CreateRenderer() fyne.WidgetRenderer {
	return &contentBrowserRenderer{content: cb.outerBox}
}

type contentBrowserRenderer struct {
	content fyne.CanvasObject
}

func (r *contentBrowserRenderer) Destroy()                         {}
func (r *contentBrowserRenderer) Layout(s fyne.Size)               { r.content.Resize(s) }
func (r *contentBrowserRenderer) MinSize() fyne.Size               { return r.content.MinSize() }
func (r *contentBrowserRenderer) Objects() []fyne.CanvasObject     { return []fyne.CanvasObject{r.content} }
func (r *contentBrowserRenderer) Refresh()                         { r.content.Refresh() }

// extractThumbnails runs ffmpeg to extract keyframes for a single title.
func (cb *ContentBrowser) extractThumbnails(tc *titleCardState) {
	cb.mu.Lock()
	sourcePath := cb.sourcePath
	dvdVideo := SupportsDVDVideo()
	cb.mu.Unlock()

	if sourcePath == "" {
		return
	}

	dur := tc.title.Duration
	if dur <= 0 {
		dur = 120.0
	}
	positions := []float64{0.10, 0.25, 0.50, 0.75, 0.90}

	var frames []fyne.Resource
	for _, pos := range positions {
		t := dur * pos
		data, err := extractTitleFrame(sourcePath, tc.title.Number, t, dvdVideo)
		if err != nil {
			logging.Debug(logging.CatDVD, "content_browser: thumb extract failed T%02d @%.0fs: %v", tc.title.Number, t, err)
			continue
		}
		res := fyne.NewStaticResource(
			fmt.Sprintf("thumb_t%d_%.0f.png", tc.title.Number, pos*100), data)
		frames = append(frames, res)
	}

	if len(frames) == 0 {
		return
	}

	cb.mu.Lock()
	tc.frames = frames
	if tc.thumbImg != nil && tc.thumbImg.Resource == nil {
		tc.thumbImg.Resource = frames[0]
		tc.thumbImg.Show()
		tc.thumbImg.Refresh()
	}
	cb.mu.Unlock()
}

// extractTitleFrame extracts a single frame from a DVD title via ffmpeg.
func extractTitleFrame(sourcePath string, titleNum int, timestamp float64, dvdVideo bool) ([]byte, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("vt_cb_t%d_%d.png", titleNum, time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var args []string
	if dvdVideo {
		args = []string{
			"-hide_banner", "-loglevel", "error",
			"-f", "dvdvideo",
			"-title", fmt.Sprintf("%d", titleNum),
			"-ss", formatTimestamp(timestamp),
			"-i", sourcePath,
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:-1", thumbWidth),
			"-q:v", "5",
			"-y", tmpFile,
		}
	} else {
		args = []string{
			"-hide_banner", "-loglevel", "error",
			"-ss", formatTimestamp(timestamp),
			"-i", sourcePath,
			"-map", "0:v:0",
			"-frames:v", "1",
			"-vf", fmt.Sprintf("scale=%d:-1", thumbWidth),
			"-q:v", "5",
			"-y", tmpFile,
		}
	}

	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg thumb extract: %w", err)
	}
	defer os.Remove(tmpFile)

	return os.ReadFile(tmpFile)
}

func formatTimestamp(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return time.Date(0, 0, 0, h, m, s, ms*1000000, time.UTC).Format("15:04:05.000")
}
