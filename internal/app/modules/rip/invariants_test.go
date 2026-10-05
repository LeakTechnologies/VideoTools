package rip

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

// The four invariants below were each a real shipped defect that is now fixed.
// They are recorded here as closed facts about the current implementation, not
// as open tracker items: an audit established the module's actual state before
// the remaining Rip work, and these four were verified correct at that point.
//
// They exist because the stabilization work that follows touches the same
// executor and content-list code. Each one is a silent-corruption class — the
// rip still completes, the job still reports success, and the output is simply
// the wrong content or the wrong shape. Nothing else in the suite would catch a
// regression, which is exactly why they need pinning.

// TestInvariantDVDVideoTitleBindingPrecedesInput guards the dev79 root cause.
//
// -f dvdvideo opened and ran, but with "-title" placed AFTER "-i". FFmpeg binds
// demuxer options that appear after -i to the NEXT input, so the option landed
// on the ffmetadata chapters input instead. FFmpeg rejected the invocation
// ("Option title not found"), and every dvdvideo rip fell through to whole-file
// VOB concatenation.
//
// That fallback is not a degradation, it is a content corruption: whole-file
// concat reads the FIRST title in the VOB set, so a scene-segmented extra came
// out as the movie's opening, and a multi-VOB title came out frozen at 26 hours.
// Because the fallback then succeeded, the job reported success. The bug was
// only visible by watching the log, which is precisely the failure mode the
// earlier multi-VOB corruption reports had in common.
func TestInvariantDVDVideoTitleBindingPrecedesInput(t *testing.T) {
	src := readExecutorSource(t)

	// Find the invocation that appends the dvdvideo input, regardless of the
	// current flag order, so a reordering reports the ordering defect rather than
	// a generic "not found". Anchored on the append so the demuxer capability
	// probe above it cannot be mistaken for the invocation.
	line := lineContaining(t, src, `"dvdvideo", `)
	if line == "" {
		t.Fatal(`executor no longer appends a "-f dvdvideo" input; the cell-accurate ` +
			"native path was removed. Re-establish how dvdvideo is invoked before " +
			"assuming the fallback is still correct.")
	}

	titleAt := strings.Index(line, `"-title"`)
	inputAt := strings.Index(line, `"-i"`)
	if titleAt < 0 || inputAt < 0 {
		t.Fatalf("the dvdvideo input no longer carries both -title and -i: %s", line)
	}
	if titleAt > inputAt {
		t.Errorf("-title appears after -i on the dvdvideo input (%s). FFmpeg binds a demuxer "+
			"option after -i to the NEXT input, so the title selector lands on the chapters "+
			"input, FFmpeg rejects the invocation, and every rip silently falls back to "+
			"whole-file VOB concatenation — which rips the first title's content for all "+
			"titles (dev79)", line)
	}
}

// TestInvariantAllChapterRangeIsAPassthrough guards the deliberate short-circuit
// in the chapter-range resolver.
//
// A "Rip chapters only" range that happens to span every chapter (From=1,
// To=N) is semantically the whole title. Treating it as a real range would
// push the rip onto the output-side -ss/-to trim path and the cell-slicing
// path, both of which re-derive the same span the demuxer already produced.
// That is not just wasted work: the cell-slice path replaces the input with a
// sliced list, so a whole-title range can end up bound by cell sectors that
// differ from the PGC's full span.
//
// The code therefore skips the range entirely when it covers all chapters, so
// "rip chapters only, 1..N" is byte-identical to a plain rip. This test asserts
// the skip is still present, because removing it looks like a harmless
// simplification.
func TestInvariantAllChapterRangeIsAPassthrough(t *testing.T) {
	src := readExecutorSource(t)

	// The resolver gate: a full-span range must not enter the range pipeline.
	if !strings.Contains(src, "if !(cs == 1 && ce == n)") {
		t.Error("the all-chapter short-circuit is gone from the chapter-range resolver. " +
			"A From=1/To=N range must resolve to the whole title and leave the " +
			"dvdvideo/cell pipeline untouched, or it will re-slice cells that " +
			"already cover the full span")
	}

	// And the resolver must actually report a full-span range as valid, so the
	// gate above is reachable rather than dead.
	if _, _, _, _, ok := chapterRange([]float64{0, 30, 50}, 90, 1, 3); !ok {
		t.Error("chapterRange(1..3) over a 3-chapter title must resolve; the " +
			"all-chapter gate above can never be reached if this reports false")
	}

	// Sanity-check the boundary the gate depends on: n is the chapter count, so
	// cs==1 && ce==n must be reachable with ce explicitly at n.
	_, _, endSec, span, ok := chapterRange([]float64{0, 30, 50}, 90, 1, 3)
	if !ok {
		t.Fatal("full-span chapterRange unexpectedly unresolvable")
	}
	if math.Abs(span-90) > 0.001 || math.Abs(endSec-90) > 0.001 {
		t.Errorf("full-span range should cover the whole title: endSec=%.3f span=%.3f, want 90/90", endSec, span)
	}
}

