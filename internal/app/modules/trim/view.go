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
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type Options struct {
	Window         fyne.Window
	ModuleColor    color.Color
	OnShowMainMenu func()
	OnShowQueue    func()
}

type trimState struct {
	engine *media.Engine
	player *media.VideoPlayer
	
	inPoint  time.Duration
	outPoint time.Duration
	
	currentTime float64 // in seconds
	duration    float64 // in seconds
	
	// UI refs for updates
	timeLabel    *widget.Label
	inPointLabel *widget.Label
	outPointLabel *widget.Label
	durLabel     *widget.Label
	timeline     *widget.Slider
}

func BuildView(opts Options, initialPath string) fyne.CanvasObject {
	t := i18n.T()
	trimColor := opts.ModuleColor
	navyBlue := utils.MustHex("#191F35")
	gridColor := utils.MustHex("#171C2A")

	state := &trimState{
		player: media.NewVideoPlayer(),
	}

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

	// --- UI Components ---
	state.timeLabel = widget.NewLabel("00:00:00.000")
	state.durLabel = widget.NewLabel("00:00:00.000")
	state.inPointLabel = widget.NewLabel(t.TrimInPoint + ": 00:00:00.000")
	state.outPointLabel = widget.NewLabel(t.TrimOutPoint + ": 00:00:00.000")
	
	state.timeline = widget.NewSlider(0, 100)
	state.timeline.OnChanged = func(val float64) {
		if state.engine != nil {
			target := (val / 100.0) * state.duration
			state.engine.Seek(target)
			state.currentTime = target
			state.updateTimeLabels()
			// Fetch a frame immediately for feedback
			if img, err := state.engine.NextFrame(); err == nil {
				state.player.SetFrame(img)
			}
		}
	}

	setInBtn := widget.NewButton(t.TrimSetIn, func() {
		state.inPoint = time.Duration(state.currentTime * float64(time.Second))
		state.inPointLabel.SetText(fmt.Sprintf("%s: %s", t.TrimInPoint, formatDuration(state.inPoint)))
	})
	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		state.outPoint = time.Duration(state.currentTime * float64(time.Second))
		state.outPointLabel.SetText(fmt.Sprintf("%s: %s", t.TrimOutPoint, formatDuration(state.outPoint)))
	})
	
	// Step buttons for frame-accuracy
	stepBackBtn := widget.NewButton("<", func() {
		if state.engine != nil {
			// FFmpeg seek back is trickier, for now we seek back slightly and Step forward
			target := state.currentTime - 0.033 // rough 1 frame at 30fps
			if target < 0 { target = 0 }
			state.engine.Seek(target)
			if img, err := state.engine.NextFrame(); err == nil {
				state.player.SetFrame(img)
			}
		}
	})
	stepFwdBtn := widget.NewButton(">", func() {
		if state.engine != nil {
			if img, err := state.engine.Step(1); err == nil {
				state.player.SetFrame(img)
				// Update clock or current time based on frame PTS...
			}
		}
	})

	playBtn := widget.NewButton(t.ActionPlay, func() {
		if state.engine != nil {
			state.engine.Start()
			go state.playbackLoop()
		}
	})

	transport := container.NewHBox(
		layout.NewSpacer(),
		stepBackBtn, playBtn, stepFwdBtn,
		widget.NewSeparator(),
		setInBtn, setOutBtn,
		layout.NewSpacer(),
	)

	videoContainer := container.NewBorder(
		nil,
		container.NewVBox(
			container.NewBorder(nil, nil, state.timeLabel, state.durLabel, state.timeline),
			transport,
		),
		nil, nil,
		container.NewMax(canvas.NewRectangle(color.Black), state.player),
	)

	// Layout from design spec
	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleTrim), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance
	topBar := ui.TintedBar(trimColor, container.NewHBox(backBtn, layout.NewSpacer()))

	rightSide := container.NewVBox(
		buildTrimBox("Selection", container.NewVBox(
			state.inPointLabel,
			state.outPointLabel,
		)),
	)

	content := container.NewHSplit(videoContainer, container.NewPadded(rightSide))
	content.Offset = 0.8

	// Auto-load if path provided
	if initialPath != "" {
		state.loadVideo(initialPath)
	}

	return container.NewBorder(topBar, nil, nil, nil, content)
}

func (s *trimState) loadVideo(path string) {
	s.engine = media.NewEngine()
	if err := s.engine.Open(path); err != nil {
		logging.Error(logging.CatPlayer, "Trim: failed to open %s: %v", path, err)
		return
	}
	s.duration = s.engine.Duration()
	s.durLabel.SetText(formatDuration(time.Duration(s.duration * float64(time.Second))))
	
	// Show first frame
	if img, err := s.engine.NextFrame(); err == nil {
		s.player.SetFrame(img)
	}
}

func (s *trimState) playbackLoop() {
	for {
		img, err := s.engine.NextFrame()
		if err != nil {
			break
		}
		s.player.SetFrame(img)
		// Update timeline/timeLabel...
	}
}

func (s *trimState) updateTimeLabels() {
	s.timeLabel.SetText(formatDuration(time.Duration(s.currentTime * float64(time.Second))))
}

func formatDuration(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	ms := (d % time.Second) / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
