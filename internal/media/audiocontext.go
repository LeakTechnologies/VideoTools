package media

import (
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	// TargetSampleRate is the output sample rate used by all audio players.
	TargetSampleRate = 48000
	// TargetChannels is the number of output channels (stereo).
	TargetChannels = 2
	// audioBufferSize is the oto context buffer duration.
	// 500ms provides smooth playback with sufficient headroom against underruns.
	// The old value of 170ms was too small and caused stuttering on slower systems.
	audioBufferSize = 500 * time.Millisecond
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
			<-ready
		}
		sharedOtoCtx.ctx = ctx
		sharedOtoCtx.err = err
	})
	return sharedOtoCtx.ctx, sharedOtoCtx.err
}
