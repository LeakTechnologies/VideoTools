package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/LeakTechnologies/VideoTools/internal/convert"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/interlace"
)

// TestNormalizeLoudnessFromJob guards the silent-zero class of bug: a queued
// job whose loudness target is missing or unusable must never reach ffmpeg as 0,
// because a loudnorm filter built from 0 targets 0 LUFS and 0 dBTP.
func TestNormalizeLoudnessFromJob(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]interface{}
		key  string
		def  float64
		want float64
	}{
		{
			name: "present float64 is used",
			cfg:  map[string]interface{}{"normalizeLUFS": -14.0},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -14.0,
		},
		{
			name: "absent key falls back to default",
			cfg:  map[string]interface{}{"audioCodec": "AAC"},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "nil map falls back to default",
			cfg:  nil,
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "explicit zero is repaired",
			cfg:  map[string]interface{}{"normalizeLUFS": 0.0},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "wrong type falls back to default",
			cfg:  map[string]interface{}{"normalizeLUFS": "-14"},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "true peak default",
			cfg:  map[string]interface{}{"normalizeTruePeak": -2.0},
			key:  "normalizeTruePeak",
			def:  -1.5,
			want: -2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLoudnessFromJob(tt.cfg, tt.key, tt.def); got != tt.want {
				t.Errorf("normalizeLoudnessFromJob(%v, %q, %v) = %v, want %v",
					tt.cfg, tt.key, tt.def, got, tt.want)
			}
		})
	}
}

// TestNormalizeLoudnessFromJobJSONRoundTrip mirrors how the queue persists jobs:
// JSON numbers decode into interface{} as float64. A job enqueued by a build
// predating the loudness fields has no key at all and must still resolve to a
// usable target rather than 0.
func TestNormalizeLoudnessFromJobJSONRoundTrip(t *testing.T) {
	cfg := map[string]interface{}{"normalizeLUFS": -14.0, "normalizeTruePeak": -2.0}

	if got := normalizeLoudnessFromJob(cfg, "normalizeLUFS", -16.0); got != -14.0 {
		t.Errorf("enqueued value = %v, want -14", got)
	}

	legacy := map[string]interface{}{"audioCodec": "AAC"}
	if got := normalizeLoudnessFromJob(legacy, "normalizeLUFS", -16.0); got != -16.0 {
		t.Errorf("legacy job = %v, want default -16", got)
	}
}

// TestFormatPresetCodecRoundTrip guards the #13 fall-through: every format
// preset's codec spelling must resolve to a UI codec name, and that UI name
// must map back to the preset's own encoder (AV1 excepted - its encoder is
// resolved per environment). A preset that resolves to no UI codec silently
// falls through to libx264 or fails at the muxer, which is exactly how
// MOV (ProRes) and OGG (Theora) shipped broken.
func TestFormatPresetCodecRoundTrip(t *testing.T) {
	for _, opt := range convert.FormatOptions {
		if opt.VideoCodec == "" || strings.EqualFold(opt.VideoCodec, "copy") {
			continue // passthrough formats; callers special-case copy
		}
		friendly := friendlyCodecFromPreset(opt.VideoCodec)
		if friendly == "" {
			t.Errorf("format %q: preset codec %q resolves to no UI codec", opt.Label, opt.VideoCodec)
			continue
		}
		if strings.EqualFold(friendly, "AV1") {
			continue // encoder is environment-resolved (svtav1/aom/hw)
		}
		// Hardware accel is environment-resolved (this test host has NVENC);
		// pin it off so the software round-trip is exact.
		enc := determineVideoCodec(convertConfig{VideoCodec: friendly, HardwareAccel: "none"})
		if enc != opt.VideoCodec {
			t.Errorf("format %q: preset %q -> UI %q -> encoder %q (want %q)",
				opt.Label, opt.VideoCodec, friendly, enc, opt.VideoCodec)
		}
	}
}

// TestVideoCodecUIOptions pins the select vocabulary: every canonical codec
// plus Copy, in table order, and every entry must be resolvable both ways.
func TestVideoCodecUIOptions(t *testing.T) {
	opts := videoCodecUIOptions()
	want := []string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "ProRes", "Theora", "Copy"}
	if len(opts) != len(want) {
		t.Fatalf("videoCodecUIOptions() = %v, want %v", opts, want)
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Errorf("option %d = %q, want %q", i, opts[i], want[i])
		}
	}
	for _, o := range opts[:len(opts)-1] { // all but Copy
		if videoCodecIdentityForUI(o) == nil {
			t.Errorf("select option %q has no canonical identity", o)
		}
	}
}

