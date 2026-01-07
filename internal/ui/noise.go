package ui

import (
	"image"
	"image/color"
	"math/rand"
	"os"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

const (
	noiseTileSize  = 128
	noiseAlpha     = uint8(8)  // ~3% opacity
	noiseVariance  = uint8(6)  // low contrast
	noiseMeanValue = uint8(128)
)

var (
	noiseOnce         sync.Once
	noiseTile         *image.NRGBA
	uiTextureEnabled  = true
)

func init() {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VT_UI_TEXTURE"))) {
	case "0", "false", "off", "no":
		uiTextureEnabled = false
	}
}

// NoisyBackgroundObjects returns background layers with an optional static noise overlay.
func NoisyBackgroundObjects(bg fyne.CanvasObject) []fyne.CanvasObject {
	if !uiTextureEnabled {
		return []fyne.CanvasObject{bg}
	}
	return []fyne.CanvasObject{bg, NewNoiseOverlay()}
}

// NewNoiseOverlay creates a static, cached grayscale noise layer.
func NewNoiseOverlay() fyne.CanvasObject {
	return canvas.NewRaster(func(w, h int) image.Image {
		if w <= 0 || h <= 0 {
			return image.NewNRGBA(image.Rect(0, 0, 1, 1))
		}
		tile := getNoiseTile()
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		tw := tile.Bounds().Dx()
		th := tile.Bounds().Dy()
		for y := 0; y < h; y++ {
			ty := y % th
			for x := 0; x < w; x++ {
				tx := x % tw
				dst.SetNRGBA(x, y, tile.NRGBAAt(tx, ty))
			}
		}
		return dst
	})
}

func getNoiseTile() *image.NRGBA {
	noiseOnce.Do(func() {
		tile := image.NewNRGBA(image.Rect(0, 0, noiseTileSize, noiseTileSize))
		rng := rand.New(rand.NewSource(1))
		for y := 0; y < noiseTileSize; y++ {
			for x := 0; x < noiseTileSize; x++ {
				delta := int(rng.Intn(int(noiseVariance)*2+1)) - int(noiseVariance)
				v := int(noiseMeanValue) + delta
				if v < 0 {
					v = 0
				} else if v > 255 {
					v = 255
				}
				tile.SetNRGBA(x, y, color.NRGBA{R: uint8(v), G: uint8(v), B: uint8(v), A: noiseAlpha})
			}
		}
		noiseTile = tile
	})
	return noiseTile
}
