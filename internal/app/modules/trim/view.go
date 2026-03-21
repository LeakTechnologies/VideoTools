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

	videoPath string
	inPoint   time.Duration
	outPoint  time.Duration

	currentTime float64
	duration    float64

	mode   string // "keep" or "cut"
	export string // "copy" or "reencode"

	inPointLabel  *widget.Label
	outPointLabel *widget.Label
	fileLabel     *widget.Label
	addBtn        *widget.Button
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
		ts.inPoint = time.Duration(ts.currentTime * float64(time.Second))
		ts.inPointLabel.SetText(t.TrimInPoint + ": " + formatDuration(ts.inPoint))
	})

	setOutBtn := widget.NewButton(t.TrimSetOut, func() {
		ts.outPoint = time.Duration(ts.currentTime * float64(time.Second))
		ts.outPointLabel.SetText(t.TrimOutPoint + ": " + formatDuration(ts.outPoint))
	})

	clearBtn := widget.NewButton(t.TrimClear, func() {
		ts.inPoint = 0
		ts.outPoint = time.Duration(ts.duration * float64(time.Second))
		ts.inPointLabel.SetText(t.TrimInPoint + ": " + formatDuration(ts.inPoint))
		ts.outPointLabel.SetText(t.TrimOutPoint + ": " + formatDuration(ts.outPoint))
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
		clip := TrimClip{
			Path:     ts.videoPath,
			InPoint:  ts.inPoint,
			OutPoint: ts.outPoint,
			Duration: time.Duration(ts.duration * float64(time.Second)),
			Mode:     ts.mode,
			Export:   ts.export,
		}
		if opts.OnAddToQueue != nil {
			opts.OnAddToQueue(clip)
		}
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
