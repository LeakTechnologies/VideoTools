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
	"fyne.io/fyne/v2/driver/desktop"
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
	engine      *media.Engine
	player      *media.VideoPlayer
	resumeState *state.ResumeState
	keyCapture  *keyboardCapture

	videoPath string
	inPoint   time.Duration
	outPoint  time.Duration

	currentTime float64
	duration    float64

	mode   string // "keep" or "cut"
	export string // "copy" or "reencode"

	inPointLabel  *widget.Label
	outPointLabel *widget.Label
	durationLabel *widget.Label
	fileLabel     *widget.Label
	addBtn        *widget.Button
}

type keyboardCapture struct {
	widget.BaseWidget
	onKey func(event *desktop.KeyEvent)
}

func (k *keyboardCapture) Tapped(*fyne.PointEvent)          {}
func (k *keyboardCapture) TappedSecondary(*fyne.PointEvent) {}

func (k *keyboardCapture) KeyDown(event *desktop.KeyEvent) {
	if k.onKey != nil {
		k.onKey(event)
	}
}

func (k *keyboardCapture) SetOnKey(onKey func(event *desktop.KeyEvent)) {
	k.onKey = onKey
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

	ts := &trimState{
		player:      media.NewVideoPlayer(),
		resumeState: resume,
		mode:        "keep",
		export:      "copy",
	}

	ts.player.OnPlay(func() {
		if ts.engine != nil {
			ts.engine.Start()
			go ts.playbackLoop()
		}
	})

	ts.player.OnPause(func() {
		if ts.engine != nil {
			ts.engine.Pause()
		}
	})

	ts.player.OnSeek(func(target float64) {
		if ts.engine != nil {
			ts.engine.Seek(target)
			ts.currentTime = target
		}
	})

	ts.player.OnSpeedChange(func(speed float64) {
		if ts.engine != nil {
			ts.engine.SetSpeed(speed)
		}
	})

	// Initialize keyboard capture
	ts.keyCapture = &keyboardCapture{}
	ts.keyCapture.ExtendBaseWidget(ts.keyCapture)
	ts.keyCapture.SetOnKey(func(event *desktop.KeyEvent) {
		if ts.engine == nil || ts.videoPath == "" {
			return
		}

		switch event.Key {
		case desktop.KeyI:
			ts.setInPoint()
		case desktop.KeyO:
			ts.setOutPoint()
		case desktop.KeyC:
			ts.clearPoints()
		case desktop.KeyLeft:
			if event.Modifier&desktop.ShiftModifier != 0 {
				// Shift+Left: jump back 1 second
				ts.seekRelative(-1.0)
			} else {
				// Left: step back 1 frame
				ts.stepFrame(-1)
			}
		case desktop.KeyRight:
			if event.Modifier&desktop.ShiftModifier != 0 {
				// Shift+Right: jump forward 1 second
				ts.seekRelative(1.0)
			} else {
				// Right: step forward 1 frame
				ts.stepFrame(1)
			}
		case desktop.KeySpace:
			ts.togglePlayPause()
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

	// In/out point labels
	ts.inPointLabel = widget.NewLabel(t.TrimInPoint + ": 00:00:00.000")
	ts.outPointLabel = widget.NewLabel(t.TrimOutPoint + ": 00:00:00.000")
	ts.durationLabel = widget.NewLabel(t.TrimDuration + ": 00:00:00.000")

	// Frame stepping
	stepBackBtn := widget.NewButton("<", func() {
		if ts.engine != nil {
			target := ts.currentTime - 0.033
			if target < 0 {
				target = 0
			}
			ts.engine.Seek(target)
			if img, err := ts.engine.NextFrame(); err == nil {
				ts.player.SetFrame(img)
				ts.currentTime = target
				ts.player.SetCurrentTime(target)
			}
		}
	})

	stepFwdBtn := widget.NewButton(">", func() {
		if ts.engine != nil {
			if img, err := ts.engine.Step(1); err == nil {
				ts.player.SetFrame(img)
			}
		}
	})

	// Set In / Set Out
	setInBtn := widget.NewButton(t.TrimSetIn, func() {
		ts.setInPoint()
	})

	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		ts.setOutPoint()
	})

	clearBtn := widget.NewButton(t.TrimClear, func() {
		ts.clearPoints()
	})

	// Mode selector
	modeSelect := widget.NewSelect([]string{t.TrimModeKeep, t.TrimModeCut}, func(s string) {
		if s == t.TrimModeCut {
			ts.mode = "cut"
		} else {
			ts.mode = "keep"
		}
	})
	modeSelect.SetSelected(t.TrimModeKeep)

	// Export selector
	exportSelect := widget.NewSelect([]string{t.TrimSmartCopy, t.TrimRecode}, func(s string) {
		if s == t.TrimRecode {
			ts.export = "reencode"
		} else {
			ts.export = "copy"
		}
	})
	exportSelect.SetSelected(t.TrimSmartCopy)

	// Add to Queue
	ts.addBtn = widget.NewButton(t.MenuQueue, func() {
		if ts.videoPath == "" {
			dialog.ShowInformation(t.DialogNoVideo, "Please load a video first.", opts.Window)
			return
		}
		if ts.outPoint <= ts.inPoint {
			dialog.ShowInformation(t.TrimInvalidSelection, "Out point must be after in point.", opts.Window)
			return
		}
		// Warn about smart copy keyframe limitations
		if ts.export == "copy" {
			dialog.ShowConfirm(t.TrimSmartCopyWarningTitle, t.TrimSmartCopyWarning, func(confirmed bool) {
				if !confirmed {
					return
				}
				ts.doAddToQueue(opts)
			}, opts.Window)
			return
		}
		ts.doAddToQueue(opts)
	})
	ts.addBtn.Importance = widget.HighImportance
	ts.addBtn.Disable()

	// File name label — updated when a video is loaded
	fileLabel := widget.NewLabel(func() string {
		if initialPath != "" {
			return filepath.Base(initialPath)
		}
		return t.TrimInstructions[:0] + "No file loaded"
	}())
	fileLabel.Wrapping = fyne.TextTruncate
	ts.fileLabel = fileLabel

	// Browse button
	openBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFileOpen(func(f fyne.URIReadCloser, err error) {
			if err != nil || f == nil {
				return
			}
			path := f.URI().Path()
			f.Close()
			ts.loadVideo(path)
		}, opts.Window)
	})

	// Toolbar row under the player
	toolbar := container.NewHBox(
		stepBackBtn,
		setInBtn,
		setOutBtn,
		stepFwdBtn,
		layout.NewSpacer(),
	)

	videoContainer := container.NewMax(
		canvas.NewRectangle(color.Black),
		ts.player,
	)

	// Left: video + toolbar + selection labels
	leftSide := container.NewVBox(
		container.NewVBox(videoContainer),
		toolbar,
		buildTrimBox(t.TrimInPoint+" / "+t.TrimOutPoint, container.NewVBox(
			ts.inPointLabel,
			ts.outPointLabel,
			ts.durationLabel,
		)),
	)

	// Right: settings + actions
	rightSide := container.NewVBox(
		buildTrimBox(t.ModuleTrim+" "+t.ModuleSettings, container.NewVBox(
			widget.NewLabel(t.TrimMode),
			modeSelect,
			widget.NewLabel(t.TrimOutput),
			exportSelect,
		)),
		layout.NewSpacer(),
		buildTrimBox(t.ActionBrowse, container.NewVBox(
			fileLabel,
			openBtn,
		)),
		container.NewHBox(clearBtn, layout.NewSpacer(), ts.addBtn),
	)

	content := container.NewHSplit(leftSide, container.NewPadded(rightSide))
	content.Offset = 0.72

	if initialPath != "" {
		ts.loadVideo(initialPath)
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleTrim), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance
	topBar := ui.TintedBar(trimColor, container.NewHBox(backBtn, layout.NewSpacer()))

	return container.NewBorder(topBar, nil, nil, nil, container.NewMax(s.keyCapture, content))
}

