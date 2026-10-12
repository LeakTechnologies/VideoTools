//go:build native_media

package media

import (
	"context"
	"errors"
	"image"
	"io"
	"testing"
	"time"
)

func awaitFrameError(t *testing.T, result <-chan error, want error) {
	t.Helper()
	select {
	case err := <-result:
		if !errors.Is(err, want) {
			t.Fatalf("frame read returned %v, want %v", err, want)
		}
	case <-time.After(time.Second):
		t.Fatal("frame read did not exit after cancellation/retirement")
	}
}

func TestFrameReadCancellation(t *testing.T) {
	for _, paused := range []bool{false, true} {
		for _, retire := range []bool{false, true} {
			t.Run(fmtCase(paused, retire), func(t *testing.T) {
				e := NewEngine()
				e.paused = paused
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result := make(chan error, 1)
				go func() { _, err := e.NextFrameContext(ctx); result <- err }()
				// Exercise cancellation of an already pending read, rather than
				// only entry with an already cancelled context.
				time.Sleep(20 * time.Millisecond)
				want := error(context.Canceled)
				if retire {
					close(e.stop)
					want = ErrEngineClosed
				} else {
					cancel()
				}
				awaitFrameError(t, result, want)
			})
		}
	}
}

func fmtCase(paused, retired bool) string {
	name := "playing"
	if paused {
		name = "paused"
	}
	if retired {
		return name + "/retired"
	}
	return name + "/cancelled"
}

func TestFrameReadRetirementDuringPTSWait(t *testing.T) {
	e := NewEngine()
	e.clock.SetPaused(false)
	e.frameQueue <- decodedFrame{pts: 3600, img: image.NewRGBA(image.Rect(0, 0, 2, 2))}
	result := make(chan error, 1)
	go func() { _, err := e.NextFrame(); result <- err }()
	time.Sleep(20 * time.Millisecond)
	close(e.stop)
	awaitFrameError(t, result, ErrEngineClosed)
}

func TestFrameReadNaturalEOF(t *testing.T) {
	e := NewEngine()
	e.frameQueue <- decodedFrame{pts: decodeEOFPTS}
	_, err := e.NextFrame()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("natural EOF returned %v", err)
	}
}

func TestClockWaitCancellation(t *testing.T) {
	c := NewMasterClock()
	c.SetPaused(false)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- c.WaitForPTSContext(ctx, 3600) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	awaitFrameError(t, result, context.Canceled)
}

func TestClockWaitTracksSpeed(t *testing.T) {
	c := NewMasterClock()
	c.SetSpeed(4)
	c.SetPaused(false)
	start := time.Now()
	if err := c.WaitForPTSContext(context.Background(), 0.4); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("4x wait took %s for 0.4 media seconds", elapsed)
	}
	if c.GetTime() < 0.4 {
		t.Fatal("wait returned before target presentation time")
	}
}

func TestCloseBeforeStartRetiresEngine(t *testing.T) {
	e := NewEngine()
	e.Close()
	e.Close()
	e.Start()
	if e.IsRunning() {
		t.Fatal("retired engine restarted")
	}
	_, err := e.NextFrame()
	if !errors.Is(err, ErrEngineClosed) {
		t.Fatalf("closed engine returned %v", err)
	}
}

func TestPausedAudioCloseKeepsStopSignal(t *testing.T) {
	p := &AudioPlayer{paused: true, decodeStop: make(chan struct{}), pcmCh: make(chan audioChunk), queue: NewPacketQueue()}
	p.decodeWg.Add(1)
	go p.audioDecodeLoop()
	time.Sleep(20 * time.Millisecond)
	done := make(chan struct{})
	go func() { p.Close(); p.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("paused audio decoder did not finish Close")
	}
	select {
	case <-p.decodeStop:
	default:
		t.Fatal("Close lost the decoder stop signal")
	}
}

func TestScrubberRepeatedStartStop(t *testing.T) {
	s := NewSmoothScrubbing(NewEngine())
	s.Start()
	s.Start()
	for i := 0; i < 20; i++ {
		s.RequestSeek(float64(i))
	}
	done := make(chan struct{})
	go func() { s.Stop(); s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scrubber repeated start/stop did not retire")
	}
}
