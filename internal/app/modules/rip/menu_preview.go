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

const menuPreviewWidth = 320

// MenuPreview displays a static capture of the DVD menu with toggle overlays
// for Preserve Menus and Main Feature options.
type MenuPreview struct {
	widget.BaseWidget

	mu          sync.Mutex
	sourcePath  string
	menuFrame   []fyne.Resource
	menuImg     *canvas.Image
	loadingLbl  *widget.Label
	placeholder *canvas.Text
	outerBox    *fyne.Container

	onTogglePreserve func(preserve bool)
	onToggleMain     func(main bool)
}

// NewMenuPreview creates a new menu preview widget.
func NewMenuPreview() *MenuPreview {
	mp := &MenuPreview{}

	ripNavy := utils.MustHex("#191F35")
	ripTeal := color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}

	mp.menuImg = canvas.NewImageFromResource(nil)
	mp.menuImg.FillMode = canvas.ImageFillContain
	mp.menuImg.SetMinSize(fyne.NewSize(menuPreviewWidth, float32(float64(menuPreviewWidth)*9.0/16.0)))
	mp.menuImg.Hide()

	mp.loadingLbl = widget.NewLabel("Loading menu...")
	mp.loadingLbl.Alignment = fyne.TextAlignCenter
	mp.loadingLbl.Hide()

	mp.placeholder = canvas.NewText("NO MENU FOUND", color.NRGBA{R: 80, G: 80, B: 80, A: 255})
	mp.placeholder.TextStyle = fyne.TextStyle{Monospace: true}
	mp.placeholder.Alignment = fyne.TextAlignCenter

	previewBg := canvas.NewRectangle(ripCardBg)
	previewBg.CornerRadius = 8
	previewBg.StrokeColor = ui.GridColor
	previewBg.StrokeWidth = 1

	previewArea := container.NewMax(previewBg, mp.placeholder, mp.loadingLbl, mp.menuImg)

	t := i18n.T()
	preserveCheck := widget.NewCheck(t.RipPreserveMenus, func(v bool) {
		if mp.onTogglePreserve != nil {
			mp.onTogglePreserve(v)
		}
	})
	mainCheck := widget.NewCheck(t.RipMainFeature, func(v bool) {
		if mp.onToggleMain != nil {
			mp.onToggleMain(v)
		}
	})

	headerBg := canvas.NewRectangle(ripTeal)
	headerBg.CornerRadius = 10
	headerBg.SetMinSize(fyne.NewSize(0, 34))
	headerTitle := canvas.NewText(strings.ToUpper(t.RipMenuPreview), color.White)
	headerTitle.TextStyle = fyne.TextStyle{Bold: true}
	headerTitle.TextSize = 12
	header := container.NewMax(
		headerBg,
		container.NewPadded(container.NewHBox(
			headerTitle,
			layout.NewSpacer(),
		)),
	)

	body := container.NewVBox(
		container.NewPadded(previewArea),
		container.NewPadded(container.NewVBox(preserveCheck, mainCheck)),
	)

	innerBg := canvas.NewRectangle(ripNavy)
	innerBg.CornerRadius = 10
	innerBg.StrokeColor = ui.GridColor
	innerBg.StrokeWidth = 1

	mp.outerBox = container.NewMax(
		ui.NoisyBackgroundObjects(innerBg)...,
	)
	mp.outerBox.Add(container.NewBorder(header, nil, nil, nil, body))

	mp.ExtendBaseWidget(mp)
	return mp
}

// CreateRenderer implements fyne.Widget.
func (mp *MenuPreview) CreateRenderer() fyne.WidgetRenderer {
	return &menuPreviewRenderer{content: mp.outerBox}
}

type menuPreviewRenderer struct {
	content fyne.CanvasObject
}

