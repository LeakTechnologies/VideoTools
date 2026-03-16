package theme

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strconv"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// MenuAssets contains the rendered images and data for a DVD menu.
type MenuAssets struct {
	Background image.Image
	Highlight  *image.Paletted // 2-bit indexed (0: bg, 1: pattern, 2: e1, 3: e2)
	Buttons    []ButtonRect
}

type ButtonRect struct {
	ID     string
	X0, Y0 int
	X1, Y1 int
}

// Renderer converts a ScriptableTheme into images for DVD menu creation.
type Renderer struct {
	theme *ScriptableTheme
	fontData []byte
}

// NewRenderer creates a new theme renderer.
func NewRenderer(theme *ScriptableTheme) *Renderer {
	return &Renderer{theme: theme}
}

// SetFont sets the font data for text rendering.
func (r *Renderer) SetFont(data []byte) {
	r.fontData = data
}

// RenderMenu produces all assets needed for the menu.
func (r *Renderer) RenderMenu() (*MenuAssets, error) {
	logging.Info(logging.CatDVD, "Rendering full menu for theme: %s", r.theme.Name)
	
	width, height := r.theme.Resolution.Width, r.theme.Resolution.Height
	bg := image.NewRGBA(image.Rect(0, 0, width, height))
	
	spuPalette := color.Palette{
		color.Transparent,
		color.RGBA{255, 255, 255, 255}, // Pattern
		color.RGBA{255, 255, 0, 255},   // E1
		color.RGBA{255, 0, 0, 255},     // E2
	}
	highlight := image.NewPaletted(image.Rect(0, 0, width, height), spuPalette)

	assets := &MenuAssets{
		Background: bg,
		Highlight:  highlight,
	}

	// 1. Draw Background
	if r.theme.Background.Type == "image" {
		img, err := r.loadImage(r.theme.Background.Source)
		if err == nil {
			draw.Draw(bg, bg.Bounds(), img, image.Point{}, draw.Src)
		}
	} else {
		bgColor := parseHexColor(r.theme.Elements[0].Style.BackgroundColor)
		draw.Draw(bg, bg.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
	}

	// 2. Draw Elements
	for _, el := range r.theme.Elements {
		x, y, w, h := r.parseLayout(el.Style)
		
		switch el.Type {
		case "text":
			r.drawText(bg, el.Content, x, y, el.Style)
		case "button":
			r.drawText(bg, el.Content, x, y, el.Style)
			assets.Buttons = append(assets.Buttons, ButtonRect{
				ID: el.ID, X0: x, Y0: y, X1: x + w, Y1: y + h,
			})
			r.drawHighlight(highlight, x, y, w, h)
		}
	}

	return assets, nil
}

func (r *Renderer) parseLayout(s Style) (x, y, w, h int) {
	w, _ = strconv.Atoi(strings.TrimSuffix(s.Width, "px"))
	h, _ = strconv.Atoi(strings.TrimSuffix(s.Height, "px"))
	
	// Default starting pos
	x = 100
	y = 100
	
	if strings.HasSuffix(s.Left, "px") {
		x, _ = strconv.Atoi(strings.TrimSuffix(s.Left, "px"))
	}
	if strings.HasSuffix(s.Top, "px") {
		y, _ = strconv.Atoi(strings.TrimSuffix(s.Top, "px"))
	}
	
	return x, y, w, h
}

func (r *Renderer) drawText(dst draw.Image, text string, x, y int, s Style) {
	if r.fontData == nil {
		return
	}
	
	ot, err := opentype.Parse(r.fontData)
	if err != nil {
		return
	}
	
	fontSize, _ := strconv.ParseFloat(strings.TrimSuffix(s.FontSize, "px"), 64)
	if fontSize == 0 {
		fontSize = 24
	}
	
	face, err := opentype.NewFace(ot, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	
	drawer := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{parseHexColor(s.Color)},
		Face: face,
		Dot:  fixed.P(x, y+int(fontSize)), // Baseline adjustment
	}
	drawer.DrawString(text)
}

func (r *Renderer) drawHighlight(dst *image.Paletted, x, y, w, h int) {
	rect := image.Rect(x, y, x+w, y+h)
	for py := rect.Min.Y; py < rect.Max.Y; py++ {
		for px := rect.Min.X; px < rect.Max.X; px++ {
			if px >= 0 && px < dst.Bounds().Dx() && py >= 0 && py < dst.Bounds().Dy() {
				dst.SetColorIndex(px, py, 1) // Pattern
			}
		}
	}
}

func (r *Renderer) loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func parseHexColor(s string) color.Color {
	if !strings.HasPrefix(s, "#") || len(s) != 7 {
		return color.White
	}
	r, _ := strconv.ParseUint(s[1:3], 16, 8)
	g, _ := strconv.ParseUint(s[3:5], 16, 8)
	b, _ := strconv.ParseUint(s[5:7], 16, 8)
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}
}
