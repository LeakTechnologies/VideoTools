//go:build native_media

package ui

import (
	"errors"
	"image"
	"testing"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/media"
)

type resumePreviewEngine struct {
	media.PlaybackEngine
	position float64
	timeout  time.Duration
	seekErr  error
	grabErr  error
	frame    *image.RGBA
}

func (e *resumePreviewEngine) Seek(position float64) error { e.position = position; return e.seekErr }
func (e *resumePreviewEngine) GrabFrame(timeout time.Duration) (*image.RGBA, error) {
	e.timeout = timeout
	return e.frame, e.grabErr
}
func (e *resumePreviewEngine) NextFrame() (*image.RGBA, error) {
	panic("frame queue has no producer before first Resume")
}

func TestResumePreviewBeforePlayback(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	e := &resumePreviewEngine{frame: frame}
	got, err := grabResumePreview(e, 45.61)
	if err != nil || got != frame || e.position != 45.61 || e.timeout != 4*time.Second {
		t.Fatalf("resume preview: frame=%p err=%v position=%v timeout=%v", got, err, e.position, e.timeout)
	}
	seekErr := errors.New("seek failed")
	e = &resumePreviewEngine{seekErr: seekErr}
	if _, err := grabResumePreview(e, 45.61); !errors.Is(err, seekErr) || e.timeout != 0 {
		t.Fatal("failed seek must return without decoding")
	}
	grabErr := errors.New("preview timed out")
	e = &resumePreviewEngine{grabErr: grabErr}
	if _, err := grabResumePreview(e, 45.61); !errors.Is(err, grabErr) {
		t.Fatal("preview failure must return to loading caller")
	}
}
