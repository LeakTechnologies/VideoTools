//go:build native_media

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type InlineVideoPlayer struct {
	player  *media.VideoPlayer
	engine  *media.Engine
	playing bool
}

func NewInlineVideoPlayer() *InlineVideoPlayer {
	return &InlineVideoPlayer{
		player: media.NewInlineVideoPlayer(),
	}
}

func (v *InlineVideoPlayer) Widget() *media.VideoPlayer {
	return v.player
}

func (v *InlineVideoPlayer) Load(path string) error {
	v.player.ClearError()
	v.player.SetLoading(true)

	if v.engine != nil {
		v.engine.Close()
	}

	v.engine = media.NewEngine()
	v.engine.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	v.engine.SetDropFrames(true)

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: opening %s", path)
	if err := v.engine.Open(path); err != nil {
		logging.Error(logging.CatPlayer, "InlineVideoPlayer: failed to open %s: %v", path, err)
		v.player.SetError(err.Error())
		v.player.SetLoading(false)
		return err
	}

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: file opened successfully")

	v.engine.InitFrameCache(30)

	duration := v.engine.Duration()
	v.player.SetDuration(duration)

	if img, err := v.engine.NextFrame(); err == nil {
		v.player.SetFrame(img)
	}

	v.player.SetLoading(false)
	return nil
}

func (v *InlineVideoPlayer) Play() {
	if v.engine == nil {
		return
	}
	v.playing = true
	v.engine.Start()
	go v.playbackLoop()
}

func (v *InlineVideoPlayer) Pause() {
	if v.engine == nil {
		return
	}
	v.playing = false
	v.engine.Pause()
}

func (v *InlineVideoPlayer) Seek(target float64) {
	if v.engine == nil {
		return
	}
	v.engine.Seek(target)
	if img, err := v.engine.NextFrame(); err == nil {
		v.player.SetFrame(img)
	}
	v.player.SetCurrentTime(target)
}

func (v *InlineVideoPlayer) SetSpeed(speed float64) {
	if v.engine == nil {
		return
	}
	v.engine.SetSpeed(speed)
}

func (v *InlineVideoPlayer) StepFrame(dir int) {
	if v.engine == nil {
		return
	}
	v.playing = false
	v.engine.Pause()
	if img, err := v.engine.Step(dir); err == nil {
		v.player.SetFrame(img)
	}
}

func (v *InlineVideoPlayer) Close() {
	v.playing = false
	if v.engine != nil {
		v.engine.Close()
		v.engine = nil
	}
}

func (v *InlineVideoPlayer) playbackLoop() {
	defer logging.RecoverPanic()
	defer logging.LogAllGoroutines()

	for v.playing {
		img, err := v.engine.NextFrame()
		if err != nil {
			return
		}
		v.player.SetFrame(img)
		currentTime := v.engine.CurrentTime()
		v.player.SetCurrentTime(currentTime)
	}
}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	player := NewInlineVideoPlayer()

	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.SetMinSize(size)

	container := container.NewMax(bg, player.Widget())

	return container, player
}
