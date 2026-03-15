package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/enhancement"
)

func buildEnhancementView(state *appState) fyne.CanvasObject {
	return enhancement.BuildView()
}
