package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// ---- lazy font loading ----

var (
	pvFontOnce sync.Once
	pvFont     *opentype.Font
)

func loadPreviewFont() *opentype.Font {
	pvFontOnce.Do(func() {
		p := findMenuFontPath()
		if p == "" {
			return
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return
		}
		f, err := opentype.Parse(data)
		if err != nil {
			return
		}
		pvFont = f
	})
	return pvFont
}

// ---- color helpers ----

// menuColorFromHex parses 0xRRGGBB or #RRGGBB into color.RGBA.
func menuColorFromHex(s string) color.RGBA {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	s = strings.TrimPrefix(s, "#")
	if len(s) < 6 {
		return color.RGBA{A: 255}
	}
	r, _ := strconv.ParseUint(s[0:2], 16, 8)
	g, _ := strconv.ParseUint(s[2:4], 16, 8)
	b, _ := strconv.ParseUint(s[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func dimColor(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
		A: 255,
	}
}

// ---- drawing primitives ----

func pvFillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func pvBlendRect(img *image.RGBA, r image.Rectangle, c color.RGBA, alpha float64) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			src := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(float64(src.R)*(1-alpha) + float64(c.R)*alpha),
				G: uint8(float64(src.G)*(1-alpha) + float64(c.G)*alpha),
				B: uint8(float64(src.B)*(1-alpha) + float64(c.B)*alpha),
				A: 255,
			})
		}
	}
}

func pvDrawText(img *image.RGBA, text string, size float64, x, y int, col color.RGBA) {
	f := loadPreviewFont()
	if f == nil {
		return
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return
	}
	d := &xfont.Drawer{
		Dst:  img,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func pvMeasureText(text string, size float64) int {
	f := loadPreviewFont()
	if f == nil {
		return int(size*0.6) * len(text)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72})
	if err != nil {
		return int(size*0.6) * len(text)
	}
	d := &xfont.Drawer{Face: face}
	return d.MeasureString(text).Ceil()
}

// ---- image scaling ----

func pvScaleImage(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(sb.Min.X+x*sw/w, sb.Min.Y+y*sh/h))
		}
	}
	return dst
}

// ---- theme resolution ----

// resolvePreviewTheme builds a MenuTheme from current app state, safe to call from the UI goroutine.
func resolvePreviewTheme(state *appState) *MenuTheme {
	convertColor := func(s string) string {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "#") {
			return "0x" + strings.TrimPrefix(s, "#")
		}
		return s
	}
	t := &MenuTheme{
		Name:            state.authorMenuTheme,
		BackgroundColor: convertColor(state.authorMenuCustomBgColor),
		TextColor:       convertColor(state.authorMenuCustomTextColor),
		AccentColor:     convertColor(state.authorMenuCustomAccentColor),
		IsCustom:        state.authorMenuTheme == "Custom",
		FontPath:        findMenuFontPath(),
	}
	return resolveMenuTheme(t)
}

// ---- preview rendering params (goroutine-safe snapshot) ----

type previewParams struct {
	theme       *MenuTheme
	template    string
	title       string
	region      string
	bgImagePath string
	buttons     []dvdMenuButton
	highlighted int // -1 = none
}

// ---- template renderers ----

func pvRenderSimple(img *image.RGBA, title string, buttons []dvdMenuButton, txt, hdr, accent, dim color.RGBA, w int, t i18n.Strings) {
	pvFillRect(img, image.Rect(0, 0, w, 72), hdr)
	pvDrawText(img, t.AuthorVideoToolsDVD, 22, 36, 50, txt)
	pvDrawText(img, utils.ShortenMiddle(title, 40), 16, 36, 98, txt)
	pvFillRect(img, image.Rect(36, 110, w-36, 112), accent)
	pvDrawText(img, t.AuthorSelectTitleChapter, 12, 36, 130, dim)
	for i, btn := range buttons {
		pvDrawText(img, btn.Label, 18, 110, 184+i*34+18, txt)
	}
}

func pvRenderMinimal(img *image.RGBA, title string, buttons []dvdMenuButton, txt, accent color.RGBA, w int) {
	upper := strings.ToUpper(utils.ShortenMiddle(title, 40))
	tw := pvMeasureText(upper, 28)
	x := (w - tw) / 2
	if x < 20 {
		x = 20
	}
	pvDrawText(img, upper, 28, x, 64, txt)
	pvFillRect(img, image.Rect(100, 97, w-100, 99), accent)
	for i, btn := range buttons {
		pvDrawText(img, strings.ToUpper(btn.Label), 18, 120, 150+i*40+18, txt)
	}
}