func (r *menuPreviewRenderer) Destroy()                     {}
func (r *menuPreviewRenderer) Layout(s fyne.Size)           { r.content.Resize(s) }
func (r *menuPreviewRenderer) MinSize() fyne.Size           { return r.content.MinSize() }
func (r *menuPreviewRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.content} }
func (r *menuPreviewRenderer) Refresh()                     { r.content.Refresh() }

// SetSourcePath sets the disc source for menu frame extraction.
func (mp *MenuPreview) SetSourcePath(path string) {
	mp.mu.Lock()
	mp.sourcePath = path
	mp.mu.Unlock()

	if path == "" {
		mp.showPlaceholder()
		return
	}
	mp.extractMenuFrame()
}

// SetOnTogglePreserve registers a callback for the Preserve Menus toggle.
func (mp *MenuPreview) SetOnTogglePreserve(fn func(bool)) {
	mp.mu.Lock()
	mp.onTogglePreserve = fn
	mp.mu.Unlock()
}

// SetOnToggleMain registers a callback for the Main Feature toggle.
func (mp *MenuPreview) SetOnToggleMain(fn func(bool)) {
	mp.mu.Lock()
	mp.onToggleMain = fn
	mp.mu.Unlock()
}

// SetPreserveMenus sets the Preserve Menus checkbox state without firing callback.
func (mp *MenuPreview) SetPreserveMenus(v bool) {
	// Handled by the caller via rebuildEnrich.
}

// GetContainer returns the root canvas object for embedding.
func (mp *MenuPreview) GetContainer() fyne.CanvasObject {
	return mp.outerBox
}

func (mp *MenuPreview) showPlaceholder() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.menuImg.Hide()
	mp.loadingLbl.Hide()
	mp.placeholder.Show()
}

func (mp *MenuPreview) extractMenuFrame() {
	mp.mu.Lock()
	path := mp.sourcePath
	mp.mu.Unlock()

	if path == "" {
		return
	}

	mp.mu.Lock()
	mp.placeholder.Hide()
	mp.loadingLbl.Show()
	mp.menuImg.Hide()
	mp.mu.Unlock()

	go func() {
		lower := filepath.Ext(path)
		var args []string

		if len(lower) > 0 {
			// ISO — try to read the first menu VOB via dvdvideo demuxer.
			args = []string{
				"-hide_banner", "-loglevel", "error",
				"-f", "dvdvideo",
				"-title", "0",
				"-ss", "00:00:01",
				"-i", path,
				"-frames:v", "1",
				"-vf", fmt.Sprintf("scale=%d:-1", menuPreviewWidth),
				"-q:v", "3",
				"-y", filepath.Join(os.TempDir(), "vt_menu_preview.png"),
			}
		} else {
			// VIDEO_TS directory — find menu VOB directly.
			args = []string{
				"-hide_banner", "-loglevel", "error",
				"-ss", "00:00:01",
				"-i", filepath.Join(path, "VIDEO_TS", "VTS_01_0.VOB"),
				"-frames:v", "1",
				"-vf", fmt.Sprintf("scale=%d:-1", menuPreviewWidth),
				"-q:v", "3",
				"-y", filepath.Join(os.TempDir(), "vt_menu_preview.png"),
			}
		}

		tmpFile := args[len(args)-1]
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
		if err := cmd.Run(); err != nil {
			logging.Debug(logging.CatDVD, "menu_preview: frame extract failed: %v", err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				mp.showPlaceholder()
			}, false)
			return
		}
		defer os.Remove(tmpFile)

		data, err := os.ReadFile(tmpFile)
		if err != nil {
			logging.Debug(logging.CatDVD, "menu_preview: read frame failed: %v", err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				mp.showPlaceholder()
			}, false)
			return
		}

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			mp.mu.Lock()
			mp.menuImg.Resource = fyne.NewStaticResource("dvd_menu_frame.png", data)
			mp.menuImg.Show()
			mp.loadingLbl.Hide()
			mp.placeholder.Hide()
			mp.menuImg.Refresh()
			mp.mu.Unlock()
		}, false)
	}()
}
