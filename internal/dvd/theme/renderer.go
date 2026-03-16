package theme

import (
	"fmt"
	"image"
	"image/draw"
	"os"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Renderer converts a ScriptableTheme into images for DVD menu creation.
type Renderer struct {
	theme *ScriptableTheme
}

// NewRenderer creates a new theme renderer.
func NewRenderer(theme *ScriptableTheme) *Renderer {
	return &Renderer{theme: theme}
}

// RenderBackground produces the static background image for the menu.
func (r *Renderer) RenderBackground() (image.Image, error) {
	logging.Info(logging.CatDVD, "Rendering background for theme: %s", r.theme.Name)
	
	dest := image.NewRGBA(image.Rect(0, 0, r.theme.Resolution.Width, r.theme.Resolution.Height))
	
	// Draw background color or image
	if r.theme.Background.Type == "image" {
		img, err := r.loadImage(r.theme.Background.Source)
		if err != nil {
			return nil, err
		}
		draw.Draw(dest, dest.Bounds(), img, image.Point{}, draw.Src)
	}
	
	// Draw text elements
	for _, el := range r.theme.Elements {
		if el.Type == "text" {
			// [Implement font drawing using golang.org/x/image/font]
		}
	}
	
	return dest, nil
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