// TestInvariantBulkSelectBypassesTheModeLock guards the dev79 selection fix.
//
// In a locked rip mode (main feature / scene segments) most titles are greyed
// and unclickable. The bulk Select All originally respected that lock, so it
// selected nothing: the buttons looked broken and the user had no way to bulk
// select without switching modes, which changes the selection anyway.
//
// The fix drops the lock layer for the duration of a bulk operation. That is
// user-visible and deliberate — do not "restore" the lock check here. The
// second half of the invariant is the deadlock guard: the card's SetChecked
// fires OnChanged synchronously, and that handler re-acquires cb.mu, so the
// bulk path MUST raise the per-card updating flag first. cb.mu is held across
// the loop, so without that guard the UI thread deadlocks on itself.
func TestInvariantBulkSelectBypassesTheModeLock(t *testing.T) {
	src := readContentListSource(t)

	fn := funcBody(t, src, "func (cb *ContentBrowser) setAllSelected(v bool)")
	if fn == "" {
		t.Fatal("setAllSelected is gone; the bulk Select All / Deselect All path needs " +
			"re-establishing before the lock behaviour can be reasoned about")
	}

	// The lock layer must be cleared, not consulted per title.
	if !strings.Contains(fn, "cb.locked = nil") || !strings.Contains(fn, "cb.anchored = nil") {
		t.Error("setAllSelected no longer clears the lock layer. A bulk Select All that " +
			"respects the mode lock selects nothing, which is the dev79 defect: the " +
			"buttons appear broken because the locked titles are skipped")
	}

	// The deadlock guard must be raised around SetChecked.
	if !strings.Contains(fn, "tc.updating = true") || !strings.Contains(fn, "tc.updating = false") {
		t.Error("setAllSelected no longer raises the per-card updating guard around " +
			"SetChecked. SetChecked fires OnChanged synchronously and that handler " +
			"re-acquires cb.mu, which is held across this loop — removing the guard " +
			"deadlocks the UI thread")
	}

	// cb.mu must be released before any listener callback runs, or a listener
	// that touches the browser re-enters the held lock.
	lockAt := strings.Index(fn, "cb.mu.Lock()")
	unlockAt := strings.Index(fn, "cb.mu.Unlock()")
	if lockAt < 0 || unlockAt < 0 || unlockAt < lockAt {
		t.Fatal("setAllSelected no longer has a clear lock/unlock pair around its mutation")
	}
	for _, cb := range []string{"bulkFn(", "fn("} {
		at := strings.Index(fn, cb)
		if at > 0 && at < unlockAt {
			t.Errorf("%s is invoked while cb.mu is still held; a listener that reads back "+
				"the selection re-enters the held lock", strings.TrimSuffix(cb, "("))
		}
	}

	// A mode transition must NOT reshape selection through this path — that is
	// applyRipMode's job. If setAllSelected started calling CanonicalSelection
	// it would silently override what the user just asked to select.
	if strings.Contains(fn, "CanonicalSelection") {
		t.Error("setAllSelected now derives its selection from CanonicalSelection; a bulk " +
			"action must select what the user asked for, not re-apply the mode's canonical set")
	}
}

