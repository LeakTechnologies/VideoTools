package ui

import (
	vtheme "github.com/LeakTechnologies/VideoTools/internal/theme"
)

// Slider is a styled slider: thin coloured track + hollow circle thumb.
// Canonical implementation lives in internal/theme to avoid import cycles
// with internal/media packages.
type Slider = vtheme.Slider

// MakeSlider constructs a Slider with the given range.
func MakeSlider(min, max float64) *Slider {
	return vtheme.MakeSlider(min, max)
}

// ProgressBar renders a thin styled progress bar (track + filled portion).
// Use instead of widget.ProgressBar in module views.
// Not for use in the queue module where the stock widget is intentional.
type ProgressBar = vtheme.ProgressBar

// MakeProgressBar constructs a ProgressBar with the default [0, 1] range.
func MakeProgressBar() *ProgressBar {
	return vtheme.MakeProgressBar()
}