func (s *trimState) setInPoint() {
	if s.engine == nil || s.videoPath == "" {
		return
	}
	s.inPoint = time.Duration(s.currentTime * float64(time.Second))
	if s.inPointLabel != nil {
		s.inPointLabel.SetText(i18n.T().TrimInPoint + ": " + formatDuration(s.inPoint))
	}
	if s.player != nil {
		s.player.SetInPoint(s.currentTime)
	}
	s.updateDurationLabel()
}

func (s *trimState) setOutPoint() {
	if s.engine == nil || s.videoPath == "" {
		return
	}
	s.outPoint = time.Duration(s.currentTime * float64(time.Second))
	if s.outPointLabel != nil {
		s.outPointLabel.SetText(i18n.T().TrimOutPoint + ": " + formatDuration(s.outPoint))
	}
	if s.player != nil {
		s.player.SetOutPoint(s.currentTime)
	}
	s.updateDurationLabel()
}

func (s *trimState) clearPoints() {
	if s.engine == nil {
		return
	}
	s.inPoint = 0
	s.outPoint = time.Duration(s.duration * float64(time.Second))
	if s.inPointLabel != nil {
		s.inPointLabel.SetText(i18n.T().TrimInPoint + ": " + formatDuration(s.inPoint))
	}
	if s.outPointLabel != nil {
		s.outPointLabel.SetText(i18n.T().TrimOutPoint + ": " + formatDuration(s.outPoint))
	}
	if s.player != nil {
		s.player.ClearTrimMarkers()
	}
	s.updateDurationLabel()
}

