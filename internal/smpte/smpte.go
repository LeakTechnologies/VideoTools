package smpte

import (
	"image"
	"image/color"
	"math"
	"sync"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	vcrFontData   []byte
	parsedFont    *opentype.Font
	fontParseOnce sync.Once

	faceMu       sync.Mutex
	faceLastSize float64
	faceLastFace font.Face
)

// SetVCRFont registers the VCR OSD Mono TTF bytes for the SMPTE idle text overlay.
// Call once at startup before any player is shown.
func SetVCRFont(data []byte) {
	vcrFontData = data
}

// DrawBars generates a SMPTE 75% colour-bar test pattern of the given dimensions.
// idleText is rendered in a black box centred in the top section; pass "" to omit it.
func DrawBars(w, h int, idleText string) *image.RGBA {
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// Row heights: top 66.67%, mid 8.33%, bottom 25%
	topH := int(float64(h) * 0.6667)
	midH := int(float64(h) * 0.0833)
	botH := h - topH - midH
	midY := topH
	botY := topH + midH

	topColors := [7]color.RGBA{
		{R: 0xb4, G: 0xb4, B: 0xb4, A: 0xff}, // light grey
		{R: 0xb4, G: 0xb4, B: 0x10, A: 0xff}, // yellow
		{R: 0x10, G: 0xb4, B: 0xb4, A: 0xff}, // cyan
		{R: 0x10, G: 0xb4, B: 0x10, A: 0xff}, // green
		{R: 0xb4, G: 0x10, B: 0xb4, A: 0xff}, // magenta
		{R: 0xb4, G: 0x10, B: 0x10, A: 0xff}, // red
		{R: 0x10, G: 0x10, B: 0xb4, A: 0xff}, // blue
	}
	barW := w / 7
	for i, c := range topColors {
		x := i * barW
		bw := barW
		if i == 6 {
			bw = w - x
		}
		fillRect(img, x, 0, bw, topH, c)
	}

	midColors := [7]color.RGBA{
		{R: 0x10, G: 0x10, B: 0xb4, A: 0xff}, // blue
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff}, // black
		{R: 0xb4, G: 0x10, B: 0xb4, A: 0xff}, // magenta
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff}, // black
		{R: 0x10, G: 0xb4, B: 0xb4, A: 0xff}, // cyan
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff}, // black
		{R: 0xb4, G: 0xb4, B: 0xb4, A: 0xff}, // light grey
	}
	for i, c := range midColors {
		x := i * barW
		bw := barW
		if i == 6 {
			bw = w - x
		}
		fillRect(img, x, midY, bw, midH, c)
	}

	// Bottom row: proportional PLUGE blocks (fractions normalised from 1024px SVG)
	botFracs := [8]float64{
		181.85 / 1024.0, // dark navy
		183.17 / 1024.0, // near-white
		183.99 / 1024.0, // purple
		182.42 / 1024.0, // reference black
		48.76 / 1024.0,  // PLUGE sub-black
		48.76 / 1024.0,  // reference black
		48.76 / 1024.0,  // PLUGE super-black
		146.29 / 1024.0, // reference black
	}
	botColors := [8]color.RGBA{
		{R: 0x00, G: 0x21, B: 0x4c, A: 0xff},
		{R: 0xeb, G: 0xeb, B: 0xeb, A: 0xff},
		{R: 0x4c, G: 0x00, B: 0x82, A: 0xff},
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
		{R: 0x08, G: 0x08, B: 0x08, A: 0xff},
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
		{R: 0x18, G: 0x18, B: 0x18, A: 0xff},
		{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
	}
	bx := 0
	for i, frac := range botFracs {
		bw := int(frac * float64(w))
		if i == 7 {
			bw = w - bx
		}
		fillRect(img, bx, botY, bw, botH, botColors[i])
		bx += bw
	}

	if idleText != "" {
		drawText(img, w, topH, idleText)
	}

	return img
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			img.SetRGBA(px, py, c)
		}
	}
}

func getFontFace(size float64) font.Face {
	fontParseOnce.Do(func() {
		if len(vcrFontData) == 0 {
			return
		}
		f, err := opentype.Parse(vcrFontData)
		if err != nil {
			logging.Warning(logging.CatPlayer, "SMPTE: failed to parse VCR font: %v", err)
			return
		}
		parsedFont = f
	})
	if parsedFont == nil {
		return nil
	}

	rounded := math.Round(size)
	faceMu.Lock()
	if rounded == faceLastSize && faceLastFace != nil {
		f := faceLastFace
		faceMu.Unlock()
		return f
	}
	faceMu.Unlock()

	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    rounded,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		logging.Warning(logging.CatPlayer, "SMPTE: failed to create font face at %.0fpt: %v", rounded, err)
		return nil
	}

	faceMu.Lock()
	faceLastSize = rounded
	faceLastFace = face
	faceMu.Unlock()
	return face
}

func drawText(img *image.RGBA, w, topH int, text string) {
	// Scale proportionally to width; reference: 48pt looks right at 1024px.
	fontSize := math.Round(48.0 * float64(w) / 1024.0)
	if fontSize < 10 {
		fontSize = 10
	} else if fontSize > 72 {
		fontSize = 72
	}

	face := getFontFace(fontSize)
	if face == nil {
		return
	}

	var textPx fixed.Int26_6
	for _, r := range text {
		adv, ok := face.GlyphAdvance(r)
		if ok {
			textPx += adv
		}
	}
	textW := textPx.Ceil()
	metrics := face.Metrics()
	textH := metrics.Ascent.Ceil() + metrics.Descent.Ceil()

	padX := int(math.Round(float64(fontSize) * 0.4))
	padY := int(math.Round(float64(fontSize) * 0.25))

	boxW := textW + padX*2
	boxH := textH + padY*2
	if boxW > w {
		boxW = w
	}

	boxX := (w - boxW) / 2
	boxY := topH/2 - boxH/2
	if boxY < 0 {
		boxY = 0
	}

	fillRect(img, boxX, boxY, boxW, boxH, color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff})

	dotX := boxX + padX
	dotY := boxY + padY + metrics.Ascent.Ceil()
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 0xeb, G: 0xeb, B: 0xeb, A: 0xff}),
		Face: face,
		Dot:  fixed.P(dotX, dotY),
	}
	drawer.DrawString(text)
}