// ---- main render ----

func renderMenuPreviewImage(p previewParams, t i18n.Strings) image.Image {
	width, height := dvdMenuDimensions(p.region)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	thm := p.theme
	if thm == nil {
		thm = menuThemes["VideoTools"]
	}
	bg := menuColorFromHex(thm.BackgroundColor)
	txt := menuColorFromHex(thm.TextColor)
	hdr := menuColorFromHex(thm.HeaderColor)
	accent := menuColorFromHex(thm.AccentColor)
	dim := dimColor(txt, 0.55)

	// Background: image if available, else solid fill
	bgLoaded := false
	if p.bgImagePath != "" {
		if f, err := os.Open(p.bgImagePath); err == nil {
			if srcImg, _, err2 := image.Decode(f); err2 == nil {
				draw.Draw(img, img.Bounds(), pvScaleImage(srcImg, width, height), image.Point{}, draw.Src)
				bgLoaded = true
			}
			f.Close()
		}
	}
	if !bgLoaded {
		draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	}

	title := strings.TrimSpace(p.title)
	if title == "" {
		title = t.AuthorDVDMenu
	}

	switch p.template {
	case "Minimal":
		pvRenderMinimal(img, title, p.buttons, txt, accent, width)
	default:
		pvRenderSimple(img, title, p.buttons, txt, hdr, accent, dim, width, t)
	}

	// Highlight overlay
	if p.highlighted >= 0 && p.highlighted < len(p.buttons) {
		btn := p.buttons[p.highlighted]
		pvBlendRect(img, image.Rect(btn.X0, btn.Y0, btn.X1, btn.Y1), accent, 0.38)
		// Also draw a thin top border on the highlight rect for clarity
		pvFillRect(img, image.Rect(btn.X0, btn.Y0, btn.X1, btn.Y0+2), accent)
	}

	return img
}

// ---- panel builder ----

