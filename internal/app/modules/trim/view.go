//go:build native_media

package trim

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type Options struct {
	Window         fyne.Window
	ModuleColor    color.Color
	OnShowMainMenu func()
	OnShowQueue    func()
	OnAddToQueue   func(clip authorClip)
}

// authorClip is used here as a placeholder for trim parameters.
// In a real implementation, we might use a dedicated trimJob struct.
type authorClip struct {
	Path     string
	InPoint  time.Duration
	OutPoint time.Duration
	Mode     string // "keep" or "cut"
	Export   string // "smart" or "recode"
}

func BuildView(opts Options, initialPath string) fyne.CanvasObject {
	t := i18n.T()
	trimColor := opts.ModuleColor
	navyBlue := utils.MustHex("#191F35")
	gridColor := utils.MustHex("#171C2A")

	// --- Helpers ---
	buildTrimBox := func(title string, content fyne.CanvasObject) *fyne.Container {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	sectionGap := func() fyne.CanvasObject {
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 10))
		return gap
	}

	// --- State ---
	player := media.NewVideoPlayer()
	var engine *media.Engine
	var duration time.Duration
	currentTime := 0.0

	// --- UI Components ---
	
	// Top Navigation
	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleTrim), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance
	queueBtn := widget.NewButton(t.MenuQueue, opts.OnShowQueue)
	topBar := ui.TintedBar(trimColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// Left Side: Video & Timeline
	timeLabel := widget.NewLabel("00:00:00.000")
	durLabel := widget.NewLabel("00:00:00.000")
	
	timeline := widget.NewSlider(0, 100)
	timeline.OnChanged = func(val float64) {
		if engine != nil {
			target := (val / 100.0) * duration.Seconds()
			engine.Seek(target)
			// Update frame logic...
		}
	}

	setInBtn := widget.NewButton(t.TrimSetIn, func() {})
	setOutBtn := widget.NewButton(t.TrimSetOut, func() {})
	playBtn := widget.NewButton(t.ActionPlay, func() {})
	pauseBtn := widget.NewButton(t.ActionPause, func() {})
	
	transport := container.NewHBox(
		layout.NewSpacer(),
		playBtn, pauseBtn,
		widget.NewSeparator(),
		setInBtn, setOutBtn,
		layout.NewSpacer(),
	)

	videoContainer := container.NewBorder(
		nil,
		container.NewVBox(
			container.NewBorder(nil, nil, timeLabel, durLabel, timeline),
			transport,
		),
		nil, nil,
		container.NewMax(canvas.NewRectangle(color.Black), player),
	)

	// Right Side: Settings
	inPointEntry := widget.NewEntry()
	inPointEntry.SetPlaceHolder("00:00:00.000")
	outPointEntry := widget.NewEntry()
	outPointEntry.SetPlaceHolder("00:00:00.000")

	rangeBox := buildTrimBox("Trim Range", container.NewVBox(
		widget.NewLabel(t.TrimInPoint),
		inPointEntry,
		widget.NewLabel(t.TrimOutPoint),
		outPointEntry,
	))

	modeSelect := widget.NewRadioGroup([]string{t.TrimModeKeep, t.TrimModeCut}, nil)
	modeSelect.SetSelected(t.TrimModeKeep)
	
	exportSelect := widget.NewRadioGroup([]string{t.TrimSmartCopy, t.TrimRecode}, nil)
	exportSelect.SetSelected(t.TrimSmartCopy)

	optionsBox := buildTrimBox("Output Options", container.NewVBox(
		widget.NewLabel(t.TrimMode),
		modeSelect,
		widget.NewSeparator(),
		widget.NewLabel("Method:"),
		exportSelect,
	))

	settingsScroll := container.NewVScroll(container.NewVBox(
		rangeBox,
		sectionGap(),
		optionsBox,
	))

	// Bottom Bar
	addQueueBtn := widget.NewButton(t.ActionAddToQueue, func() {})
	trimNowBtn := widget.NewButton("Trim Now", func() {})
	trimNowBtn.Importance = widget.HighImportance
	
	bottomBar := ui.TintedBar(trimColor, container.NewHBox(layout.NewSpacer(), addQueueBtn, trimNowBtn))

	// Main Layout
	mainContent := container.NewHSplit(
		videoContainer,
		container.NewPadded(settingsScroll),
	)
	mainContent.Offset = 0.75

	return container.NewBorder(topBar, bottomBar, nil, nil, mainContent)
}
