//go:build native_media

package ui

import (
	"image"
	"io"
	"testing"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/media"
)

type retiredReadEngine struct {
	media.PlaybackEngine
	entered chan struct{}
	release chan struct{}
	result  error
}

func (e *retiredReadEngine) NextFrame() (*image.RGBA, error) {
	close(e.entered)
	<-e.release
	return image.NewRGBA(image.Rect(0, 0, 2, 2)), e.result
}

func TestPlaybackDiscardsRetiredRead(t *testing.T) {
	for _, result := range []error{nil, io.EOF, media.ErrEngineClosed} {
		old := &retiredReadEngine{entered: make(chan struct{}), release: make(chan struct{}), result: result}
		replacement := &retiredReadEngine{}
		player := &InlineVideoPlayer{engine: old, playing: true}
		done := make(chan struct{})
		go func() { defer close(done); player.playbackLoop() }()
		select {
		case <-old.entered:
		case <-time.After(time.Second):
			t.Fatal("playback did not enter frame read")
		}
		player.mu.Lock()
		player.engine = replacement
		player.mu.Unlock()
		close(old.release)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("retired playback read did not exit")
		}
		player.mu.Lock()
		stillPlaying := player.playing
		player.mu.Unlock()
		if !stillPlaying {
			t.Fatal("retired read stopped replacement playback")
		}
	}
}