// buildMenuPreviewPanel returns the interactive preview panel and a refresh trigger.
// Call the returned function whenever any menu-related state changes.
func buildMenuPreviewPanel(state *appState) (fyne.CanvasObject, func()) {
	t := i18n.T()
	highlighted := -1
	viewType := "main" // "main" or "chapters"

	previewImg := canvas.NewImageFromResource(nil)
	previewImg.FillMode = canvas.ImageFillContain
	previewImg.SetMinSize(fyne.NewSize(360, 240))

	navRow := container.NewHBox()
	statusLabel := widget.NewLabel(t.AuthorClickButtonPreview)
	statusLabel.TextStyle = fyne.TextStyle{Italic: true}
	statusLabel.Wrapping = fyne.TextWrapWord

	viewMainBtn := widget.NewButton("Main Menu", nil)
	viewChaptersBtn := widget.NewButton("Chapters", nil)
	viewMainBtn.Importance = widget.HighImportance

	var debounceMu sync.Mutex
	var debounceTimer *time.Timer
	var scheduleRefresh func()
	var rebuildNavRow func()

	getCurrentButtons := func() []dvdMenuButton {
		hasExtras := false
		for _, c := range state.authorClips {
			if c.IsExtra {
				hasExtras = true
				break
			}
		}
		region := state.authorRegion
		if region == "" {
			region = "NTSC"
		}
		w, h := dvdMenuDimensions(region)
		if viewType == "chapters" {
			return buildChapterMenuButtons(state.authorChapters, w, h)
		}
		return buildDVDMenuButtons(state.authorChapters, hasExtras, w, h)
	}

	scheduleRefresh = func() {
		// Capture all inputs on the UI goroutine before scheduling background work.
		thm := resolvePreviewTheme(state)
		tmpl := state.authorMenuTemplate
		if tmpl == "" {
			tmpl = "Minimal"
		}
		region := state.authorRegion
		if region == "" {
			region = "NTSC"
		}
		btns := getCurrentButtons()
		hi := highlighted
		if hi >= len(btns) {
			hi = -1
		}
		p := previewParams{
			theme:       thm,
			template:    tmpl,
			title:       state.authorTitle,
			region:      region,
			bgImagePath: state.authorMenuBackgroundImage,
			buttons:     btns,
			highlighted: hi,
		}

		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(80*time.Millisecond, func() {
			rendered := renderMenuPreviewImage(p, t)
			var buf bytes.Buffer
			_ = png.Encode(&buf, rendered)
			data := buf.Bytes()
			runOnUI(func() {
				previewImg.Resource = fyne.NewStaticResource("dvd_menu_preview.png", data)
				previewImg.Refresh()
			})
		})
		debounceMu.Unlock()
	}

	rebuildNavRow = func() {
		btns := getCurrentButtons()

		hasChapters := len(state.authorChapters) > 1
		if hasChapters {
			viewChaptersBtn.Show()
		} else {
			viewChaptersBtn.Hide()
			if viewType == "chapters" {
				viewType = "main"
				viewMainBtn.Importance = widget.HighImportance
				viewChaptersBtn.Importance = widget.MediumImportance
			}
		}
		viewMainBtn.Refresh()
		viewChaptersBtn.Refresh()

		if highlighted >= len(btns) {
			highlighted = -1
		}

		navRow.Objects = nil
		for i, btn := range btns {
			i, lbl := i, btn.Label
			b := widget.NewButton(lbl, func() {
				if highlighted == i {
					highlighted = -1
				} else {
					highlighted = i
				}
				rebuildNavRow()
				scheduleRefresh()
			})
			if i == highlighted {
				b.Importance = widget.HighImportance
			} else {
				b.Importance = widget.MediumImportance
			}
			navRow.Add(b)
		}
		navRow.Refresh()

		if highlighted >= 0 && highlighted < len(btns) {
			statusLabel.SetText("Highlighted: " + btns[highlighted].Label)
		} else {
			statusLabel.SetText(t.AuthorClickButtonPreview)
		}
	}

	prevBtn := widget.NewButton("◄", func() {
		btns := getCurrentButtons()
		if len(btns) == 0 {
			return
		}
		if highlighted <= 0 {
			highlighted = len(btns) - 1
		} else {
			highlighted--
		}
		rebuildNavRow()
		scheduleRefresh()
	})

	nextBtn := widget.NewButton("►", func() {
		btns := getCurrentButtons()
		if len(btns) == 0 {
			return
		}
		if highlighted < 0 || highlighted >= len(btns)-1 {
			highlighted = 0
		} else {
			highlighted++
		}
		rebuildNavRow()
		scheduleRefresh()
	})

	viewMainBtn.OnTapped = func() {
		viewType = "main"
		highlighted = -1
		viewMainBtn.Importance = widget.HighImportance
		viewChaptersBtn.Importance = widget.MediumImportance
		viewMainBtn.Refresh()
		viewChaptersBtn.Refresh()
		rebuildNavRow()
		scheduleRefresh()
	}

	viewChaptersBtn.OnTapped = func() {
		viewType = "chapters"
		highlighted = -1
		viewMainBtn.Importance = widget.MediumImportance
		viewChaptersBtn.Importance = widget.HighImportance
		viewMainBtn.Refresh()
		viewChaptersBtn.Refresh()
		rebuildNavRow()
		scheduleRefresh()
	}

	viewToggle := container.NewHBox(viewMainBtn, viewChaptersBtn)
	navWithArrows := container.NewBorder(nil, nil, prevBtn, nextBtn, navRow)

	panel := container.NewVBox(
		viewToggle,
		previewImg,
		navWithArrows,
		statusLabel,
	)

	// Kick off initial render
	rebuildNavRow()
	scheduleRefresh()

	return panel, scheduleRefresh
}

