package rip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeProbe runs verifyRipOutput with a canned ffprobe response, so the file
// checks and the video-stream detection are testable without real media.
func fakeProbe(t *testing.T, size int, codec string, probeErr error) error {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.mkv")
	if err := os.WriteFile(out, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}

	var logged []string
	opts := ExecuteOptions{
		Format: FormatH264MKV,
		OnRunCommand: func(name string, args []string, logFn func(string)) error {
			if probeErr != nil {
				return probeErr
			}
			logFn(codec)
			return nil
		},
	}
	return verifyRipOutput(opts, out, func(s string) { logged = append(logged, s) })
}

func TestVerifyRipOutputRejectsTrivialFile(t *testing.T) {
	if err := fakeProbe(t, 512, "h264", nil); err == nil {
		t.Fatal("expected error for a sub-1024-byte output")
	}
}

func TestVerifyRipOutputRejectsMissingFile(t *testing.T) {
	opts := ExecuteOptions{Format: FormatH264MKV}
	if err := verifyRipOutput(opts, filepath.Join(t.TempDir(), "nope.mkv"), func(string) {}); err == nil {
		t.Fatal("expected error for a missing output")
	}
}

func TestVerifyRipOutputRejectsNoVideoStream(t *testing.T) {
	// File exists and is large enough, but ffprobe reports no video stream.
	if err := fakeProbe(t, 2048, "", nil); err == nil {
		t.Fatal("expected error when no video stream is detected")
	}
}

func TestVerifyRipOutputPasses(t *testing.T) {
	if err := fakeProbe(t, 2048, "mpeg2video", nil); err != nil {
		t.Fatalf("expected success with an mpeg2 video stream: %v", err)
	}
}

func TestVerifyRipOutputErrorTextIsActionable(t *testing.T) {
	err := fakeProbe(t, 2048, "", nil)
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(err.Error(), "no video stream") {
		t.Fatalf("error should say why the output is rejected: %v", err)
	}
}
