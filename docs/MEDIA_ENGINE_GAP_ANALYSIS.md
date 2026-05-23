# Media Engine Gap Analysis

Comprehensive catalog of missing features, dead code, and gaps vs. a production video player.
Every item has a file:line reference, severity, and estimated effort.

---

## Phase 0 — Critical Stability (broken/missing paths that cause hangs or unrecoverable errors)

| # | Gap | Location | Problem | Fix |
|---|-----|----------|---------|-----|
| P0-1 | **DegradeToSoftware is dead code** — `ShouldDegrade()`, `RecordHWFailure()`, `ResetHWFailureCount()` also unreachable | `errors.go:57,91,105,111` | HW decode failure sets `videoDecodeDead=true` — permanent kill switch with no SW fallback. `hwFailCount` never incremented. `hwDegraded` never set. Every frame retries HW → fails → skip. | Wire into GrabFrame/videoDecodeLoop: on N HW failures, call `go DegradeToSoftware()`. Replace `videoDecodeDead` kill with graceful degrade. |
| P0-2 | **No last-good-frame on decode error** — when `videoDecodeDead` fires, `NextFrame` hangs forever | `playback.go:585-588`, `playback.go:662` | After SEH in `videoDecodeLoop`, remaining `frameQueue` frames are drained, then `NextFrame` blocks at `df = <-e.frameQueue` forever — no EOF sentinel arrives because `decodeEOFSent` was never set. | Save last successful frame to a fallback `atomic.Pointer`. On error, set EOF sentinel into frameQueue so NextFrame exits. |
| P0-3 | **Backward frame stepping broken** — `Step()` rejects negative `frames` | `playback.go:335-338` | `if frames <= 0 { return error }` — all `StepFrame(-1)` callers silently fail. No frame backward mechanism exists. | Add true backward step: seek back 2 keyframes, decode forward to previous frame. |
| P0-4 | **No error history** — `lastError` is a single slot written only in dead code | `errors.go:17-48`, `engine.go:205` | `GetLastError()` / `ClearError()` never called. Errors that occur are logged but not exposed to the UI or diagnostics. | Replace `*PlaybackError` with ring buffer. Wire `SetError` into all error paths. Expose via `InlineVideoPlayer.GetErrorHistory()`. |
| P0-5 | **No fallback between `Open()` and `OpenDVD()`** — if `Open()` fails on a DVD structure, no retry | `engine.go:843`, `inline_player.go:157-165` | `Load()` calls `eng.Open()` only. `LoadDVD()` calls `eng.OpenDVD()` only. No auto-detection of disc structures. | Add `Engine.OpenAuto(path)` that tries `Open()`, then `OpenDVD(path, 0)` on failure. Wire into `Load()`. |

---

## Phase 1 — Player Completeness (missing basic player features)

