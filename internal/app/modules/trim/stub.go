//go:build !native_media

package trim

import (
	"image/color"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

type Options struct {
	Window         fyne.Window
	ModuleColor    color.Color
	StatsBar       fyne.CanvasObject
	Player         *ui.InlineVideoPlayer
	OnShowMainMenu func()
	OnShowQueue    func()
	OnAddToQueue   func(clip TrimClip)
	OnLoadFile     func(path string)
	OnProbeVideo   func(path string) (float64, error)
}

type TrimClip struct {
	Path     string
	InPoint  time.Duration
	OutPoint time.Duration
	Duration time.Duration // total file duration, used for Cut Region progress
	Mode     string
	Export   string
}

type trimState struct {
	videoPath    string
	duration     float64
	currentMs    int64
	inPointMs    int64
	outPointMs   int64
	modeSelect   *widget.Select
	exportSelect *widget.Select
	inEntry      *widget.Entry
	outEntry     *widget.Entry
	timeline     *widget.Slider
	addBtn       *ui.PillButton
	videoPreview *ui.VideoPreview
}

func BuildView(opts Options, initialPath string) fyne.CanvasObject {
	t := i18n.T()
	trimColor := opts.ModuleColor
	if trimColor == nil {
		trimColor = utils.MustHex("#4A5D73")
	}
	navyBlue := utils.MustHex("#191F35")
	gridColor := utils.MustHex("#171C2A")
	darkBg := utils.MustHex("#0F1529")

	state := &trimState{inPointMs: 0, outPointMs: 0}

	section := func(title string, content fyne.CanvasObject) *fyne.Container {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		body := container.NewVBox()
		body.Add(widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		body.Add(widget.NewSeparator())
		body.Add(content)
		return container.NewMax(bg, container.NewPadded(body))
	}

	timeLabel := widget.NewLabel("00:00:00.000")
	durLabel := widget.NewLabel("00:00:00.000")

	state.timeline = widget.NewSlider(0, 100)
	state.timeline.OnChanged = func(val float64) {
		if state.duration > 0 {
			state.currentMs = int64(val / 100.0 * float64(state.duration*1000))
			timeLabel.SetText(formatMs(state.currentMs))
			if state.videoPreview != nil {
				state.videoPreview.UpdatePreview(float64(state.currentMs) / 1000.0)
			}
		}
	}

	state.inEntry = widget.NewEntry()
	state.inEntry.SetPlaceHolder("00:00:00.000")
	state.inEntry.OnChanged = func(s string) {
		if ms := parseMs(s); ms >= 0 {
			state.inPointMs = ms
		}
	}

	state.outEntry = widget.NewEntry()
	state.outEntry.SetPlaceHolder("00:00:00.000")
	state.outEntry.OnChanged = func(s string) {
		if ms := parseMs(s); ms >= 0 {
			state.outPointMs = ms
		}
	}

	setInBtn := widget.NewButton(t.TrimSetIn, func() {
		state.inPointMs = state.currentMs
		state.inEntry.SetText(formatMs(state.inPointMs))
	})
	setInBtn.Disable()

	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		state.outPointMs = state.currentMs
		state.outEntry.SetText(formatMs(state.outPointMs))
	})
	setOutBtn.Disable()

	clearBtn := widget.NewButton(t.TrimClear, func() {
		state.inPointMs = 0
		state.outPointMs = int64(state.duration * 1000)
		state.inEntry.SetText("")
		state.outEntry.SetText("")
	})

	openBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFileOpen(func(f fyne.URIReadCloser, err error) {
			if err != nil || f == nil {
				return
			}
			path := f.URI().Path()
			f.Close()
			if opts.OnLoadFile != nil {
				opts.OnLoadFile(path)
			}
			if opts.OnProbeVideo != nil {
				if dur, err := opts.OnProbeVideo(path); err == nil {
					state.videoPath = path
					state.duration = dur
					state.currentMs = 0
					state.inPointMs = 0
					state.outPointMs = int64(dur * 1000)
					durLabel.SetText(formatMs(int64(dur * 1000)))
					timeLabel.SetText(formatMs(0))
					state.timeline.SetValue(0)
					setInBtn.Enable()
					setOutBtn.Enable()
					state.addBtn.Enable()
					if state.videoPreview != nil {
						state.videoPreview.SetVideo(path, dur)
					}
				}
			}
		}, opts.Window)
	})

	previewBtn := widget.NewButton(t.TrimPreview, func() {
		if state.videoPath != "" {
			if u, err := url.Parse("file://" + state.videoPath); err == nil {
				fyne.CurrentApp().OpenURL(u)
			}
		}
	})

	state.modeSelect = widget.NewSelect([]string{t.TrimModeKeep, t.TrimModeCut}, func(s string) {})
	state.modeSelect.SetSelected(t.TrimModeKeep)

	state.exportSelect = widget.NewSelect([]string{t.TrimSmartCopy, t.TrimRecode}, func(s string) {})
	state.exportSelect.SetSelected(t.TrimSmartCopy)

	state.addBtn = ui.NewPillButton(t.MenuQueue, trimColor, func() {
		if state.videoPath == "" || state.duration == 0 {
			dialog.ShowInformation(t.DialogNoVideo, "Please load a video first.", opts.Window)
			return
		}
		if state.outPointMs <= state.inPointMs {
			dialog.ShowInformation(t.TrimInvalidSelection, "Out point must be after in point.", opts.Window)
			return
		}
		clip := TrimClip{
			Path:     state.videoPath,
			InPoint:  time.Duration(state.inPointMs) * time.Millisecond,
			OutPoint: time.Duration(state.outPointMs) * time.Millisecond,
			Duration: time.Duration(state.duration * float64(time.Second)),
			Mode:     strings.ToLower(state.modeSelect.Selected),
			Export:   strings.ToLower(state.exportSelect.Selected),
		}
		if opts.OnAddToQueue != nil {
			opts.OnAddToQueue(clip)
		}
	})
	state.addBtn.Disable()

	state.videoPreview = ui.NewVideoPreview()

	videoStage := canvas.NewRectangle(navyBlue)
	videoStage.CornerRadius = 8
	videoStage.StrokeColor = gridColor
	videoStage.StrokeWidth = 1
	_ = darkBg // retained for potential future use
	videoArea := container.NewMax(videoStage, state.videoPreview.GetContainer())

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleTrim), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance
	topBar := ui.TintedBar(trimColor, container.NewHBox(backBtn, layout.NewSpacer()))

	leftPanel := section(t.ModuleTrim, container.NewVBox(
		container.NewBorder(nil, nil, timeLabel, durLabel, state.timeline),
		container.NewHBox(layout.NewSpacer(), setInBtn, setOutBtn, layout.NewSpacer()),
	))

	rightPanel := section(t.ModuleTrim+" Settings", container.NewVBox(
		widget.NewLabel(t.TrimMode), state.modeSelect,
		widget.NewLabel(t.TrimOutput), state.exportSelect,
		widget.NewSeparator(),
		container.NewHBox(clearBtn, state.addBtn),
	))

	selPanel := section(t.TrimInPoint+" / "+t.TrimOutPoint, container.NewVBox(
		container.NewHBox(widget.NewLabel(t.TrimInPoint+":"), state.inEntry),
		container.NewHBox(widget.NewLabel(t.TrimOutPoint+":"), state.outEntry),
	))

	split := container.NewHSplit(
		container.NewBorder(nil, container.NewVBox(leftPanel), nil, nil, videoArea),
		container.NewVBox(rightPanel, selPanel, container.NewHBox(layout.NewSpacer(), openBtn, previewBtn)),
	)
	split.Offset = 0.65

	// Footer matching other modules' moduleFooter pattern.
	footerBg := canvas.NewRectangle(trimColor)
	footerBg.SetMinSize(fyne.NewSize(0, 44))
	footerContent := container.NewMax(footerBg, container.NewPadded(container.NewHBox(layout.NewSpacer())))
	var footer fyne.CanvasObject
	if opts.StatsBar != nil {
		statsBg := canvas.NewRectangle(&color.RGBA{R: 34, G: 34, B: 34, A: 255})
		statsBg.SetMinSize(fyne.NewSize(0, 32))
		statsStrip := container.NewMax(statsBg, container.NewPadded(opts.StatsBar))
		footer = container.NewVBox(statsStrip, footerContent)
	} else {
		footer = footerContent
	}

	return container.NewBorder(topBar, footer, nil, nil, split)
}

func formatMs(ms int64) string {
	return time.Time{}.Add(time.Duration(ms) * time.Millisecond).Format("15:04:05.000")
}

func parseMs(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	for _, f := range []string{"15:04:05.000", "15:04:05", "15:04.000", "15:04"} {
		if t, err := time.Parse(f, s); err == nil {
			return int64(t.Hour())*3600000 + int64(t.Minute())*60000 + int64(t.Second())*1000 + int64(t.Nanosecond()/1000000)
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f * 1000)
	}
	return -1
}