// TestFriendlyCodecFromPreset covers preset, encoder, hardware, and legacy
// spellings, including the two that previously fell through.
func TestFriendlyCodecFromPreset(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"libx264", "H.264"}, {"h264_nvenc", "H.264"}, {"avc1", "H.264"}, {"H.264", "H.264"},
		{"libx265", "H.265"}, {"hevc_amf", "H.265"}, {"HEVC", "H.265"},
		{"libvpx-vp9", "VP9"},
		{"libaom-av1", "AV1"}, {"av1_qsv", "AV1"},
		{"mpeg2video", "MPEG-2"},
		// "MPEG-2" itself resolves to "" — the hyphen breaks the contiguous
		// "mpeg2" alias, exactly as in the original substring mapping. Not a
		// regression: friendlyCodecFromPreset only ever receives preset/encoder
		// spellings ("mpeg2video"), never UI names.
		{"MPEG-2", ""},
		{"prores_ks", "ProRes"}, // previously "" (the #13 defect)
		{"libtheora", "Theora"}, // previously "" (the #13 defect)
		{"copy", ""},            // passthrough; callers special-case it
		{"", ""},
		{"mpeg4", ""}, // deliberately unmapped: not a supported output codec
	}
	for _, tt := range tests {
		if got := friendlyCodecFromPreset(tt.in); got != tt.want {
			t.Errorf("friendlyCodecFromPreset(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDetermineVideoCodec covers the deterministic paths: table codecs, the
// Copy passthrough, legacy raw spellings, and the unknown-value default.
// Hardware and AV1 resolution is environment-probed and excluded.
func TestDetermineVideoCodec(t *testing.T) {
	tests := []struct {
		codec, accel, want string
	}{
		{"H.264", "none", "libx264"},
		{"H.265", "none", "libx265"},
		{"VP9", "", "libvpx-vp9"},
		{"MPEG-2", "", "mpeg2video"},
		{"ProRes", "", "prores_ks"},      // previously fell to libx264
		{"Theora", "", "libtheora"},      // previously fell to libx264
		{"mpeg2video", "", "mpeg2video"}, // legacy raw spelling
		{"Copy", "", "copy"},
		{"Nonsense", "", "libx264"}, // unknown default
	}
	for _, tt := range tests {
		got := determineVideoCodec(convertConfig{VideoCodec: tt.codec, HardwareAccel: tt.accel})
		if got != tt.want {
			t.Errorf("determineVideoCodec(%q, accel=%q) = %q, want %q", tt.codec, tt.accel, got, tt.want)
		}
	}
}

// TestHwEncoderFor pins the acceleration->encoder mapping for the H.26x pair.
func TestHwEncoderFor(t *testing.T) {
	tests := []struct {
		accel, family, want string
	}{
		{"nvenc", "h264", "h264_nvenc"},
		{"amf", "hevc", "hevc_amf"},
		{"qsv", "h264", "h264_qsv"},
		{"videotoolbox", "hevc", "hevc_videotoolbox"},
		{"none", "h264", ""},
		{"", "hevc", ""},
	}
	for _, tt := range tests {
		if got := hwEncoderFor(tt.accel, tt.family); got != tt.want {
			t.Errorf("hwEncoderFor(%q, %q) = %q, want %q", tt.accel, tt.family, got, tt.want)
		}
	}
}

// TestCodecFollowsFormat pins the format-vs-user-choice policy: the H.26x
// cross-pair swap, user-override survival for generic codecs, and the
// codec-specific (ProRes/Theora) preset rules in both directions.
func TestCodecFollowsFormat(t *testing.T) {
	tests := []struct {
		name, current, friendly string
		want                    bool
	}{
		{"empty current adopts format codec", "", "H.264", true},
		{"no implied codec", "H.264", "", false},
		{"H.26x cross swap 264->265", "H.264", "H.265", true},
		{"H.26x cross swap 265->264", "H.265", "H.264", true},
		{"user VP9 survives generic format", "VP9", "H.264", false},
		{"user MPEG-2 survives generic format", "MPEG-2", "H.264", false},
		{"equal codec is a no-op", "H.264", "H.264", false},
		{"entering codec-specific preset", "H.264", "ProRes", true},
		{"entering codec-specific preset over VP9", "VP9", "ProRes", true},
		{"entering Theora preset", "H.264", "Theora", true},
		{"leaving codec-specific preset", "ProRes", "H.264", true},
		{"leaving Theora preset", "Theora", "H.264", true},
		{"staying on ProRes is a no-op", "ProRes", "ProRes", false},
	}
	for _, tt := range tests {
		if got := codecFollowsFormat(tt.current, tt.friendly); got != tt.want {
			t.Errorf("%s: codecFollowsFormat(%q, %q) = %v, want %v",
				tt.name, tt.current, tt.friendly, got, tt.want)
		}
	}
}

// --- Interlace analysis: shared operation + generation claims (#21 / #14) ---

// TestInterlaceClaimLifecycle pins the claim semantics: a fresh claim is
// valid; a source transition, a newer dispatch, and a clear all invalidate it.
func TestInterlaceClaimLifecycle(t *testing.T) {
	s := &appState{}
	claim := s.beginConvertInterlaceAnalysis()
	if !claim.valid() {
		t.Fatal("a freshly captured claim must be valid")
	}
	if !s.interlaceAnalyzing || s.interlaceResult != nil {
		t.Fatal("beginConvertInterlaceAnalysis must mark the slot analyzing and clear the result")
	}

	// A newer dispatch invalidates the previous claim.
	claim2 := s.beginConvertInterlaceAnalysis()
	if claim.valid() {
		t.Fatal("a newer dispatch must invalidate the previous claim")
	}
	if !claim2.valid() {
		t.Fatal("the newest claim must be valid")
	}

	// A source transition invalidates even the newest claim and clears the slot.
	s.setConvertSource(&videoSource{Path: "next.mkv"})
	if claim2.valid() {
		t.Fatal("a source transition must invalidate in-flight claims")
	}
	if s.interlaceAnalyzing {
		t.Fatal("a source transition must leave the slot not-analyzing")
	}
	if s.interlaceResult != nil {
		t.Fatal("a source transition must clear the published result")
	}
}

// TestInterlaceClaimRoundTripABA guards the acceptance case a path comparison
// alone would fail: after A -> B -> A, the first A's claim must stay invalid -
// the generation moved on, even though the path is back to A.
func TestInterlaceClaimRoundTripABA(t *testing.T) {
	s := &appState{}
	claimA1 := s.beginConvertInterlaceAnalysis()

	s.setConvertSource(&videoSource{Path: "a.mkv"})
	s.setConvertSource(&videoSource{Path: "b.mkv"})
	s.setConvertSource(&videoSource{Path: "a.mkv"}) // path returns to A

	if claimA1.valid() {
		t.Fatal("first A's claim must stay invalid after A -> B -> A; a path-only check would wrongly accept it")
	}

	claimA2 := s.beginConvertInterlaceAnalysis()
	if !claimA2.valid() {
		t.Fatal("a fresh dispatch under the current generation must be valid")
	}
}

// TestInterlaceSlotsIndependent guards Convert and Inspect retaining
// independent result state: a transition in one slot must not invalidate an
// in-flight analysis of the other.
func TestInterlaceSlotsIndependent(t *testing.T) {
	s := &appState{}

	inspectClaim := s.resetInspectInterlace()
	s.setConvertSource(nil) // Convert transition
	if !inspectClaim.valid() {
		t.Fatal("a Convert source transition must not invalidate an in-flight Inspect analysis")
	}

	convertClaim := s.beginConvertInterlaceAnalysis()
	s.clearInspectInterlace() // Inspect clear
	if !convertClaim.valid() {
		t.Fatal("an Inspect clear must not invalidate an in-flight Convert analysis")
	}
}

// TestInspectInterlaceClearLifecycle pins the Inspect slot semantics: the
// load reset marks analyzing, the clear does not, and both clear the result.
func TestInspectInterlaceClearLifecycle(t *testing.T) {
	s := &appState{}
	s.inspectInterlaceResult = &interlace.DetectionResult{Status: "Interlaced"}

	claim := s.resetInspectInterlace()
	if !claim.valid() || !s.inspectInterlaceAnalyzing || s.inspectInterlaceResult != nil {
		t.Fatal("resetInspectInterlace must clear the result, mark analyzing, and return a valid claim")
	}

	s.inspectInterlaceResult = &interlace.DetectionResult{Status: "Interlaced"}
	s.clearInspectInterlace()
	if s.inspectInterlaceAnalyzing || s.inspectInterlaceResult != nil {
		t.Fatal("clearInspectInterlace must clear the result without marking analyzing")
	}
	if claim.valid() {
		t.Fatal("clearInspectInterlace must invalidate in-flight claims")
	}
}

// requireTestFFmpeg skips when the PATH lacks an ffmpeg/ffprobe pair (the
// detector falls back to PATH executables; QuickAnalyze is unrunnable
// without them).
func requireTestFFmpeg(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH - interlace analysis is not runnable here", tool)
		}
	}
}

// newInterlaceTestSource generates a tiny synthetic progressive source with
// the PATH ffmpeg. Self-contained: no fixture files ship with the repo.
func newInterlaceTestSource(t *testing.T) string {
	t.Helper()
	requireTestFFmpeg(t)
	src := filepath.Join(t.TempDir(), "src.mov")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=160x120:rate=10:duration=0.5",
		"-c:v", "libx264", "-preset", "ultrafast", src).CombinedOutput()
	if err != nil {
		t.Skipf("synthetic source generation failed: %v: %s", err, out)
	}
	return src
}

