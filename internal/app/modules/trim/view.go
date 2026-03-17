//go:build native_media

package trim

import (
	"fmt"
	"image"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
)

type Options struct {
	Window         fyne.Window
	OnShowMainMenu func()
}

type trimState struct {
	engine *media.Engine
	raster *canvas.Raster
	
	inPoint  time.Duration
	outPoint time.Duration
	
	currentTime time.Duration
	duration    time.Duration
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()
	state := &trimState{}

	// Native Player Raster
	state.raster = canvas.NewRaster(func(w, h int) image.Image {
		if state.engine == nil {
			return image.NewRGBA(image.Rect(0, 0, w, h))
		}
		// In a real implementation, we'd pull the current cached frame
		return image.NewRGBA(image.Rect(0, 0, w, h))
	})

	inPointLabel := widget.NewLabel(t.TrimInPoint + ": 00:00:00.000")
	outPointLabel := widget.NewLabel(t.TrimOutPoint + ": 00:00:00.000")
	durationLabel := widget.NewLabel(t.LabelDuration + ": 00:00:00.000")

	setInBtn := widget.NewButton(t.TrimSetIn, func() {
		state.inPoint = state.currentTime
		inPointLabel.SetText(fmt.Sprintf("%s: %v", t.TrimInPoint, state.inPoint))
	})
	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		state.outPoint = state.currentTime
		outPointLabel.SetText(fmt.Sprintf("%s: %v", t.TrimOutPoint, state.outPoint))
	})
	
	clearBtn := widget.NewButton(t.TrimClear, func() {
		state.inPoint = 0
		state.outPoint = 0
		inPointLabel.SetText(t.TrimInPoint + ": 00:00:00.000")
		outPointLabel.SetText(t.TrimOutPoint + ": 00:00:00.000")
	})

	timeline := widget.NewSlider(0, 100)
	timeline.OnChanged = func(val float64) {
		if state.engine != nil {
			target := (val / 100.0) * state.duration.Seconds()
			state.engine.Seek(target)
		}
	}
	
	playBtn := widget.NewButton(t.ActionPlay, func() {
		if state.engine != nil {
			state.engine.Start()
		}
	})
	pauseBtn := widget.NewButton(t.ActionPause, func() {
		// Engine pause logic...
	})

	transport := container.NewHBox(playBtn, pauseBtn, setInBtn, setOutBtn, clearBtn)
	
	leftSide := container.NewBorder(
		nil,
		container.NewVBox(timeline, transport),
		nil,
		nil,
		state.raster,
	)

	trimMode := widget.NewRadioGroup([]string{t.TrimModeKeep, t.TrimModeCut}, nil)
	trimMode.SetSelected(t.TrimModeKeep)

	rightSide := container.NewVBox(
		widget.NewLabelWithStyle(t.ModuleTrim+" "+t.SettingsTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(t.TrimMode),
		trimMode,
		widget.NewSeparator(),
		inPointLabel,
		outPointLabel,
		durationLabel,
		widget.NewSeparator(),
		widget.NewButton(t.ActionAddToQueue, func() {}),
	)

	return container.NewHSplit(leftSide, rightSide)
}
