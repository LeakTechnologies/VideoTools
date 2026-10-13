package media

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	// TargetSampleRate is the output sample rate used by all audio players.
	TargetSampleRate = 48000
	// TargetChannels is the number of output channels (stereo).
	TargetChannels = 2
	// The device context and per-player output queue are each bounded to
	// 50ms. Oto's default per-player queue can hold roughly half a second,
	// which must not be treated as if only the device buffer exists.
	audioBufferSize = 50 * time.Millisecond
	// AudioBufferLatency includes both configured output queues. The reader
	// applies playback speed when converting this physical latency to media time.
	// OS/device latency beyond those queues still requires runtime measurement.
	audioPlayerBufferDuration = 50 * time.Millisecond
	audioPlayerBufferBytes    = int(audioPlayerBufferDuration * time.Duration(TargetSampleRate*TargetChannels*2) / time.Second)
	AudioBufferLatency        = audioBufferSize + audioPlayerBufferDuration
)

var sharedOtoCtx struct {
	once sync.Once
	ctx  *oto.Context
	err  error
}

// GetSharedAudioContext returns the process-wide oto audio context, creating it on first call.
// oto only supports one Context per process; all players must share this instance.
func GetSharedAudioContext() (*oto.Context, error) {
	sharedOtoCtx.once.Do(func() {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   TargetSampleRate,
			ChannelCount: TargetChannels,
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   audioBufferSize,
		})
		if err == nil && ready != nil {
			select {
			case <-ready:
			case <-time.After(10 * time.Second):
				// Audio device init timed out — continue without audio rather than hanging forever.
				err = fmt.Errorf("audio device init timed out")
				ctx = nil
			}
		}
		sharedOtoCtx.ctx = ctx
		sharedOtoCtx.err = err
	})
	return sharedOtoCtx.ctx, sharedOtoCtx.err
}
