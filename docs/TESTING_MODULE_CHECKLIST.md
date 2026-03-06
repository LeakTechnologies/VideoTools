# VideoTools Module Testing Checklist

Use this checklist before promoting a dev build to a public release (`v0.1.x`).
Run tests on:
- Windows 10/11 (secondary platform)
- Linux (primary platform)

For each module, test with:
- one short file (<2 min)
- one medium file (10-20 min)
- one large file (60+ min)
- at least one non-16:9 source (4:3 or ultrawide)

## Release Gate (Public Bump Readiness)

- [ ] Windows and Linux CI package builds pass on latest commit.
- [ ] No critical regressions in Convert, Queue, Subtitles, or startup stability.
- [ ] `docs/CHANGELOG.md` has a complete section for the target public version.
- [ ] Smoke tests pass for all enabled modules in this document.
- [ ] Known issues are either fixed or documented as deferred in `TODO.md`.
- [ ] Release artifacts install and launch from a clean machine profile.

## Main Menu / Navigation

- [ ] Window size remains stable while switching modules.
- [ ] Module tiles stay aligned (3 per row) at common resolutions.
- [ ] Optional module toggles immediately show/hide modules.
- [ ] Queue counter updates correctly when jobs are added/completed/failed.

## Convert Module

- [ ] Source aspect option shows detected source ratio correctly.
- [ ] Aspect conversion lands on expected dimensions (no odd sizes like 1920x1082 unless intended).
- [ ] Source aspect with auto-crop off ignores stale crop values.
- [ ] Custom aspect workflow works for ultrawide ratios.
- [ ] CBR, VBR, and CRF modes apply expected encoder parameters.
- [ ] Hardware acceleration modes (`auto`, `none`, vendor-specific) execute correctly.
- [ ] Conversion completion updates queue, UI state, and output metadata correctly.
- [ ] Failure path surfaces a clear error and keeps the app running.

## Merge Module

- [ ] Merges same-codec/same-resolution inputs successfully.
- [ ] Handles mismatched input streams with clear validation errors.
- [ ] Output duration matches concatenated source duration.

## Trim Module

- [ ] Start/end trim points are respected exactly.
- [ ] Invalid ranges are blocked with clear messages.
- [ ] Trimmed output plays correctly and is seekable.

## Filters Module

- [ ] Selected filters are applied in expected order.
- [ ] No filter causes crashes on 720p, 1080p, and 4K inputs.
- [ ] Output metadata remains valid after filter processing.

## Audio Module

- [ ] Audio extraction works for AAC and Opus targets.
- [ ] Audio-only output has correct duration/channels/sample rate.
- [ ] Invalid source/audio selections produce readable errors.

## Subtitles Module

- [ ] Embedded subtitle tracks can be listed and extracted.
- [ ] OCR path (Tesseract) produces SRT and ASS outputs.
- [ ] OCR cleanup removes duplicate consecutive cues.
- [ ] Re-embedding subtitles keeps sync versus source timeline.
- [ ] DVD/BD image-based subtitle handling works without sync drift.

## Inspect Module

- [ ] Metadata loads quickly and matches ffprobe output.
- [ ] Compare views clearly report differences for key fields (codec, resolution, fps, bitrate, AR).
- [ ] Unicode/formatting in report output is readable (no mojibake).

## Compare Module

- [ ] Input selection and compare execution works for common formats.
- [ ] Output report is complete and readable on Windows and Linux.
- [ ] Report handles missing metadata fields safely.

## Upscale Module

- [ ] FFmpeg-based upscale pipeline starts and reports progress.
- [ ] Output resolution and aspect handling remain correct.
- [ ] Module availability reflects dependency state accurately.

## Author / Rip / Blu-ray Modules (if enabled)

- [ ] Module visibility toggles in Preferences behave correctly.
- [ ] Disabled/hidden modules do not affect app startup or layout.
- [ ] Basic rip/author workflows fail gracefully when dependencies are missing.

## Player Module

- [ ] Playback starts/stops/pauses without UI freezes.
- [ ] Seeking works and playback resumes from expected position.
- [ ] Fullscreen toggle works and exits cleanly.
- [ ] End-of-stream handling resets player state correctly.

## Thumbnail Module

- [ ] Thumbnails generate at requested intervals/count.
- [ ] Output images have correct dimensions and timestamps.
- [ ] Errors for unreadable inputs are surfaced clearly.

## Settings / Preferences / Dependencies

- [ ] Preferences persist across restart (including hardware acceleration and module visibility).
- [ ] Benchmark apply updates hardware acceleration only.
- [ ] Dependency status reflects actual install state.
- [ ] Dependency install/uninstall actions work on each platform path.

## Queue / Background Jobs

- [ ] Multiple queued jobs execute in order and update status correctly.
- [ ] Cancel/stop behavior is predictable and does not deadlock UI.
- [ ] App remains open during long-running jobs.
- [ ] Recovery notice appears after forced close during active conversion.

## Packaging / Release Validation

- [ ] Windows package contains expected files only.
- [ ] Linux package artifacts are named and uploaded correctly.
- [ ] Release assets replace prior artifacts (no duplicates).
- [ ] Release note includes nightly context and current version changelog section.

## Test Run Log Template

- Build/version tested:
- Platform:
- Module:
- Input file(s):
- Settings used:
- Result (pass/fail):
- Notes:
