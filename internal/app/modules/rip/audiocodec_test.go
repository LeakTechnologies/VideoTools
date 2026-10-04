package rip

import (
	"errors"
	"strings"
	"testing"
)

// TestIsMuxerCodecError pins the classification that decides whether a failed rip
// retries on the same input with a substituted audio codec, or falls back to a
// different input. A DVD title carrying pcm_dvd cannot be stream-copied into
// MKV, and retrying it down the VOB-concat path fails identically while masking
// the real cause — so that class must be recognised from stderr.
func TestIsMuxerCodecError(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   bool
	}{
		{
			name:   "matroska rejects pcm_dvd at header write (observed on ffmpeg 8.1)",
			stderr: "[matroska @ 000001be06d6cb80] No wav codec tag found for codec pcm_dvd\n[out#0/matroska @ 000001be06d70e40] Could not write header (incorrect codec parameters ?): Invalid argument",
			want:   true,
		},
		{
			name:   "invalid audio codec phrasing",
			stderr: "Could not write header for output file #0 (incorrect codec parameters ?): Invalid audio codec. Defaulting to no audio.",
			want:   true,
		},
		{
			name:   "older invalid audio codec phrasing",
			stderr: "[matroska @ 0000] Invalid audio codec",
			want:   true,
		},
		{
			name:   "generic unsupported codec",
			stderr: "Could not find tag for codec pcm_dvd in stream #1, codec not currently supported in container",
			want:   true,
		},
		{
			name:   "unsupported codec id",
			stderr: "Unsupported codec id in input",
			want:   true,
		},
		{
			name:   "case insensitive",
			stderr: "COULD NOT WRITE HEADER for output file #0",
			want:   true,
		},
		{
			name:   "dvdvideo demux failure is NOT a codec rejection",
			stderr: "libdvdnav: no open_pgc() call found",
			want:   false,
		},
		{
			name:   "css auth failure is NOT a codec rejection",
			stderr: "libdvdcss: css certificate authentication failed",
			want:   false,
		},
		{
			name:   "unrelated demux noise",
			stderr: "Packet corrupt (stream 0, length: 0)",
			want:   false,
		},
		{
			// "invalid argument" and "codec" appear all over ffmpeg output; a bare
			// match on either would misclassify a demux failure and trigger a
			// pointless re-encode retry. Only the combined muxer-tag phrasing
			// counts.
			name:   "generic invalid argument alone is not a codec rejection",
			stderr: "Conversion failed! Invalid argument",
			want:   false,
		},
		{
			name:   "empty stderr cannot be classified",
			stderr: "",
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := error(&ffmpegError{err: errors.New("exit status 1"), stderr: tc.stderr})
			if got := isMuxerCodecError(err); got != tc.want {
				t.Errorf("isMuxerCodecError(%q) = %v, want %v", tc.stderr, got, tc.want)
			}
		})
	}
}

// TestIsMuxerCodecErrorIgnoresPlainErrors ensures the classifier only inspects
// ffmpeg's own stderr. An unrelated error carrying similar wording (or any
// non-ffmpeg error) must not trigger a codec-substitution retry.
func TestIsMuxerCodecErrorIgnoresPlainErrors(t *testing.T) {
	if isMuxerCodecError(errors.New("Could not write header: Invalid audio codec")) {
		t.Error("a plain error must not be classified as a muxer codec failure")
	}
	if isMuxerCodecError(nil) {
		t.Error("nil must not be classified as a muxer codec failure")
	}
}

// TestFFmpegErrorUnwrap keeps the process error reachable through errors.As /
// errors.Is, so existing callers that inspect the exit status still work.
func TestFFmpegErrorUnwrap(t *testing.T) {
	sentinel := errors.New("exit status 1")
	err := error(&ffmpegError{err: sentinel, stderr: "boom"})

	if !errors.Is(err, sentinel) {
		t.Error("errors.Is must reach the wrapped process error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to include stderr for diagnosis", err.Error())
	}

	var fe *ffmpegError
	if !errors.As(err, &fe) {
		t.Fatal("errors.As must recover the typed error")
	}
	if fe.Stderr() != "boom" {
		t.Errorf("Stderr() = %q, want %q", fe.Stderr(), "boom")
	}
}

// TestBuildRipArgsAudioEncoder pins the argument shape behind the retry. A bare
// "-c copy" would force an all-or-nothing stream copy, so audio must be split
// out with -c:v copy / -c:s copy / -c:a <encoder>; a default rip keeps audio
// copied, and the retry substitutes only the audio encoder.
func TestBuildRipArgsAudioEncoder(t *testing.T) {
	base := RipArgs{
		ListFile:    "list.txt",
		OutputPath:  "out.mkv",
		Format:      FormatLosslessMKV,
		AudioLangs:  []string{"eng"},
		MaxDuration: 1800,
	}

	t.Run("default copies every stream", func(t *testing.T) {
		args := BuildRipArgs(base)
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-c:v copy") {
			t.Errorf("args = %v, want -c:v copy", args)
		}
		if !strings.Contains(joined, "-c:a copy") {
			t.Errorf("args = %v, want -c:a copy", args)
		}
		if strings.Contains(joined, "-c copy") {
			t.Errorf("args = %v, must not use a bare \"-c copy\" (blocks audio substitution)", args)
		}
	})

	t.Run("retry substitutes audio only, video stays copied", func(t *testing.T) {
		ra := base
		ra.AudioEncoder = "flac"
		args := BuildRipArgs(ra)
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-c:a flac") {
			t.Errorf("args = %v, want -c:a flac", args)
		}
		if !strings.Contains(joined, "-c:v copy") {
			t.Errorf("args = %v, want -c:v copy preserved (video must not be re-encoded)", args)
		}
		if strings.Contains(joined, "-c:a copy") {
			t.Errorf("args = %v, must not also copy audio", args)
		}
	})

	t.Run("h264 mkv honours the override", func(t *testing.T) {
		ra := base
		ra.Format = FormatH264MKV
		args := BuildRipArgs(ra)
		if !strings.Contains(strings.Join(args, " "), "-c:a copy") {
			t.Errorf("args = %v, want -c:a copy by default", args)
		}

		ra.AudioEncoder = "flac"
		joined := strings.Join(BuildRipArgs(ra), " ")
		if !strings.Contains(joined, "-c:a flac") {
			t.Errorf("args = %v, want -c:a flac", joined)
		}
		if !strings.Contains(joined, "-c:v libx264") {
			t.Errorf("args = %v, want -c:v libx264 preserved", joined)
		}
	})

	t.Run("h264 mp4 ignores the override (already lossy aac)", func(t *testing.T) {
		ra := base
		ra.Format = FormatH264MP4
		ra.AudioEncoder = "flac"
		joined := strings.Join(BuildRipArgs(ra), " ")
		if strings.Contains(joined, "-c:a flac") {
			t.Errorf("args = %v, MP4 already re-encodes audio to aac; override must not apply", joined)
		}
		if !strings.Contains(joined, "-c:a aac") {
			t.Errorf("args = %v, want -c:a aac", joined)
		}
	})
}