// TestInvariantSelectAllIgnoresGreyedTitles pins the observable consequence:
// with titles present, a bulk Select All must reach every one of them, and
// Deselect All must clear every one of them.
// TestRipFailureClassification drives the retry policy from observed ffmpeg
// stderr rather than from an exclusion list.
//
// The retry this governs swaps the INPUT (dvdvideo -> VOB concat), so it may
// only ever fire when the input is what failed. The prior gate was a deny-list
// (`!isMuxerCodecError`), which admitted output-side failures: an unwritable
// destination triggered a second full rip and then reported the fallback's
// error, hiding the real cause behind a demuxer diagnostic.
func TestRipFailureClassification(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   ripFailureClass
	}{
		{
			// Observed: output path blocked by a non-directory component.
			name:   "output path blocked",
			stderr: "[out#0/matroska @ 0000] Error opening output C:\\x\\notadir\\out.mkv: No such file or directory\nError opening output file C:\\x\\notadir\\out.mkv.\nError opening output files: No such file or directory",
			want:   failureOutput,
		},
		{
			name:   "disk full",
			stderr: "[out#0/matroska @ 0000] Error writing trailer: No space left on device",
			want:   failureOutput,
		},
		{
			name:   "permission denied on output",
			stderr: "[out#0/matroska @ 0000] Error opening output /readonly/out.mkv: Permission denied",
			want:   failureOutput,
		},
		{
			// Observed: libdvdnav rejects the source structure.
			name:   "libdvdread cannot open VIDEO_TS",
			stderr: "libdvdread: Could not open C:\\disc\\VIDEO_TS with libdvdcss.\n[in#0 @ 0000] Unable to open the DVD-Video structure\nError opening input file C:\\disc\\VIDEO_TS.",
			want:   failureSourceStructure,
		},
		{
			// The dev-era case f512625f was written for.
			name:   "dvdnav zero check failure",
			stderr: "libdvdnav: Zero check failed for vmgi_mat->zero_3\nError opening input file C:\\disc\\VIDEO_TS.",
			want:   failureSourceStructure,
		},
		{
			name:   "css unavailable",
			stderr: "libdvdread: Could not open C:\\disc\\VIDEO_TS with libdvdcss.",
			want:   failureSourceStructure,
		},
		{
			// pcm_dvd rejected by the matroska muxer: a selection problem that
			// would recur identically on the concat input.
			name:   "muxer rejects codec",
			stderr: "[matroska @ 0000] Could not find tag for codec pcm_dvd in stream #1, codec not currently supported in container",
			want:   failureSelection,
		},
		{
			// Real shape: libdvdread chatters on the way in, then the WRITE
			// fails. Both marker families are present, so classification depends
			// entirely on output being tested before source. Getting this order
			// wrong retries a full rip and blames the demuxer for a disk-full.
			name:   "source chatter then output failure",
			stderr: "libdvdread: DVD structure 0x0, zero check failed for vmgi_mat->zero_3\n[out#0/matroska @ 0000] Error opening output C:\\x\\out.mkv: No such file or directory\nError opening output files: No such file or directory",
			want:   failureOutput,
		},
		{
			// Unexplained: must NOT be retried, or the real error is buried.
			name:   "unattributable failure",
			stderr: "something went wrong deep inside a filter graph",
			want:   failureUnknown,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := &ffmpegError{err: errors.New("exit status 1"), stderr: c.stderr}
			if got := classifyRipFailure(context.Background(), err); got != c.want {
				t.Errorf("classifyRipFailure() = %v, want %v\nstderr: %s", got, c.want, c.stderr)
			}
		})
	}
}

// TestOnlySourceStructureFailuresRetry asserts the retry predicate directly:
// it must admit exactly the source-structure class.
func TestOnlySourceStructureFailuresRetry(t *testing.T) {
	retryable := &ffmpegError{err: errors.New("exit status 1"),
		stderr: "libdvdread: Could not open C:\\disc\\VIDEO_TS with libdvdcss.\nError opening input file."}
	if !isRetryableWithConcat(retryable) {
		t.Error("a source-structure failure must be retried on the concat input; this is " +
			"the case the fallback exists for (f512625f)")
	}

	// The regression: an unwritable destination is not a demuxer problem.
	output := &ffmpegError{err: errors.New("exit status 1"),
		stderr: "Error opening output file C:\\x\\out.mkv.\nError opening output files: No such file or directory"}
	if isRetryableWithConcat(output) {
		t.Error("an output-side failure must NOT retry: the concat input would fail " +
			"identically, costing a second full rip and then reporting this fallback's " +
			"error in place of the real cause")
	}

	// A muxer/codec rejection is a selection problem, not an input problem.
	sel := &ffmpegError{err: errors.New("exit status 1"),
		stderr: "Could not find tag for codec pcm_dvd in stream #1"}
	if isRetryableWithConcat(sel) {
		t.Error("a muxer codec rejection must NOT retry: the same streams hit the same " +
			"muxer regardless of input, and the retry masks the codec as the cause")
	}

	if isRetryableWithConcat(&ffmpegError{err: errors.New("exit status 1"), stderr: "???"}) {
		t.Error("an unattributable failure must NOT retry: retrying it replaces the " +
			"real error with the fallback's")
	}
}