| # | Gap | Location | Problem | Fix |
|---|-----|----------|---------|-----|
| P1-1 | **No network/URL streaming** — `avformat_open_input` gets no AVDictionary options | `engine.go:851` | No `timeout`, `reconnect_streamed`, `reconnect_on_network_error`, `reconnect_delay_max`, `protocol_whitelist`, `tls_verify`. Network URLs block indefinitely on failure. | Add `Engine.OpenURL(url string, opts map[string]string)` that builds an AVDictionary with sensible defaults (60s timeout, reconnect for streamed, etc.). Add `InlineVideoPlayer.LoadURL()`. |
| P1-2 | **No resume/watch-later** — only Trim module has it | `state/resume.go`, `trim/view.go:584` | No other module saves/restores playback position. Main player has no resume capability. | Integrate `ResumeState` into `InlineVideoPlayer`. Auto-save position on pause/seek/close. Restore on load. Persist audio track, subtitle track, volume. |
| P1-3 | **No audio delay adjustment** — no lip-sync correction | engine/playback/inline_player — zero references | All A/V sync relies on `WaitForPTS` + clock ratchet. No per-user or per-file offset. | Add `AudioDelay` field to Engine + InlineVideoPlayer. Offset clock target in Seek and WaitForPTS. Persist in PrefsConfig. UI slider in Settings → Player. |
| P1-4 | **No speed + pitch correction** — speed change shifts pitch | `playback.go:172-174` (speed field) | `MasterClock.SetSpeed()` changes playback rate. Audio is resampled linearly — chipmunk/baritone effect at non-1.0x speed. | Add scaletempo/rubberband via libavfilter (atempo filter + rubberband). Or integrate a pitch-preserving resampler in the audio pipeline. |
| P1-5 | **No A-B loop** — no repeat-section support | not implemented anywhere | Missing basic player feature for review/editing workflows. | Add `SetLoopPoints(a, b float64)`, `SetLoopEnabled(bool)`. Wire into NextFrame: after B, seek back to A. Expose via InlineVideoPlayer. |
| P1-6 | **SeekAccuracy locked to Keyframe** — Frame and Accurate modes unreachable | `engine.go:355`, `inline_player.go:230` | All callers hardcode `SeekAccuracyKeyframe`. No Settings toggle. No persist. | Add seek accuracy dropdown to Settings → Player. Persist in PrefsConfig. Apply in loadViaOpen. |
| P1-7 | **No bilinear/mitchell scaling** — nearest-neighbour only, aliasing on downscale | `view.go` `scaleNearest` | Fine horizontal lines/text produce aliasing when source is larger than widget. | Add `scaleBilinear` path with integer arithmetic inner loop. Auto-select when scale < 1.0. Expose as InlineVideoPlayer.SetScaleMode(). |
| P1-8 | **No frame timing diagnostics overlay** — no `Ctrl+J` stats | not implemented | No way to see A/V sync drift, drop rate, frame rate, decode time from the UI. MEDIA_ENGINE_ARCHITECTURE.md plans it but no code. | Ring buffer of per-frame PTS/clock/drop/display-time. Hotkey to toggle overlay on VideoPlayer. |
| P1-9 | **No Settings UI for player tuning** — only deinterlace + aspect ratio | `settings/tabs.go` | No controls for HW decode toggle, seek accuracy, buffer size, thread count, scale mode, vsync mode, audio buffer latency, max drift threshold. | Build Player tab in Settings with all tunables. Restart engine on changes that require it. |
| P1-10 | **No growing/in-progress file support** — reads file once at `Open()` | `engine.go:868-874` | No re-probe, no size-change detection, no re-read of stream info. Partially-written files produce bad results. | Watch file size via goroutine. When size grows, re-probe format context. Or add `Reopen()` that re-runs `openFinalize()`. |
| P1-11 | **No clock drift correction goroutine** — audio underruns let clock drift forever | `clock.go` | No background comparison of clock time vs wall-time reference. Long-running playback with intermittent audio errors accumulates uncorrected drift. | Background goroutine waking every 250ms. If no audio advance within window, snap clock to wall-time anchor. Track `lastAudioAdvanceAt`. |

---

## Phase 2 — ISO Engine Integration

These belong to the ISO Engine project, not the Media Engine. The Media Engine opens them as files.

| # | Gap | Location | Problem | Fix |
|---|-----|----------|---------|-----|
| P2-1 | **ISO-as-file playback** — can't play a .ISO directly | `engine.go:843-851` | `avformat_open_input` treats a .ISO as raw MPEG-PS or fails. FFmpeg supports `dvdvideo` demuxer (via libdvdnav) for ISO files, but only when `OpenDVD()` is called, and `Load()` never calls `OpenDVD()`. | Wire `OpenAuto()`: try `Open()`, detect DVD structure, retry with `OpenDVD()`. Expose as `LoadAuto(path)`. |
| P2-2 | **UDF reader robustness** — fallback AVDP scan, format validation, multi-extent | `internal/dvd/udf/` | Planned but not done. The UDF reader is needed for IFO parsing and disc inspection, not for playback. | See TODO.md. Already scoped. |
| P2-3 | **No Blu-ray / BDMV support** | not implemented | No BDMV directory parsing, no Blu-ray menu support, no AACS/BD+ handling. | Future scope after DVD playback is solid. |

