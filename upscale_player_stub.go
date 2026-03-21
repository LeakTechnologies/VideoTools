//go:build !native_media

package main

import (
	"fyne.io/fyne/v2"
)

func BuildUpscaleVideoCompare(size fyne.Size) fyne.CanvasObject {
	return nil
}

func BuildUpscaleDualPlayerControls() fyne.CanvasObject {
	return nil
}