// TestDispatchInterlaceAnalysis_Success: a real analysis of a real (synthetic)
// source delivers a result while its claim is current.
func TestDispatchInterlaceAnalysis_Success(t *testing.T) {
	test.NewApp() // registers a headless current app; its driver runs DoFromGoroutine synchronously
	src := newInterlaceTestSource(t)

	s := &appState{}
	claim := s.beginConvertInterlaceAnalysis()

	delivered := make(chan struct{}, 1)
	var gotResult *interlace.DetectionResult
	var gotErr error
	dispatchInterlaceAnalysis(src, claim, func(result *interlace.DetectionResult, err error) {
		gotResult, gotErr = result, err
		delivered <- struct{}{}
	})

	select {
	case <-delivered:
	case <-time.After(3 * time.Minute):
		t.Fatal("analysis did not deliver within the timeout budget")
	}
	if gotErr != nil {
		t.Fatalf("expected a successful analysis, got: %v", gotErr)
	}
	if gotResult == nil {
		t.Fatal("expected a detection result")
	}
}

// TestDispatchInterlaceAnalysis_ErrorPropagates: an unanalysable path
// delivers the error to the caller - the caller decides what to publish.
func TestDispatchInterlaceAnalysis_ErrorPropagates(t *testing.T) {
	test.NewApp()
	requireTestFFmpeg(t)

	s := &appState{}
	claim := s.beginConvertInterlaceAnalysis()

	delivered := make(chan struct{}, 1)
	var gotErr error
	dispatchInterlaceAnalysis(filepath.Join(t.TempDir(), "definitely-missing.mkv"), claim, func(result *interlace.DetectionResult, err error) {
		if result != nil {
			t.Error("an errored analysis must not carry a result")
		}
		gotErr = err
		delivered <- struct{}{}
	})

	select {
	case <-delivered:
	case <-time.After(3 * time.Minute):
		t.Fatal("analysis did not deliver within the timeout budget")
	}
	if gotErr == nil {
		t.Fatal("expected the analysis error to propagate")
	}
}