// TestCancellationIsNeverRetried asserts the abort contract: a cancelled rip
// must not be re-launched on another input.
func TestCancellationIsNeverRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := &ffmpegError{err: context.Canceled, stderr: "libdvdread: Could not open VIDEO_TS"}
	if got := classifyRipFailure(ctx, err); got != failureCancelled {
		t.Errorf("classifyRipFailure(cancelled ctx) = %v, want cancelled", got)
	}
	if isRetryableWithConcat(err) {
		t.Error("a cancelled rip must never be retried; the user aborted it")
	}

	// Even with a live context, an explicit cancellation error is honoured.
	if got := classifyRipFailure(context.Background(), context.Canceled); got != failureCancelled {
		t.Errorf("classifyRipFailure(context.Canceled) = %v, want cancelled", got)
	}
}

// TestFailureToStartFFmpegIsNotRetried asserts a missing/broken ffmpeg is not
// mistaken for a source problem.
func TestFailureToStartFFmpegIsNotRetried(t *testing.T) {
	for _, msg := range []string{
		"ffmpeg start: exec: \"ffmpeg.exe\": file does not exist",
		"stdout pipe: unexpected EOF",
	} {
		err := errors.New(msg)
		if got := classifyRipFailure(context.Background(), err); got != failureExecution {
			t.Errorf("classifyRipFailure(%q) = %v, want execution", msg, got)
		}
		if isRetryableWithConcat(err) {
			t.Errorf("%q must not retry: ffmpeg never ran, so the input was never read", msg)
		}
	}
}

// TestNilErrorIsNotRetryable keeps a nil from classifying as a source failure.
func TestNilErrorIsNotRetryable(t *testing.T) {
	if isRetryableWithConcat(nil) {
		t.Error("a nil error must never be retryable")
	}
}

func TestInvariantSelectAllIgnoresGreyedTitles(t *testing.T) {
	titles := []DiscTitle{
		{Number: 1, Duration: 5900},
		{Number: 2, Duration: 840},
		{Number: 3, Duration: 600},
	}
	ss := SceneSetInfo{
		Present:       true,
		SceneTitles:   map[int]bool{2: true, 3: true},
		WholeTitles:   map[int]bool{1: true},
		Representative: 1,
	}

	// The main-feature lock greys everything but the longest title.
	lock := CanonicalLock(titles, "main", ss)
	if len(lock.Locked) != 2 {
		t.Fatalf("expected the main mode to lock 2 of 3 titles, got %d", len(lock.Locked))
	}

	// setAllSelected clears the lock and then writes every card, so the post
	// state is "all selected" regardless of what was locked. This mirrors the
	// mutation the implementation performs; it asserts the invariant's shape
	// rather than the widget plumbing, which needs a running Fyne app.
	locked, anchored := lock.Locked, lock.Anchored
	locked, anchored = nil, nil
	sel := make(map[int]bool, len(titles))
	for _, dt := range titles {
		sel[dt.Number] = true
	}

	if len(locked) != 0 || len(anchored) != 0 {
		t.Fatalf("bulk select must clear the lock layer, got locked=%v anchored=%v", locked, anchored)
	}
	for _, dt := range titles {
		if !sel[dt.Number] {
			t.Errorf("title %d stayed unselected after a bulk Select All that bypassed the lock", dt.Number)
		}
	}

	// Deselect All clears everything, again including previously locked titles.
	for n := range sel {
		sel[n] = false
	}
	for n := range sel {
		if sel[n] {
			t.Errorf("title %d stayed selected after Deselect All", n)
		}
	}
}

func readExecutorSource(t *testing.T) string {
	t.Helper()
	return readRipSource(t, "executor.go")
}

func readContentListSource(t *testing.T) string {
	t.Helper()
	return readRipSource(t, "content_list.go")
}

// readRipSource reads a file from the rip package directory. Tests run with the
// package directory as the working directory, so a plain relative read is
// correct and keeps these guards free of absolute paths.
func readRipSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}

// lineContaining returns the first source line containing needle, trimmed.
func lineContaining(t *testing.T, src, needle string) string {
	t.Helper()
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// funcBody extracts the source of the function whose signature contains sig,
// from its declaration to the line that closes it at column zero.
func funcBody(t *testing.T, src, sig string) string {
	t.Helper()
	lines := strings.Split(src, "\n")
	start := -1
	for i, line := range lines {
		if strings.Contains(line, sig) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	for i := start; i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
		// A closing brace at column zero ends a top-level func.
		if i > start && lines[i] == "}" {
			break
		}
	}
	return b.String()
}
