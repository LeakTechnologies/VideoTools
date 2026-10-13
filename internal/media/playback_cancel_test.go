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

func TestResumePreservesQueuedFramesAndSeekState(t *testing.T) {
	e := NewEngine()
	e.running = true
	e.paused = true
	e.pausedAtomic.Store(true)
	e.decodeLoopActive = true // no native decoder is needed for this transport test
	e.decodeEOFSent = true
	e.seekFlushBefore.Store(123)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	e.frameQueue <- decodedFrame{img: img, pts: 1}
	e.Resume()
	if len(e.frameQueue) != 1 {
		t.Fatal("Resume discarded queued frames")
	}
	if !e.decodeEOFSent || e.seekFlushBefore.Load() != 123 {
		t.Fatal("Resume reset source/seek state")
	}
	e.Close()
}
func TestCancelledPresentationRetainsFrame(t *testing.T) {
	e := NewEngine()
	e.clock.SetPaused(false)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	e.frameQueue <- decodedFrame{img: img, pts: 100}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := e.NextFrameContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read returned %v", err)
	}
	e.clock.ResetTime(100)
	e.clock.SetPaused(true)
	actual, err := e.NextFrame()
	if err != nil || actual != img {
		t.Fatal("cancelled presentation skipped its frame")
	}
	e.Close()
}
func TestPausedAudioReadPreservesPCMAndLeftover(t *testing.T) {
	p := &AudioPlayer{paused: true, speed: 1, volumeMul: 1, pcmCh: make(chan audioChunk, 1), leftover: []byte{9, 8}}
	p.pcmCh <- audioChunk{data: []byte{1, 2, 3, 4}}
	buf := make([]byte, 4)
	if n, err := p.Read(buf); err != nil || n != 4 {
		t.Fatal("paused read failed")
	}
	if len(p.leftover) != 2 || len(p.pcmCh) != 1 {
		t.Fatal("paused callback discarded buffered audio")
	}
	for _, b := range buf {
		if b != 0 {
			t.Fatal("paused read was not silent")
		}
	}
}
func TestPausedFullFrameQueueDoesNotDiscardDecodedFrame(t *testing.T) {
	e := NewEngine()
	e.paused = true
	for i := 0; i < cap(e.frameQueue); i++ {
		e.frameQueue <- decodedFrame{pts: float64(i)}
	}
	sent := make(chan bool, 1)
	go func() { sent <- e.sendToFrameQueue(decodedFrame{pts: 999}) }()
	select {
	case <-sent:
		t.Fatal("decoder discarded a frame while paused queue was full")
	case <-time.After(30 * time.Millisecond):
	}
	<-e.frameQueue
	select {
	case ok := <-sent:
		if !ok {
			t.Fatal("decoder did not enqueue retained frame")
		}
	case <-time.After(time.Second):
		t.Fatal("retained decoded frame did not enqueue")
	}
	found := false
	for len(e.frameQueue) > 0 {
		if (<-e.frameQueue).pts == 999 {
			found = true
		}
	}
	if !found {
		t.Fatal("retained decoded frame is missing")
	}
	e.Close()
}

func TestAudioSeekPrerollDoesNotExpandIntoSilence(t *testing.T) {
	p := &AudioPlayer{speed: 1, volumeMul: 1, clock: NewMasterClock(), pcmCh: make(chan audioChunk, 4)}
	p.SetSeekTarget(10)
	p.pcmCh <- audioChunk{pts: 8, data: []byte{9, 9, 9, 9}}
	p.pcmCh <- audioChunk{pts: 9, data: []byte{9, 9, 9, 9}}
	p.pcmCh <- audioChunk{pts: 10, data: []byte{1, 2, 3, 4}}
	buf := make([]byte, 4)
	n, err := p.Read(buf)
	if err != nil || n != 4 {
		t.Fatalf("read: n=%d err=%v", n, err)
	}
	for i, b := range []byte{1, 2, 3, 4} {
		if buf[i] != b {
			t.Fatalf("preroll returned silence/old audio: %v", buf)
		}
	}
	if p.seekGuardActive.Load() {
		t.Fatal("valid audio did not clear seek guard")
	}
	if p.GetLastPTS() != 10 {
		t.Fatal("audio timestamp did not reach target")
	}
}

func TestPlaybackPreparationUsesReadyBuffers(t *testing.T) {
	e := NewEngine()
	e.running = true
	e.paused = true
	e.pausedAtomic.Store(true)
	e.frameQueue <- decodedFrame{pts: 0.1}
	if err := e.PreparePlayback(context.Background(), time.Second); err != nil {
		t.Fatal(err)
	}
	if !e.IsPaused() {
		t.Fatal("buffer preparation started output clock")
	}
	if e.prebuffering.Load() {
		t.Fatal("buffer preparation left priming active")
	}
	e.Close()
}
func TestPlaybackPreparationIsCancelled(t *testing.T) {
	e := NewEngine()
	e.running = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.PreparePlayback(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("preparation returned %v", err)
	}
	if e.prebuffering.Load() {
		t.Fatal("cancelled preparation kept priming active")
	}
	e.Close()
}

func TestPreparationKeepsDemuxReadingPastQueuedVideo(t *testing.T) {
	e := NewEngine()
	defer e.Close()
	e.pausedAtomic.Store(true)
	if !e.demuxShouldWait() {
		t.Fatal("idle paused demux must park")
	}
	e.prebuffering.Store(true)
	// A queued video packet must not strand audio later in the mux.
	e.videoQueue.mu.Lock()
	e.videoQueue.packets = append(e.videoQueue.packets, nil)
	e.videoQueue.mu.Unlock()
	if e.demuxShouldWait() {
		t.Fatal("preparation parks demux before reaching audio")
	}
	e.prebuffering.Store(false)
	if !e.demuxShouldWait() {
		t.Fatal("demux did not return to paused demand mode")
	}
}
