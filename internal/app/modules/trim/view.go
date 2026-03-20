//go:build native_media

package trim

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media/state"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type Options struct {
	Window         fyne.Window
	ModuleColor    color.Color
	OnShowMainMenu func()
	OnShowQueue    func()
	OnAddToQueue   func(clip TrimClip)
}

type TrimClip struct {
	Path     string
	InPoint  time.Duration
	OutPoint time.Duration
	Mode     string
	Export   string
}

type trimState struct {
	engine      *media.Engine
	player      *media.VideoPlayer
	resumeState *state.ResumeState

	inPoint  time.Duration
	outPoint time.Duration

	currentTime float64
	duration    float64

	inPointLabel  *widget.Label
	outPointLabel *widget.Label
}

func BuildView(opts Options, initialPath string) fyne.CanvasObject {
	t := i18n.T()
	trimColor := opts.ModuleColor
	navyBlue := utils.MustHex("#191F35")
	gridColor := utils.MustHex("#171C2A")

	resume, err := state.NewResumeState("")
	if err != nil {
		logging.Warning(logging.CatPlayer, "Failed to init resume state: %v", err)
	}

	state := &trimState{
		player:      media.NewVideoPlayer(),
		resumeState: resume,
	}

	state.player.OnPlay(func() {
		if state.engine != nil {
			state.engine.Start()
			go state.playbackLoop()
		}
	})

	state.player.OnPause(func() {
		if state.engine != nil {
			state.engine.Pause()
		}
	})

	state.player.OnSeek(func(target float64) {
		if state.engine != nil {
			state.engine.Seek(target)
			state.currentTime = target
		}
	})

	state.player.OnSpeedChange(func(speed float64) {
		if state.engine != nil {
			state.engine.SetSpeed(speed)
		}
	})

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

	state.inPointLabel = widget.NewLabel(t.TrimInPoint + ": 00:00:00.000")
	state.outPointLabel = widget.NewLabel(t.TrimOutPoint + ": 00:00:00.000")

	setInBtn := widget.NewButton(t.TrimSetIn, func() {
		state.inPoint = time.Duration(state.currentTime * float64(time.Second))
		state.inPointLabel.SetText(t.TrimInPoint + ": " + formatDuration(state.inPoint))
	})
	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		state.outPoint = time.Duration(state.currentTime * float64(time.Second))
		state.outPointLabel.SetText(t.TrimOutPoint + ": " + formatDuration(state.outPoint))
	})

	stepBackBtn := widget.NewButton("<", func() {
		if state.engine != nil {
			target := state.currentTime - 0.033
			if target < 0 {
				target = 0
			}
			state.engine.Seek(target)
			if img, err := state.engine.NextFrame(); err == nil {
				state.player.SetFrame(img)
				state.currentTime = target
				state.player.SetCurrentTime(target)
			}
		}
	})

	stepFwdBtn := widget.NewButton(">", func() {
		if state.engine != nil {
			if img, err := state.engine.Step(1); err == nil {
				state.player.SetFrame(img)
			}
		}
	})

	toolbar := container.NewHBox(
		stepBackBtn,
		setInBtn,
		setOutBtn,
		stepFwdBtn,
		layout.NewSpacer(),
	)

	videoContainer := container.NewMax(
		canvas.NewRectangle(color.Black),
		state.player,
	)

	rightSide := container.NewVBox(
		buildTrimBox("Selection", container.NewVBox(
			state.inPointLabel,
			state.outPointLabel,
		)),
		layout.NewSpacer(),
		toolbar,
	)

	content := container.NewHSplit(videoContainer, container.NewPadded(rightSide))
	content.Offset = 0.8

	if initialPath != "" {
		state.loadVideo(initialPath)
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleTrim), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance
	topBar := ui.TintedBar(trimColor, container.NewHBox(backBtn, layout.NewSpacer()))

	return container.NewBorder(topBar, nil, nil, nil, content)
}

func (s *trimState) loadVideo(path string) {
	defer logging.RecoverPanicWithCallback(func() {
		s.player.SetLoading(false)
	})

	s.player.ClearError()
	s.player.SetLoading(true)
	s.engine = media.NewEngine()
	s.engine.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	s.engine.SetDropFrames(true)

	logging.Info(logging.CatPlayer, "Trim loadVideo: opening %s", path)
	if err := s.engine.Open(path); err != nil {
		logging.Error(logging.CatPlayer, "Trim loadVideo: failed to open %s: %v", path, err)
		s.player.SetLoading(false)
		s.player.SetError(fmt.Sprintf("Failed to open: %v", err))
		return
	}
	logging.Info(logging.CatPlayer, "Trim loadVideo: file opened successfully")

	s.duration = s.engine.Duration()
	s.player.SetDuration(s.duration)

	chapters := s.engine.GetChapters()
	if len(chapters) > 0 {
		s.player.SetChapters(chapters)
	}

	// Check for saved playback position
	var resumePos float64
	if s.resumeState != nil {
		if savedPos, ok := s.resumeState.GetPosition(path); ok && s.resumeState.ShouldResume(savedPos) {
			resumePos = savedPos.Position
			logging.Info(logging.CatPlayer, "Found saved position: %.2f seconds", resumePos)
		}
	}

	if img, err := s.engine.NextFrame(); err == nil {
		s.player.SetFrame(img)
	}
	s.player.SetLoading(false)

	// Start background thumbnail extraction
	s.engine.StartThumbnailExtraction(func(t float64, thumb *image.RGBA) {
		s.player.AddThumbnailFrame(t, thumb)
	})

	// Seek to saved position if found
	if resumePos > 0 {
		s.engine.Seek(resumePos)
		s.currentTime = resumePos
		s.player.SetCurrentTime(resumePos)
		if img, err := s.engine.NextFrame(); err == nil {
			s.player.SetFrame(img)
		}
	}
}

func (s *trimState) playbackLoop() {
	defer logging.RecoverPanic()
	defer logging.LogAllGoroutines()

	saveTicker := time.NewTicker(5 * time.Second)
	defer saveTicker.Stop()
	var currentPath string

	for {
		select {
		case <-saveTicker.C:
			if s.engine != nil && currentPath != "" {
				pos := s.engine.CurrentTime()
				dur := s.engine.Duration()
				if s.resumeState != nil && dur > 0 {
					s.resumeState.SavePosition(currentPath, pos, dur)
				}
			}
		default:
			img, err := s.engine.NextFrame()
			if err != nil {
				return
			}
			s.player.SetFrame(img)
			s.currentTime = s.engine.CurrentTime()
			s.player.SetCurrentTime(s.currentTime)
		}
	}
}

func (s *trimState) savePlaybackPosition(path string) {
	if s.resumeState != nil && s.engine != nil && s.duration > 0 {
		s.resumeState.SavePosition(path, s.currentTime, s.duration)
	}
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