// TestDispatchInterlaceAnalysis_StaleDiscarded: a completed analysis whose
// claim was invalidated mid-flight is discarded - deliver is never invoked.
func TestDispatchInterlaceAnalysis_StaleDiscarded(t *testing.T) {
	test.NewApp()
	src := newInterlaceTestSource(t)

	s := &appState{}
	claim := s.beginConvertInterlaceAnalysis()

	delivered := make(chan struct{}, 1)
	dispatchInterlaceAnalysis(src, claim, func(result *interlace.DetectionResult, err error) {
		delivered <- struct{}{}
	})

	// The source changes while the analysis is in flight.
	s.setConvertSource(&videoSource{Path: "other.mkv"})

	select {
	case <-delivered:
		t.Fatal("a stale analysis must be discarded, not delivered")
	case <-time.After(5 * time.Second):
		// The analysis itself completes within this budget (a fast-failing
		// source); the absence of delivery after it means the claim rejected it.
	}
	if s.interlaceAnalyzing {
		t.Fatal("the transition must have left the slot not-analyzing")
	}
	if s.interlaceResult != nil {
		t.Fatal("no stale result may be published")
	}
}

// --- Convert queue output-path allocation (#12) ---

// TestConvertOutputAllocatorMatrix covers the #12 allocation matrix: the
// deterministic -N suffix against filesystem collisions, the batch
// used-map (which catches two queued outputs that do not exist on disk
// yet), the source-avoidance invariant, and the output-directory chain.
func TestConvertOutputAllocatorMatrix(t *testing.T) {
	t.Run("no existing output", func(t *testing.T) {
		dir := t.TempDir()
		s := &appState{convert: convertConfig{OutputDir: dir}}
		src := &videoSource{Path: filepath.Join(dir, "input.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(dir, "out.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("existing output takes -2", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "out.mkv"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		s := &appState{convert: convertConfig{OutputDir: dir}}
		src := &videoSource{Path: filepath.Join(dir, "input.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(dir, "out-2.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("out and out-2 existing take -3", func(t *testing.T) {
		dir := t.TempDir()
		for _, n := range []string{"out.mkv", "out-2.mkv"} {
			if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		s := &appState{convert: convertConfig{OutputDir: dir}}
		src := &videoSource{Path: filepath.Join(dir, "input.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(dir, "out-3.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("source avoidance is its own invariant", func(t *testing.T) {
		dir := t.TempDir()
		s := &appState{convert: convertConfig{OutputDir: dir}}
		// The requested name IS the source's own name: the candidate equals
		// the source path and must be prefixed, not suffixed.
		src := &videoSource{Path: filepath.Join(dir, "in.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "in.mkv")
		if want := filepath.Join(dir, "converted-in.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("two same-named outputs in one batch - no files on disk", func(t *testing.T) {
		dir := t.TempDir()
		s := &appState{convert: convertConfig{OutputDir: dir}}
		alloc := newConvertOutputAllocator()
		srcA := &videoSource{Path: filepath.Join(dir, "a.mkv")}
		srcB := &videoSource{Path: filepath.Join(dir, "b.mkv")}
		first := s.allocateConvertOutputPath(alloc, srcA, "out.mkv")
		second := s.allocateConvertOutputPath(alloc, srcB, "out.mkv")
		if want := filepath.Join(dir, "out.mkv"); first != want {
			t.Errorf("first = %q, want %q", first, want)
		}
		if want := filepath.Join(dir, "out-2.mkv"); second != want {
			t.Errorf("second = %q, want %q - the batch used-map must catch unwritten outputs", second, want)
		}
	})

	t.Run("filesystem plus batch collision advances deterministically", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "out.mkv"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		s := &appState{convert: convertConfig{OutputDir: dir}}
		alloc := newConvertOutputAllocator()
		srcA := &videoSource{Path: filepath.Join(dir, "a.mkv")}
		srcB := &videoSource{Path: filepath.Join(dir, "b.mkv")}
		first := s.allocateConvertOutputPath(alloc, srcA, "out.mkv")
		second := s.allocateConvertOutputPath(alloc, srcB, "out.mkv")
		if want := filepath.Join(dir, "out-2.mkv"); first != want {
			t.Errorf("first = %q, want %q (filesystem collision)", first, want)
		}
		if want := filepath.Join(dir, "out-3.mkv"); second != want {
			t.Errorf("second = %q, want %q (batch collision on the unwritten -2)", second, want)
		}
	})

	t.Run("empty configured dir falls back to the source directory", func(t *testing.T) {
		srcDir := t.TempDir()
		s := &appState{}
		src := &videoSource{Path: filepath.Join(srcDir, "in.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(srcDir, "out.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("whitespace-only configured dir falls back", func(t *testing.T) {
		srcDir := t.TempDir()
		s := &appState{convert: convertConfig{OutputDir: "   "}}
		src := &videoSource{Path: filepath.Join(srcDir, "in.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(srcDir, "out.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("defaultOutputDir used when module dir is empty", func(t *testing.T) {
		srcDir := t.TempDir()
		defDir := t.TempDir()
		s := &appState{defaultOutputDir: defDir}
		src := &videoSource{Path: filepath.Join(srcDir, "in.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(defDir, "out.mkv"); got != want {
			t.Errorf("got %q, want %q (the WithOutputs default chain)", got, want)
		}
	})

	t.Run("module dir wins over defaultOutputDir", func(t *testing.T) {
		srcDir := t.TempDir()
		defDir := t.TempDir()
		outDir := t.TempDir()
		s := &appState{convert: convertConfig{OutputDir: outDir}, defaultOutputDir: defDir}
		src := &videoSource{Path: filepath.Join(srcDir, "in.mkv")}
		got := s.allocateConvertOutputPath(newConvertOutputAllocator(), src, "out.mkv")
		if want := filepath.Join(outDir, "out.mkv"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// --- two-pass disclosure (dev83) ---------------------------------------------
//
// Two-pass encoding was never implemented. convertConfig.TwoPass was written by
// the UI and persisted to config and to every queued job map, but no
// encoder-argument builder ever read it and no ffmpeg "-pass"/"-passlogfile"
// argument was ever emitted anywhere in the repo. The control therefore did
// nothing, while its note claimed two-pass was merely "ignored in CRF mode" and
// the VBR hint claimed VBR "uses 2-pass encoding". The control is now disabled
// and the note discloses the truth, so these tests pin that disclosure.

// TestTwoPassDisclosedAsUnimplemented pins the user-visible contract in every
// locale: the two-pass note must state outright that the feature is not
// implemented, and no locale may hint that a two-pass pipeline is used.
func TestTwoPassDisclosedAsUnimplemented(t *testing.T) {
	cases := []struct {
		name   string
		lang   string
		script i18n.ScriptVariant
		want   string // must appear in the two-pass note
		banned string // the old false claim; must not survive
	}{
		{"en-CA", "en-CA", i18n.ScriptDefault, "not implemented", "ignored in CRF mode"},
		{"fr-CA", "fr-CA", i18n.ScriptDefault, "pas implémenté", "ignoré en mode CRF"},
		{"iu syllabics", "iu", i18n.ScriptSyllabics, "not implemented", "ignored in CRF mode"},
		{"iu Latin", "iu", i18n.ScriptLatin, "not implemented", "ignored in CRF mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i18n.SetLanguageWithScript(tc.lang, tc.script)
			s := i18n.T()

			note := s.ConvertTwoPassNote
			if strings.TrimSpace(note) == "" {
				t.Fatal("ConvertTwoPassNote is empty; the disabled control would show no explanation")
			}
			if !strings.Contains(strings.ToLower(note), strings.ToLower(tc.want)) {
				t.Errorf("ConvertTwoPassNote = %q, want it to disclose %q", note, tc.want)
			}
			if strings.Contains(strings.ToLower(note), strings.ToLower(tc.banned)) {
				t.Errorf("ConvertTwoPassNote = %q still carries the false claim %q", note, tc.banned)
			}

			// The VBR hint must not claim a two-pass encode. Locales that do not
			// define the key fall back to en-CA, so this holds everywhere.
			hint := strings.ToLower(s.ConvertBitrateModeHintVBR)
			for _, banned := range []string{"2-pass", "two-pass", "two pass"} {
				if strings.Contains(hint, banned) {
					t.Errorf("ConvertBitrateModeHintVBR = %q claims %q; VBR is single-pass", s.ConvertBitrateModeHintVBR, banned)
				}
			}
		})
	}
	i18n.SetLanguage("en-CA") // don't leak locale state into other tests
}

// TestNoTwoPassFfmpegArgs guards the reason the control is disabled: the app
// runs one encode pass. If a "-pass"/"-passlogfile" argument ever appears, the
// encoder path grew real two-pass support and the UI disclosure (plus this
// slice's premise) has to be revisited rather than left stale.
func TestNoTwoPassFfmpegArgs(t *testing.T) {
	skipDir := func(name string) bool {
		return name == "_fyne" || name == "vendor" || name == "testdata" ||
			strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
	}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // production encoder args never live in a test file
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, banned := range []string{`"-pass`, `"-passlogfile`} {
			if idx := strings.Index(string(b), banned); idx >= 0 {
				t.Errorf("%s emits ffmpeg %s (offset %d): two-pass is not implemented and the UI says so",
					path, banned, idx)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}
}

// TestNoDeadVLCBackendControl guards the dev84 libVLC cut.
//
// Settings shipped a "Use libVLC backend" checkbox that persisted to config and
// called into media.SetUseVLC, but the only reader of that flag lived in
// engine_factory_vlc.go behind a `vlc` build tag that no workflow has ever
// built. In every shipped binary the checkbox therefore changed nothing: the
// same user-facing lie as the Convert two-pass toggle (#23), one layer down.
//
// This asserts no user-facing string may advertise a playback backend that does
// not exist. If a real libVLC backend ever lands, it must come with a CI job
// that builds its build tag, and this test should be revisited at that point
// rather than quietly deleted.
//
// Every locale/script variant the app can actually display is enumerated: the
// registry in internal/i18n is unexported, so a new locale added there must be
// added here too or it goes unchecked.
func TestNoDeadVLCBackendControl(t *testing.T) {
	variants := []struct {
		code   string
		script i18n.ScriptVariant
	}{
		{"en-CA", i18n.ScriptDefault},
		{"fr-CA", i18n.ScriptDefault},
		{"iu", i18n.ScriptSyllabics},
		{"iu", i18n.ScriptLatin},
	}
	for _, v := range variants {
		i18n.SetLanguageWithScript(v.code, v.script)
		label := v.code
		if v.script != i18n.ScriptDefault {
			label += "/" + string(v.script)
		}
		val := reflect.ValueOf(i18n.T())
		for i := 0; i < val.NumField(); i++ {
			s, ok := val.Field(i).Interface().(string)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(s), "libvlc") {
				t.Errorf("locale %s: %s still advertises the removed libVLC backend: %q", label, val.Type().Field(i).Name, s)
			}
		}
	}
	i18n.SetLanguage("en-CA") // don't leak locale state into other tests
}

// stripGoComments returns the source with all comments removed, so a guard can
// assert on code without matching its own explanatory prose. A naive
// line-based scan would be wrong in both directions: it cannot tell a comment
// mentioning an identifier from the identifier itself, and it can be defeated by
// a trailing comment on a real line.
func stripGoComments(src string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, parser.ParseComments)
	if err != nil {
		return src // unparseable: fall back to raw text, guards still fire
	}
	type span struct{ start, end int }
	var spans []span
	for _, cg := range f.Comments {
		spans = append(spans, span{
			start: fset.Position(cg.Pos()).Offset,
			end:   fset.Position(cg.End()).Offset,
		})
	}
	var b strings.Builder
	prev := 0
	for _, s := range spans {
		if s.start > prev {
			b.WriteString(src[prev:s.start])
		}
		prev = s.end
	}
	if prev < len(src) {
		b.WriteString(src[prev:])
	}
	return b.String()
}

// TestConvertHasNoCachedPlaybackState guards Convert audit #20 / #11 finding
// F-4: the Convert pane used to write `state.playerPaused = true` in its
// constructor and branch its transport UI on that cached flag.
//
// The defect was not that the flag was wrong in isolation — it was that the flag
// was the authority. Because buildVideoPaneNative is re-run on every source
// switch, Reset, and panel toggle, each rebuild re-asserted a pause the engine
// never entered, and any transition driven from elsewhere (end-of-stream, frame
// stepping, the overlay pill, a peer player) left the icon stale.
//
// Convert must derive display state from ui.InlineVideoPlayer instead, which is
// what OnPlayStateChange exists for.
func TestConvertHasNoCachedPlaybackState(t *testing.T) {
	b, err := os.ReadFile("convert_player_native.go")
	if err != nil {
		t.Fatalf("reading convert_player_native.go: %v", err)
	}
	src := string(b)

	if strings.Contains(src, "playerPaused") {
		// Field reads are still a problem, but the fatal form is the write.
		t.Error("convert_player_native.go still references playerPaused; the Convert pane " +
			"must derive playback state from ui.InlineVideoPlayer (audit #20 / #11 F-4), " +
			"not cache its own flag")
	}

	// The pane must subscribe to the authority, otherwise a fresh pane has no
	// way to learn that state changed after it was built.
	for _, want := range []string{"player.OnPlayStateChange(", "player.IsPlaying()"} {
		if !strings.Contains(src, want) {
			t.Errorf("convert_player_native.go does not contain %q; the transport button cannot "+
				"follow the engine without it", want)
		}
	}
}

// TestPlaybackStatePublishersAreWired guards the other half of #20: an
// InlineVideoPlayer that publishes nothing is the same defect as the cached
// flag, only harder to spot — the getters would look authoritative while never
// updating.
//
// The invariant is deliberately structural rather than behavioural. Every
// mutation of the authoritative fields must have a matching notification, so the
// test walks the mutators and asserts the callback is invoked. If a future
// contributor adds a mutator without publishing, this fails.
func TestPlaybackStatePublishersAreWired(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "ui", "inline_player.go"))
	if err != nil {
		t.Fatalf("reading internal/ui/inline_player.go: %v", err)
	}
	src := string(b)

	// Mutator → the notification it must reach. Presence of the field write is
	// not enough; the callback has to actually be fired on that path.
	required := map[string][]string{
		"play state":     {"v.notifyPlayState()"},
		"speed":          {"fireOnMain(func() { cb(speed) })"},
		"audio track":    {"fireOnMain(func() { cb(idx) })"},
		"subtitle track": {"fireOnMain(func() { cb(-1) })"},
		"reset on load":  {"fireOnMain(func() { speedCB(1.0) })"},
		"reset on close": {"fireOnMain(func() { subCB(-1) })"},
		"end of stream":  {"v.notifyPlayState()"},
	}
	for name, needles := range required {
		found := false
		for _, n := range needles {
			if strings.Contains(src, n) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("internal/ui/inline_player.go does not publish the %s notification "+
				"(expected one of: %v); the authority would silently go stale", name, needles)
		}
	}

	// The nil-app guard is load-bearing for tests and early boot: without it a
	// headless construction panics inside the driver call.
	if !strings.Contains(src, "app == nil") {
		t.Error("fireOnMain lost its nil-app guard; it panics when no Fyne app is running")
	}

	// notifyPlayState must not be able to resurrect a stale value. A
	// `defer publishPlayState(true)` in Play() races playbackLoop's EOF path:
	// EOF sets playing=false and publishes, then the defer forces true again,
	// pinning the button on "playing" over a finished video.
	if !strings.Contains(src, "func (v *InlineVideoPlayer) notifyPlayState()") {
		t.Error("notifyPlayState is missing; play-state publication must compare against " +
			"notifiedPlaying so a late notification cannot overwrite a newer state")
	}
	if !strings.Contains(src, "v.notifiedPlaying == playing") {
		t.Error("notifyPlayState no longer guards on notifiedPlaying; it will re-fire on " +
			"every call and can overwrite a state that has since changed")
	}
	// Checked against code only: the function's own doc comment names
	// publishPlayState while explaining why it must not be used that way.
	code := stripGoComments(src)
	if strings.Contains(code, "publishPlayState") {
		t.Error("a deferred or assigning publishPlayState reintroduces the Play()/EOF race " +
			"it was replaced to fix; use defer v.notifyPlayState() instead")
	}
	// The positive half matters as much as the negative: Play() must actually
	// defer the notifier. Checking only that publishPlayState is absent would pass
	// a build where the defer was simply deleted and the transition never
	// published at all — the same stale UI as the cached flag, silently.
	if !strings.Contains(code, "defer v.notifyPlayState()") {
		t.Error("Play() no longer defers notifyPlayState(); starting playback would leave " +
			"subscribers unaware the transport changed state")
	}
}

// TestNoVLCBackendCode pins the other half of the cut: no production Go file
// may reintroduce libVLC engine plumbing. docs/VLC_PLAYER.md is retained as the
// design document for a future attempt, but the implementation is gone and must
// not creep back in unbuilt and unverified.
func TestNoVLCBackendCode(t *testing.T) {
	skipDir := func(name string) bool {
		return name == "_fyne" || name == "vendor" || name == "testdata" || name == "docs" ||
			strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
	}
	banned := []string{"NewVLCEngine", "libvlc_media_player", "vlc_glue.h", "vlcFrameCtx"}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(b)
		for _, sym := range banned {
			if strings.Contains(src, sym) {
				t.Errorf("%s reintroduces libVLC plumbing (%q); the backend was cut in dev84 as unbuildable", path, sym)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}
}