func (s *trimState) updateDurationLabel() {
	if s.durationLabel != nil && s.outPoint > s.inPoint {
		regionDur := s.outPoint - s.inPoint
		durText := fmt.Sprintf("%s: %s", i18n.T().TrimDuration, formatDuration(regionDur))
		s.durationLabel.SetText(durText)
	}
}

func (s *trimState) doAddToQueue(opts Options) {
	clip := TrimClip{
		Path:     s.videoPath,
		InPoint:  s.inPoint,
		OutPoint: s.outPoint,
		Duration: time.Duration(s.duration * float64(time.Second)),
		Mode:     s.mode,
		Export:   s.export,
	}
	if opts.OnAddToQueue != nil {
		opts.OnAddToQueue(clip)
	}
}

func (s *trimState) stepFrame(dir int) {
	if s.engine == nil {
		return
	}
	if img, err := s.engine.Step(dir); err == nil {
		s.player.SetFrame(img)
		s.currentTime = s.engine.CurrentTime()
		s.player.SetCurrentTime(s.currentTime)
	}
}

func (s *trimState) seekRelative(seconds float64) {
	if s.engine == nil {
		return
	}
	target := s.currentTime + seconds
	if target < 0 {
		target = 0
	}
	if target > s.duration {
		target = s.duration
	}
	s.engine.Seek(target)
	if img, err := s.engine.NextFrame(); err == nil {
		s.player.SetFrame(img)
		s.currentTime = target
		s.player.SetCurrentTime(s.currentTime)
	}
}

func (s *trimState) togglePlayPause() {
	if s.engine == nil {
		return
	}
	if s.player != nil && s.player.IsPlaying() {
		s.engine.Pause()
	} else {
		s.engine.Start()
		go s.playbackLoop()
	}
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

	s.engine.InitFrameCache(30)
	s.videoPath = path
	s.duration = s.engine.Duration()
	if s.fileLabel != nil {
		s.fileLabel.SetText(filepath.Base(path))
	}
	s.player.SetDuration(s.duration)
	s.player.SetFrameRate(s.engine.GetFrameRate())

	// Default out point to end of file
	s.outPoint = time.Duration(s.duration * float64(time.Second))
	if s.outPointLabel != nil {
		s.outPointLabel.SetText(i18n.T().TrimOutPoint + ": " + formatDuration(s.outPoint))
	}
	if s.inPointLabel != nil {
		s.inPointLabel.SetText(i18n.T().TrimInPoint + ": " + formatDuration(0))
	}
	// Update player trim markers
	s.player.SetInPoint(0)
	s.player.SetOutPoint(s.duration)
	s.updateDurationLabel()

	chapters := s.engine.GetChapters()
	if len(chapters) > 0 {
		s.player.SetChapters(chapters)
	}

	// Check for saved playback position
	var resumePos float64
	if s.resumeState != nil {
		if savedPos, ok := s.resumeState.GetPosition(path); ok && s.resumeState.ShouldResume(savedPos) {
			resumePos = savedPos.Position
		}
	}

	if img, err := s.engine.NextFrame(); err == nil {
		s.player.SetFrame(img)
	}
	s.player.SetLoading(false)

	// Background thumbnail extraction
	s.engine.StartThumbnailExtraction(func(t float64, thumb *image.RGBA) {
		s.player.AddThumbnailFrame(t, thumb)
	})

	if resumePos > 0 {
		s.engine.Seek(resumePos)
		s.currentTime = resumePos
		s.player.SetCurrentTime(resumePos)
		if img, err := s.engine.NextFrame(); err == nil {
			s.player.SetFrame(img)
		}
	}

	if s.addBtn != nil {
		s.addBtn.Enable()
	}
}

func (s *trimState) playbackLoop() {
	defer logging.RecoverPanic()
	defer logging.LogAllGoroutines()

	saveTicker := time.NewTicker(5 * time.Second)
	defer saveTicker.Stop()

	for {
		select {
		case <-saveTicker.C:
			if s.engine != nil && s.videoPath != "" {
				pos := s.engine.CurrentTime()
				dur := s.engine.Duration()
				if s.resumeState != nil && dur > 0 {
					s.resumeState.SavePosition(s.videoPath, pos, dur)
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
	sec := d / time.Second
	ms := (d % time.Second) / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, ms)
}