// buildInteractiveMenuPreviewTab creates a full interactive DVD menu preview tab.
// When menus are enabled, this provides a fully interactive preview where users
// can navigate the menu and play videos by pressing the title/chapter buttons.
func buildInteractiveMenuPreviewTab(state *appState) fyne.CanvasObject {
	t := i18n.T()

	playMainFeature := func() {
		if len(state.authorClips) > 0 {
			for _, c := range state.authorClips {
				if !c.IsExtra {
					if c.Path != "" {
						state.showPlayerViewForPath(c.Path)
						return
					}
				}
			}
			// If no non-extra, play first clip
			if state.authorClips[0].Path != "" {
				state.showPlayerViewForPath(state.authorClips[0].Path)
			}
		}
	}

	previewImg := canvas.NewImageFromResource(nil)
	previewImg.FillMode = canvas.ImageFillContain
	previewImg.SetMinSize(fyne.NewSize(640, 420))

	viewType := "main"
	highlighted := -1

	btnCommands := map[int]string{}

	getCurrentButtons := func() []dvdMenuButton {
		hasExtras := false
		for _, c := range state.authorClips {
			if c.IsExtra {
				hasExtras = true
				break
			}
		}
		region := state.authorRegion
		if region == "" {
			region = "NTSC"
		}
		w, h := dvdMenuDimensions(region)
		if viewType == "chapters" {
			return buildChapterMenuButtons(state.authorChapters, w, h)
		}
		if viewType == "extras" {
			var extras []extraItem
			for i, c := range state.authorClips {
				if c.IsExtra {
					extras = append(extras, extraItem{Title: c.DisplayName, TitleNum: i + 2})
				}
			}
			return buildExtrasMenuButtons(extras, w, h)
		}
		return buildDVDMenuButtons(state.authorChapters, hasExtras, w, h)
	}

	scheduleRefresh := func() {
		thm := resolvePreviewTheme(state)
		tmpl := state.authorMenuTemplate
		if tmpl == "" {
			tmpl = "Minimal"
		}
		region := state.authorRegion
		if region == "" {
			region = "NTSC"
		}
		btns := getCurrentButtons()
		hi := highlighted
		if hi >= len(btns) {
			hi = -1
		}
		p := previewParams{
			theme:       thm,
			template:    tmpl,
			title:       state.authorTitle,
			region:      region,
			bgImagePath: state.authorMenuBackgroundImage,
			buttons:     btns,
			highlighted: hi,
		}

		rendered := renderMenuPreviewImage(p, t)
		var buf bytes.Buffer
		_ = png.Encode(&buf, rendered)
		data := buf.Bytes()
		runOnUI(func() {
			previewImg.Resource = fyne.NewStaticResource("dvd_menu_preview.png", data)
			previewImg.Refresh()
		})
	}

	viewMainBtn := widget.NewButton("Main Menu", func() {
		viewType = "main"
		highlighted = 0
		scheduleRefresh()
	})
	viewChaptersBtn := widget.NewButton("Chapters", func() {
		if len(state.authorChapters) > 1 {
			viewType = "chapters"
			highlighted = 0
			scheduleRefresh()
		}
	})
	viewExtrasBtn := widget.NewButton("Extras", func() {
		hasExtras := false
		for _, c := range state.authorClips {
			if c.IsExtra {
				hasExtras = true
				break
			}
		}
		if hasExtras {
			viewType = "extras"
			highlighted = 0
			scheduleRefresh()
		}
	})

	rebuildNavRow := func() {
		btns := getCurrentButtons()
		btnCommands = map[int]string{}

		hasChapters := len(state.authorChapters) > 1
		hasExtras := false
		for _, c := range state.authorClips {
			if c.IsExtra {
				hasExtras = true
				break
			}
		}

		viewChaptersBtn.Hidden = !hasChapters
		viewExtrasBtn.Hidden = !hasExtras

		navRow := container.NewHBox()
		for i, btn := range btns {
			i := i
			b := widget.NewButton(btn.Label, func() {
				highlighted = i
				scheduleRefresh()

				cmd := btnCommands[i]
				switch {
				case cmd == "jump title 1;" || strings.HasPrefix(cmd, "jump title "):
					playMainFeature()
				case strings.HasPrefix(cmd, "jump title "):
					// Jump to specific title (extra)
					titleNum := 0
					fmt.Sscanf(cmd, "jump title %d", &titleNum)
					extraIdx := titleNum - 2
					if extraIdx >= 0 && extraIdx < len(state.authorClips) {
						for j, c := range state.authorClips {
							if c.IsExtra && j == extraIdx {
								if c.Path != "" {
									state.showPlayerViewForPath(c.Path)
								}
								break
							}
						}
					}
				case strings.HasPrefix(cmd, "jump menu "):
					// Menu navigation handled by view buttons
				}
			})
			if i == highlighted {
				b.Importance = widget.HighImportance
			}
			navRow.Add(b)
		}
	}

	// Initial state
	if len(state.authorChapters) > 1 {
		viewChaptersBtn.Importance = widget.HighImportance
	}
	if highlighted < 0 {
		highlighted = 0
	}
	scheduleRefresh()
	rebuildNavRow()

	previewContainer := container.NewBorder(
		container.NewHBox(viewMainBtn, viewChaptersBtn, viewExtrasBtn),
		nil,
		nil,
		nil,
		previewImg,
	)

	return previewContainer
}
