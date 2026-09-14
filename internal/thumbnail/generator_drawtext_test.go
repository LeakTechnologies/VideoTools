package thumbnail

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFakeFFmpeg(t *testing.T, hasDrawtext bool) string {
	t.Helper()
	dir := t.TempDir()
	name := "ffmpeg"
	// ffmpeg's real `-filters` output line is:
	//   T. drawtext   V->V   Draw text on top of video frames using libfreetype library.
	// The fake must not contain '>' because cmd.exe would treat it as a redirect;
	// the probe's fallback matches any line containing "drawtext".
	echoLine := ` T. drawtext  V-V  Draw text on top of video frames.` + "\n"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.cmd"
		echoLine = "echo " + ` T. drawtext  V-V  Draw text on top of video frames.` + "\r\n"
		echoLine = "@echo off\r\n" + echoLine
	}
	if !hasDrawtext {
		if runtime.GOOS == "windows" {
			echoLine = "@echo off\r\n"
		} else {
			echoLine = "#!/bin/sh\n"
		}
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(echoLine), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDrawtextAvailableDetected(t *testing.T) {
	hasPath := writeFakeFFmpeg(t, true)
	g := NewGenerator(hasPath)
	if !g.drawtextAvailable() {
		t.Fatal("expected drawtext availability to be detected")
	}
	if !g.drawtextProbed {
		t.Fatal("expected probe result to be cached")
	}
}

func TestDrawtextAvailableMissing(t *testing.T) {
	missingPath := writeFakeFFmpeg(t, false)
	g := NewGenerator(missingPath)
	if g.drawtextAvailable() {
		t.Fatal("expected drawtext to be reported missing")
	}
	if !g.drawtextProbed {
		t.Fatal("expected probe result to be cached")
	}
}

func TestDrawtextAvailableExecError(t *testing.T) {
	g := NewGenerator(filepath.Join(t.TempDir(), "does-not-exist"))
	if g.drawtextAvailable() {
		t.Fatal("expected missing executable to report drawtext as unavailable")
	}
}

func TestBuildThumbFilterSkipsDrawtextWhenUnavailable(t *testing.T) {
	g := NewGenerator(writeFakeFFmpeg(t, false))
	// Force probe state to avoid a second exec; matches the missing-filter case.
	g.drawtextProbed = true
	g.drawtextOK = false

	filter := g.buildThumbFilter(320, 240, true, 12.5)
	if strings.Contains(filter, "drawtext") {
		t.Fatalf("timestamp drawtext must be skipped when drawtext is unavailable, got: %s", filter)
	}
	if !strings.HasPrefix(filter, "setsar=1") {
		t.Fatalf("base scale/pad filter expected, got: %s", filter)
	}
}

func TestBuildThumbFilterKeepsDrawtextWhenAvailable(t *testing.T) {
	g := NewGenerator(writeFakeFFmpeg(t, true))
	if !g.drawtextAvailable() {
		t.Fatal("fake ffmpeg should report drawtext present")
	}
	filter := g.buildThumbFilter(320, 240, true, 0)
	if !strings.Contains(filter, "drawtext") {
		t.Fatalf("timestamp drawtext expected when available, got: %s", filter)
	}
}