---

## Phase 3 — Polish & Diagnostics

| # | Gap | Location | Problem | Fix |
|---|-----|----------|---------|-----|
| P3-1 | **HW decode disabled by default** — no user knows they can enable it | `hwdecode.go:132` | `hwDecodeEnabled = false`. Settings UI doesn't expose it. With VEH/SEH bridge catching AV+stack overflow, safe to enable for known-good codecs. | Add HW toggle to Settings Player tab. Default to `true` with per-codec blacklist. Wire engine restart on toggle. |
| P3-2 | **No HDR tone-mapping** — no HDR10/HLG/Dolby Vision support | not implemented | FFmpeg detects HDR metadata but VT ignores it. Playback on SDR displays shows washed-out colours. | Detect `AV_FRAME_DATA_MASTERING_DISPLAY_METADATA` + `AV_FRAME_DATA_CONTENT_LIGHT_LEVEL`. Apply BT.2390 tone-mapping via libavfilter or custom shader. |
| P3-3 | **No playlist/queue** — sequential multi-file play | not implemented | No way to queue files for sequential playback. | `InlineVideoPlayer.Enqueue(path)`, `SetOnEnd` auto-advances. Playlist widget in module. |
| P3-4 | **Per-codec HW blacklist UI** — users can't disable HW for specific codecs | `hwdecode.go:241-269` | `codecCanUseHWDevice()` is hardcoded to h264/hevc/vp9/av1/vp8. No user override. | Add codec deny-list to PrefsConfig. Add Codec HW toggle table in Settings. |

---

## How VLC and other players handle these

### VLC

- **Decoder fallback** (established since 2016): `module_need_next` / `vlc_module_load_next` — when a decoder module's `pf_decode_*` fails, VLC saves the first 3 MB of input blocks and replays them on the next decoder module. This handles both HW→SW and S/PDIF→PCM fallback seamlessly.
- **Error resilience**: `avcodec-error-resilience` level (0-4), passed to libavcodec. Level 1 is default.
- **Media library**: Central database of play history, bookmarks, metadata.
- **Stream reconnection**: Configurable via `--http-reconnect`, `--rtsp-tcp`, etc.

### Other players

- **Error concealment**: Repeating the last good frame on decode failure is the minimum bar. Most players also attempt error concealment via libavcodec's built-in mechanisms (error resilience, skip loop filter, etc.).
- **Growing-file handling**: MPV re-reads the file every N seconds and re-probes if size changes. When a file is "apparently being appended to", it uses a 4-second timeout per read instead of immediate EOF.
- **Frame stepping (backward)**: Requires re-seeking to 2 keyframes before target, then decoding forward to the exact frame. This is O(GOP) worst-case but gives correct backward stepping.

---

## Effort Estimate Summary

| Phase | Items | Est. effort |
|-------|-------|-------------|
| **P0** | 5 critical fixes | ~3-4 days |
| **P1** | 11 completeness features | ~3-4 weeks |
| **P2** | 3 ISO engine items | Already scoped in separate project |
| **P3** | 4 polish features | ~1-2 weeks |

---

## Immediate Next Steps (what we should fix NOW)

1. Wire `DegradeToSoftware()` —  make HW→SW fallback persistent insteadof per-frame kill
2. Add error ring buffer — replace single `lastError`
3. Fix backward step — un-break `StepFrame(-1)`
4. Fix `NextFrame` hang after `videoDecodeDead` — add EOF sentinel on fatal error
5. Add `OpenAuto()` — try Open then OpenDVD
6. Add network streaming — `OpenURL()` with AVDictionary options
