package rip

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipWithoutFFmpeg gates tests that drive a real ffmpeg binary. The rip
// executor's failure classification is defined by ffmpeg's stderr text, so it
// cannot be verified against a stub: the markers must match what ffmpeg really
// prints.
func skipWithoutFFmpeg(t *testing.T) string {
	t.Helper()
	exe := os.Getenv("VT_TEST_FFMPEG")
	if exe == "" {
		p, err := exec.LookPath("ffmpeg")
		if err != nil {
			t.Skip("ffmpeg not on PATH; set VT_TEST_FFMPEG to run")
		}
		exe = p
	}
	return exe
}

// skipWithoutFFprobe gates the codec assertions: they need ffprobe to read back
// the produced file's actual codecs.
func skipWithoutFFprobe(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH; cannot assert produced codecs")
	}
	return p
}

// makePCMDVDVOB builds a synthetic VOB whose audio track is pcm_dvd — the
// 20-bit LPCM codec a DVD title carries that the Matroska muxer cannot store.
// No disc image is needed: the defect is a muxer/codec incompatibility, and the
// VOB produced here triggers exactly the same failure as a ripped title.
func makePCMDVDVOB(t *testing.T, ffmpeg, path string) {
	t.Helper()
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=2",
		"-c:v", "mpeg2video", "-b:v", "800k",
		"-c:a", "pcm_dvd",
		"-f", "vob", path,
	}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("synthesize pcm_dvd VOB: %v\n%s", err, out)
	}
}

// TestRipAudioCodecRetryEndToEnd is the end-to-end proof for the pcm_dvd lossless
// rip defect, using a real ffmpeg against a synthetic DVD-shaped VOB.
//
// Before the fix, the default lossless path stream-copied the audio and the run
// died at the muxer ("No wav codec tag found for codec pcm_dvd", exit 1) — and
// the VOB-concat fallback then failed identically while hiding the real cause.
// This test pins all three halves of the fix:
//
//  1. the default stream-copy attempt still fails (the defect is real),
//  2. that failure is classified as a codec rejection, not a demux error,
//  3. substituting FLAC completes the rip with video still stream-copied.
func TestRipAudioCodecRetryEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ffmpeg integration test in short mode")
	}
	ffmpeg := skipWithoutFFmpeg(t)
	ffprobe := skipWithoutFFprobe(t)

	dir := t.TempDir()
	vob := filepath.Join(dir, "sample.vob")
	out := filepath.Join(dir, "out.mkv")
	makePCMDVDVOB(t, ffmpeg, vob)

	// Drive the real VOB-concat input path: a single-entry concat list is
	// exactly what the concat fallback feeds ffmpeg.
	list := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(list, []byte("file '"+strings.ReplaceAll(vob, `\`, "/")+"'\n"), 0o644); err != nil {
		t.Fatalf("write concat list: %v", err)
	}

	ra := RipArgs{
		ListFile:    list,
		OutputPath:  out,
		Format:      FormatLosslessMKV,
		AudioLangs:  []string{"eng"},
		MaxDuration: 0,
	}

	// 1 + 2: the default attempt fails, and it fails as a codec rejection.
	err := runRipOnce(context.Background(), ffmpeg, BuildRipArgs(ra), nil)
	if err == nil {
		t.Fatal("expected the pcm_dvd stream-copy rip to fail at the muxer")
	}
	if !isMuxerCodecError(err) {
		t.Fatalf("failure not classified as a codec rejection, so the FLAC retry would never fire: %v", err)
	}

	// 3: substituting FLAC completes the rip.
	ra.AudioEncoder = "flac"
	if err := runRipOnce(context.Background(), ffmpeg, BuildRipArgs(ra), nil); err != nil {
		t.Fatalf("FLAC audio retry failed: %v", err)
	}

	assertStreamCodecs(t, ffprobe, out, "mpeg2video", "flac")
}

// runRipOnce runs a built argument set the way the executor does, with progress
// plumbing, so the returned error is the same typed error Execute classifies.
func runRipOnce(ctx context.Context, ffmpeg string, args []string, logFn func(string)) error {
	if logFn == nil {
		logFn = func(string) {}
	}
	args = append(args[:len(args)-1:len(args)-1], append([]string{"-progress", "pipe:1", "-nostats"}, args[len(args)-1])...)
	return runFFmpegWithProgress(ctx, ffmpeg, args, 0, nil, logFn, nil)
}

// assertStreamCodecs checks the produced file's actual codecs — the point of the
// fix is that audio is repacked losslessly while video is bit-for-bit copied, so
// asserting the codecs is the assertion that matters.
func assertStreamCodecs(t *testing.T, ffprobe, path, wantVideo, wantAudio string) {
	t.Helper()
	codecName := func(selector string) string {
		out, err := exec.Command(ffprobe, "-v", "error", "-select_streams", selector,
			"-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
		if err != nil {
			t.Fatalf("ffprobe %s: %v", selector, err)
		}
		return strings.TrimSpace(string(out))
	}

	if got := codecName("v:0"); got != wantVideo {
		t.Errorf("video codec = %q, want %q (video must stay stream-copied)", got, wantVideo)
	}
	if got := codecName("a:0"); got != wantAudio {
		t.Errorf("audio codec = %q, want %q", got, wantAudio)
	}
}
