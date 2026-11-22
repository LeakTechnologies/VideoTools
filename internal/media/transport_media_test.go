//go:build native_media

package media

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in real-media check. A generated neutral fixture is supplied by the test
// run; normal package checks do not depend on private media or an encoder.
func TestTransportMediaSeekAfterEOF(t *testing.T) {
	path := os.Getenv("VT_TRANSPORT_TEST_MEDIA")
	if path == "" {
		t.Skip("set VT_TRANSPORT_TEST_MEDIA to a short generated A/V fixture")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine()
	defer e.Close()
	e.SetHWDevice(HWDeviceNone)
	if err := e.Open(path); err != nil {
		t.Fatal(err)
	}
	e.SetMuted(true)
	e.Start()
	duration := e.Duration()
	if duration < 2 || duration > 15 {
		t.Fatal("use a 2-15 second neutral A/V fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := e.Seek(duration - 1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.NextFrameContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.PreparePlayback(ctx, time.Second); err != nil {
		t.Fatal(err)
	}
	e.Resume()
	reachedEOF := false
	for i := 0; i < 100; i++ {
		_, err := e.NextFrameContext(ctx)
		if errors.Is(err, io.EOF) {
			reachedEOF = true
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reachedEOF {
		t.Fatal("fixture did not reach natural EOF")
	}
	e.Pause()
	target := 0.5
	if err := e.Seek(target); err != nil {
		t.Fatal(err)
	}
	readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	if _, err := e.NextFrameContext(readCtx); err != nil {
		t.Fatalf("seek after EOF failed: %v", err)
	}
	if pts := e.GetLastVideoPTS(); math.Abs(pts-target) > 1/e.GetFrameRate()+0.001 {
		t.Fatalf("post-EOF target %.3f returned %.3f", target, pts)
	}
	if err := e.PreparePlayback(ctx, time.Second); err != nil {
		t.Fatalf("post-EOF A/V readiness: %v", err)
	}
	e.Resume()
	for i := 0; i < 5; i++ {
		if _, err := e.NextFrameContext(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTransportMediaAlternatingSeeksAndRates(t *testing.T) {
	path := os.Getenv("VT_TRANSPORT_TEST_MEDIA")
	if path == "" {
		t.Skip("set VT_TRANSPORT_TEST_MEDIA to a short generated A/V fixture")
	}
	e := NewEngine()
	defer e.Close()
	e.SetHWDevice(HWDeviceNone)
	if err := e.Open(path); err != nil {
		t.Fatal(err)
	}
	e.SetMuted(true)
	e.SetDiagnosticsEnabled(true)
	e.Start()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, target := range []float64{5, 1, 6, 0.5} {
		e.Pause()
		if err := e.Seek(target); err != nil {
			t.Fatal(err)
		}
		if _, err := e.NextFrameContext(ctx); err != nil {
			t.Fatal(err)
		}
		if pts := e.GetLastVideoPTS(); math.Abs(pts-target) > 1/e.GetFrameRate()+0.001 {
			t.Fatalf("target %.3f got %.3f", target, pts)
		}
		if err := e.PreparePlayback(ctx, time.Second); err != nil {
			t.Fatal(err)
		}
		e.Resume()
		for _, rate := range []float64{1, 1.5, 1} {
			e.SetSpeed(rate)
			for i := 0; i < 5; i++ {
				if _, err := e.NextFrameContext(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if p := e.audioPlayer; p != nil {
				p.mu.Lock()
				tempo := 1.0
				if p.filterGraph != nil {
					tempo = p.filterGraph.GetTempo()
				}
				p.mu.Unlock()
				if tempo != rate {
					t.Fatalf("requested rate %.2f audio tempo %.2f", rate, tempo)
				}
			}
		}
	}
	t.Logf("final diagnostics: %+v", e.Diagnostics())
}
