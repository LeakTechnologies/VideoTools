package ui

import (
	vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"
)

// VTSlider and VTProgressBar are defined in internal/theme so they can be
// used from internal/media without creating an import cycle. These aliases
// let every other package continue to import them via the ui package.

type VTSlider = vtheme.VTSlider
type VTProgressBar = vtheme.VTProgressBar

func NewVTSlider(min, max float64) *VTSlider {
	return vtheme.NewVTSlider(min, max)
}

func NewVTProgressBar() *VTProgressBar {
	return vtheme.NewVTProgressBar()
}
