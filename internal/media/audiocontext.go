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
	// audioBufferSize is the oto hardware output buffer duration.
	// 100ms is low enough that A/V desync is imperceptible while still
	// providing enough headroom against underruns on typical hardware.
	// pcmChannelCap (64 × ~23ms ≈ 1.5s) provides the upstream buffer, so
	// the OS buffer only needs to be small.
	// 50ms strikes a balance between low latency and avoiding underruns.
	// Can reduce further if the system handles it.
	audioBufferSize = 50 * time.Millisecond
	// AudioBufferLatency is the nominal end-to-end audio output latency
	// (oto hardware buffer).  Used to compensate the master clock so that
	// video PTS is displayed when the corresponding audio samples actually
	// reach the speakers rather than when they are fed to the OS buffer.
	AudioBufferLatency = audioBufferSize
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
