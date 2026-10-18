# Player Debug Status

Inserted optical DVD test (2026-10-13): disc topology read and title opened as MPEG-2 720x480 at 29.97 fps with audio. Visible player showed “playback audio/video buffers did not become ready”; cached audio/video PTS remained zero. Repeated bwdif return -11 and generic-source scrub/thumbnail open failures were recorded. These are observations, not established root causes. Close logged clean engine disposal/source retirement. User was away, so no audible or continuous visual acceptance. DVD playback remains unaccepted; investigate interlaced-frame output handling, readiness and DVD-aware auxiliary decoding next. Media identity omitted.

Development CI for implementation commit 42862da5 passed Linux and Windows: https://github.com/LeakTechnologies/VideoTools/actions/runs/38440710576. User accepted alternate-container playback with no noticed lip-sync issues. Release token is absent; tag publication is a Human Director handoff, not a published release claim.

## Dev97 integration playback evidence — 2026-10-13

Two user-supplied local containers were exercised with saved-position loading, 8 seconds playback, 2 seconds pause and 12 seconds resumed playback. The final alternate-container session progressed at 25 fps; sampled frame reads were about 40 ms. No player WARNING/ERROR records, audio underruns, encoded-audio drops, video drops or clock snaps were recorded. Engine close and source retirement completed. User said the run looked good and reported no noticed lip-sync issues, then authorized integration push. Computer-use snapshots confirmed rendered pictures and the finished paused state; screenshots cannot certify motion smoothness or audible sync. The harness intentionally retained the paused frame for review; repeated stats after pause were cached, not continued playback. Earlier-container run retained a seek fallback warning and a marked post-resume glitch without starvation. Oto can refill a paused source: paused-silence buffering is a plausible unproven mechanism to instrument next. Reader audio timestamp lead is not a DAC drift measurement. Output buffering was reduced from roughly 100 kB to 10–15 kB; whole-chunk refill may exceed the requested 9600-byte threshold. Test binaries/logs remained outside the repository; media identities are omitted. Source-owned cancellation/join and retired-callback suppression are integrated with startup/resume continuity. Saved-position previews decode to the requested timestamp, audio preroll is discarded before buffering, bounded readiness precedes output, and the per-player output buffer is reduced. Opt-in diagnostics report queue occupancy, output levels, underruns, drops and clock adjustments. Native build/vet/full tests, race tests and non-native build passed. User-witnessed alternate-container playback had no noticed lip-sync issues; runtime counters showed no underruns/drops/snaps and clean teardown. Exact seeking, speed fallback quality, device-latency measurement and encrypted-disc acceptance remain open. CI/tag publication must be confirmed separately.

## Startup/resume continuity visual check — 2026-10-13

The user observed the original candidate as responsive but jumping/jittery/distorted, with a delay from pressing Play to motion. The original log showed 503ms before Engine.Resume, which followed a WaitForFrame(500ms) before the decoder was started. Resumes discarded 50 encoded video packets and up to eight decoded frames and reset codec references.

Repair: start decoding without the pre-start wait; normal Pause/Resume retains queues, codecs and PCM. Paused demux freezes except bounded preview/frame demand. A consumed frame interrupted before presentation is parked for the next consumer. Paused audio Read returns silence without consuming PCM/leftovers. Initial GrabFrame retains its codec/packet state instead of calling ResetAfterGrab. Saved-position load uses timed GrabFrame, preserving the independently landed master correction.

Visible software-decode checks on the same neutral local MP4:
- Test 1: first presented frame 82.0ms after Play, media time 0.035; normal resume 33.5ms, media 5.973 after paused readout 5.906. Completed with 417 progress frames.
- Test 2: fresh start 52.0ms; six rapid resumes 14.0-17.8ms. Paused readout 6.974 after an approximately 6.6-second active segment; this estimate is not frame-accuracy acceptance. Closing during a new load returned cancellation (completed-before-close=false), reopening presented a frame in 38.5ms, and final state was source cleared/playing=false. Combined progress count 771 at the first completed Test 2.
- No ERROR or WARNING records in these completed checks. Media titles/full paths remain in temporary logs outside the repository.
- User reports quicker continuous playback without distortion. A small hitch was noticed near the end of Test 1's 2x segment before pause; cause remains unconfirmed. User later suggested Test 2's six deliberate motion stops/restarts were perceived as jitter. Record intentional stops separately from residual pacing issues.
- Test 1's final readout was 13.914, from a pause point 5.906 plus nominal 5 seconds at 1x and 3 seconds at 2x. Non-1x clock/rate behavior deserves measurement; this session does not establish exact 2x timing or A/V alignment.

Native build/vet/full tests and race-enabled player/media tests passed. The startup regression was checked against reintroducing the old pre-decoder wait. Exact seeking, backward stepping, final scrub commit, native I/O cancellation, hardware modes and encrypted-disc acceptance remain open. These results support the targeted startup/resume repair; broad MPV/VLC parity is not claimed. Candidate integration with current master and CI/release remains outstanding.


## Source/presentation ownership candidate — 2026-10-12 (not landed)

Implemented source-owned load and presentation lifetimes, cancellation/join before replacement transport, serialized native operations, guarded callbacks/EOF reload, partial-open cleanup and consistent failure state. Paused seek and step reads use cancellation. Runtime media was a user-supplied local MP4; full filenames and titles are kept outside the repository.

Automated real-window checks exposed two failures missed by headless tests:
1. Worker held player mu across SetFrame/Refresh; Fyne synchronously waited for its main thread, whose queued run-current check needed the same mutex. Goroutine stacks confirmed the cycle. Repaired by placing frame refresh in the guarded main-thread callback outside player mu.
2. Close during loading stalled in AudioPlayer.Close waiting for the paused decoder. Close had closed decodeStop then replaced it with nil, allowing the decoder to miss retirement. Timeout stacks confirmed the paused loop and waiting cleanup. The channel is now immutable; sync.Once gates complete cleanup. Scrubber start/stop are also idempotent because construction and the wrapper both requested startup.

Final automated software-decode sequence: play, six rapid pause/play pairs, paused forward step, seek to 8 seconds, play, close, start another load, close it after 25 ms, wait for cancellation, reopen, play at 1x/2x, close. Completed with 398 progress frames, pathCleared=true and playing=false. All three source sessions retired. Close-during-load returned cancellation. Logs and diagnostic dumps remained in temporary directories outside the repository.

This run is automated evidence only. User reported being away and missed both visible checks; there is no visual/audio sign-off for this candidate. Do not land/publish it yet. One known seek accurate-fallback warning remains: requested 8 seconds displayed 7.875. After rapid resumes, the paused-step readout reached 23.891 despite a much shorter playback segment; resume queue/clock continuity needs a dedicated repair and measurement. These findings remain open rather than being called successful navigation. Hardware decode, native I/O interruption, encrypted-disc playback and synchronized comparison are not accepted by this run.


## Dev95 UI/playback verification — 2026-10-12

Final integrated candidate: local session 16:02–16:04 (America/Toronto).
Reusing the earlier sample exposed a preexisting load stall: Start launches
demuxing, but Resume starts the frame-queue decoder. Restoring a saved position
called NextFrame before that producer existed and blocked Convert metadata.
The loader now checks Seek and uses the existing timed GrabFrame path, retaining
the initial preview on failure. Behavioral tests cover direct preview decoding,
seek failure and preview failure without starting playback.

The same saved-position sample now loaded and published metadata. Play advanced
through about 54s; Pause held the frame/time while labeled tools were opened and
closed. Closing preview freed space for metadata while Load Video/source identity
remained available. Closing the window removed the candidate process. The log
records saved position 45.61s, preview PTS 41.675s and first playback PTS 43.410s;
the initial display still read 0:00 before Play. A transient visibly distorted
frame was observed during resumed playback, followed by a clean paused frame.
These are unresolved navigation/decode observations, not playback acceptance.
No WARN/ERROR appeared in this session; that does not negate the visual defect.
Native build/vet/full tests and non-native build passed after the loading fix.

The evidence below is the earlier presentation candidate, before integration
with the dev94 cancellation foundation; it is retained as separate evidence.

The metadata-first Convert presentation was checked in a local native preview
with large text and VCR OSD Mono. Empty SMPTE, metadata open/closed, preview
open/closed, persistent source loading and the labeled Player tools dialog
were inspected at restored and maximized window sizes. The empty metadata
header was found to ignore its saved fold state; its initial and programmatic
arrow state is now synchronized.

A user-supplied local AVI sample (512×384, MPEG-4 video, MP3 audio, about 15m27s)
was loaded through the new source action. Play presented real changing frames;
the visible position advanced to about 49s; Pause held the displayed frame and
time while the tools dialog was opened/closed and preview visibility changed.
The preview process exited after its window was closed. This is a real session
for this UI slice, not player transport/lifecycle or encrypted-disc acceptance.

Reviewed the on-disk runtime session covering approximately 15:03–15:09 local
(America/Toronto). It records decoder/audio initialization, Resume and the first
20 frame reads, not an indefinite per-frame logging loop. No WARN/ERROR/CRASH
entry occurred in that interval. This absence alone is not correctness proof:
the first reported frame PTS was about 3.504s after resume from 0, and recorded
clock/PTS deltas vary. Those observations remain in the coordinated VT_Player
repair/measurement scope. A/V synchronization, seek, VFR, EOF and encrypted-disc
playback were not accepted by this session.

Native build/vet/full tests and the non-native build passed. The latter exposed
an existing missing IsPlaying stub; the no-op method now matches the native
API. Linux runtime, the full resolution/scaling/font matrix and Inuktitut
translation review remain outstanding. Local preview DLL provisioning was test
infrastructure only; Windows release packaging remains three static binaries.

## Dev94 candidate playback session — 2026-10-12

A temporary visible Fyne verification window used the actual InlineVideoPlayer/FFmpeg/oto path with user-supplied local MP4 media, software decode and the cancellation candidate. Sequence: play 5 seconds, pause 1 second, resume 4 seconds, 2x for 3 seconds, close, allow teardown 2 seconds. Delivered 243 progress frames. Engine close was logged approximately 3 ms after the close request; the session completed normally. Logs remained outside the repo in the temporary candidate-session directory; media titles and full paths are not committed.

Log review: zero ERROR, WARN and frame-DROP lines; normal milestones, resume and engine-close records observed. User confirmed playback/audio looked correct, then reported slight jitter at the end. The final segment was 2x; correlation is not a proven cause. Basic session completion supports this cancellation slice, not smoothness or full lifecycle acceptance. Hardware modes, accurate seeking, overlapping source loads, rapid transport stress and synchronized comparison remain unverified.

Implemented: interruptible empty/paused frame reads and PTS waits; speed-aware capped waits; generation recheck after pacing; retired reads discarded by playback. Tests cover cancellation/retirement, future PTS, EOF separation, speed timing and detached-frame/EOF suppression. Full native build/vet/tests passed.



## Current audit — 2026-10-12 (dev93 baseline)

The dev55 status below is historical implementation reporting, not current runtime sign-off. [VT_PLAYER_AUDIT_DEV93.md](VT_PLAYER_AUDIT_DEV93.md) records 15 open findings including incorrect seek flags, uncommitted scrubbing, competing consumers, stale loads and uncancellable waits. Native build/vet/full tests passed. No current playback session or profiling was performed; the one-line February log artifact is insufficient. Player fixes must add actual session evidence here before landing. The historical seekGen logging defect remains fixed.

Rolling checklist of known issues, fixes applied, and remaining work for the native media player.
Update this file whenever a player issue is found or fixed.

---

## Current Status (dev55)

| Check | State |
|-------|-------|
| SMPTE bars display when no video loaded | ✅ |
| First frame displays after Load() | ✅ |
| SW decode — H.264, HEVC, AV1, MPEG4, VP9 | ✅ |
| HW decode — D3D11VA (Windows) | ✅ Default-on; SEH bridge catches AV; degrades to SW on failure |
| HW decode — VAAPI (Linux) | ✅ Default-on; same SEH/error path |
| HW decode — QSV | ⚠️ Available but less tested |
| HW→SW degradation on failure | ✅ DegradeToSoftware() wired into decode loop |
| Seek / scrub | ✅ Independent formatCtx per scrubber |
| Seek accuracy modes (Fast/Frame/Precise) | ✅ User-selectable in Settings → Player |
| Audio sync (master clock) | ✅ Audio-driven clock; WaitForPTS in video path |
| Audio delay (A/V offset) | ✅ ±5000 ms; persists in Settings |
| Volume / mute | ✅ |
| Speed control | ✅ Clock + atempo pitch-correct filter |
| Pitch correction at non-1.0× speed | ✅ AudioFilterGraph (libavfilter atempo) |
| bwdif deinterlace | ✅ Auto on flagged frames; Settings toggle |
| Frame stepping (forward) | ✅ |
| Frame stepping (backward) | ✅ Seek-back + decode-forward |
| A-B loop | ✅ SetLoopPoints / SetABLoopEnabled |
| Resume / watch-later | ✅ Auto-save every 5s; restore on load |
| Network streaming (HTTP/HLS/RTSP/RTMP) | ✅ OpenURL with AVDictionary defaults |
| Growing-file support | ✅ Poll + reload on EOF growth |
| Clock drift correction | ✅ SetTime monotonic ratchet; underrun recovery |
| Error ring buffer | ✅ 16-entry; GetErrorHistory() |
| Error concealment (last-good-frame) | ✅ Frozen frame on decode SEH/HW fatal |
| Subtitle display (ASS/SRT) | ✅ ASS time/escape bugs fixed dev50 |
| OpenAuto (file + disc fallback) | ✅ Open() → OpenDVD() auto |
| Close() / Load() lifecycle | ✅ WaitGroup + mutex gate |
| seekLoop goroutine lifecycle | ✅ |
| HW sws context cached | ✅ Keyed on (format, width, height) |
| Duration() lock safety | ✅ formatMu held |
| HDR tone-mapping | ✅ libavfilter: zscale(linear,npl=1000)→tonemap(hable)→zscale(bt709)→yuv420p; PQ+HLG detected; graceful fallback if zscale unavailable |
| Mid-playback audio track switching | ✅ SelectAudioTrack: close player first, reinit codec, seek to current PTS, resume if playing |
| Mid-playback subtitle track switching | ✅ SelectSubtitleTrack: subtitleCodecMu, flush queue, reinit codec, clear stale overlay |
| VFR (variable frame rate) | ⚠️ PTS-based timing handles it in principle; not stress-tested |
| Error resilience (libavcodec) | ✅ `setVideoCodecErrorFlags`: `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` explicit on both video codec init paths |
| Playlist / sequential play | ✅ `Enqueue(path)` / `ClearPlaylist()` / `PlaylistLen()`; auto-advance on EOF; manual `Load` resets queue |
| Per-codec HW blacklist UI | ✅ `PrefsConfig.HWCodecDenyList` + Settings → Player text field; `SetHWCodecDenyList` wires into `codecCanUseHWDevice` |
| Decode loop CPU spin fixed | ✅ `TimedGet(20ms)` replaces `TryGet()` + 1ms poll — no more 100% CPU when queue empty |
| Seek-on-resume stutter fixed | ✅ `FlushAudioCodec()` replaces full `Seek()` on unpause — removes 50-200ms stutter |
| Slider update congestion fixed | ✅ Throttled to ~15fps (66ms min interval) — reduces GUI thread pressure |
| Per-frame subtitle lock eliminated | ✅ `hasSubtitleActive` atomic bool replaces `subtitleCodecMu` lock/unlock 30x/sec |
| sws_scale performance improved | ✅ `SWS_FAST_BILINEAR` replaces `SWS_BICUBIC` for same-res intermediate conversion |
| Decode loop paused check optimized | ✅ `pausedAtomic` (atomic.Bool) replaces `lockMu()` for paused flag check |
| seekGen log spam crash fixed | ✅ `lastSeekGen = gen` after comparison — no more 60×/sec "first frame after seek" log spam that killed the process via I/O pressure |

---

## Known Issues (dev55)

### Strategic: libVLC Backend (HD approved)

The custom FFmpeg engine has had 10+ crash-fix cycles across dev43–dev55. Its architecture (6 goroutines, 4 mutexes, 3 packet queues, frame queue, CGo SEH bridge) is fundamentally too complex to stabilise for user-facing playback. Decision: replace with libVLC for user-facing playback; FFmpeg engine stays as long-term plan. Design doc: `docs/VLC_PLAYER.md`.

Key finding from dev55 crash analysis: every seek to 9.50/9.40/9.30 lands on `pts=9.066` (same keyframe). The decode-after-seek pipeline is unstable — rapid seeking causes the process to die silently (no SEH, no panic logged). The root cause is architectural complexity, not a single bug.

### P0 — User-visible bugs

- [x] **Error/loading/buffering overlay indicators never rendered** — `loadingSpinner`, `bufferingLabel`, `errorLabel`, `errorIndicator` were created and mutated by `SetLoading`/`SetBuffering`/`SetError`/`ClearError` but never included in `videoPlayerRenderer.Objects()` or positioned in `Layout()`. Callers like `inline_player.go:380` and `inline_player.go:983` called `SetError(...)` / `SetLoading(true)` but the user never saw anything. Fixed: all four widgets added to `Objects()`, `Layout()` positions them centred over the video area with proper z-ordering. Loading spinner shows during file open, buffering label shows during buffer underrun, red circle + error message shows on decode/stream errors.
- [x] **Stub method-set divergence** — `inline_player_stub.go` was missing `SetSeekAccuracy`, `SetAudioDelay`, `SetFilterPipeline`, `GetLastVideoPTS`, `GetLastAudioPTS`, `Enqueue`, `ClearPlaylist`, `PlaylistLen`, `SetPeer`. All nine methods added to the stub so both build targets have the identical method set.

### P1 — Playback correctness

- [x] **HDR content washed-out on SDR displays** — Fixed: `isFrameHDR` detects PQ (SMPTE 2084) and HLG (ARIB STD B67) via `frame->color_trc`. `renderSWFrame` applies HDR tone-mapping before sws_scale: libavfilter pipeline `zscale(t=linear,npl=1000)→format(gbrpf32le)→tonemap(hable,desat=0.5)→zscale(t=bt709,m=bt709)→format(yuv420p)`. If zscale/tonemap are unavailable (missing libzimg), `hdrTonemapUnsupported` is set and the frame renders without tone-mapping (avoids retrying on every frame).
- [x] **Audio track cannot be switched mid-playback** — Fixed: `SelectAudioTrack` now closes the old `AudioPlayer` before freeing `audioCodecCtx` (was a use-after-free); reinits codec with `thread_count=1`; seeks to current video PTS; resumes if engine was playing.
- [x] **Subtitle track cannot be switched mid-playback** — Fixed: `SelectSubtitleTrack` flushes `subtitleQueue`, frees old `subtitleCodecCtx` under `subtitleCodecMu`, calls `initSubtitleDecoder` for the new stream, and clears the stale overlay. `decodeSubtitle` and the `demuxerLoop` check are also guarded by `subtitleCodecMu`.
- [ ] **VFR not stress-tested** — PTS-based `WaitForPTS` handles variable frame rates correctly in theory: each frame carries its own PTS, and the clock waits for exactly that timestamp regardless of whether the interval is constant. The critical path (`NextFrame` → `WaitForPTS(pts)`) is frame-interval-agnostic. Known risk: the `preDecodeFrames=8` buffer may under-buffer during high-rate bursts (e.g. screen recordings at 60fps variable), causing micro-stutters. Needs field testing with: screen recordings, web video captures (YouTube/Twitch downloads), variable-rate game captures.

### P2 — Quality / performance

- [x] **Error resilience not set** — Fixed: `setVideoCodecErrorFlags()` called before `avcodec_open2` on both video codec init paths (`SelectVideoTrack` and `openFinalize` SW/HW paths). Sets `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` explicitly so motion-vector extrapolation and deblocking are applied to concealed macroblocks on corrupt or streamed content.
- [x] **Per-codec HW blacklist hardcoded** — Fixed: `hwCodecDenyList` package-level map populated by `SetHWCodecDenyList(s)`. `codecCanUseHWDevice` checks deny-list first. `PrefsConfig.HWCodecDenyList` persists across sessions. Settings → Player shows "HW Decode Deny-List" text entry (comma-separated codec names, e.g. `vc1,wmv3`). Loaded at startup via `initNativeMediaAssets`.
- [x] **No playlist / sequential playback** — Fixed: `Enqueue(path string)` appends to an internal queue; `ClearPlaylist()` empties it; `PlaylistLen()` reports remaining items. On clean EOF, `playbackLoop` checks for a queued item: if found, loads and auto-plays it (without resetting the playlist); otherwise reloads the current file as before. Direct `Load`/`LoadDVD`/`LoadURL` calls reset the playlist so a new manual load starts fresh.
- [ ] **QSV (Intel Quick Sync) less tested** — Detection works; frame transfer and decode path not specifically validated.

---

## Fixed (dev50)

- [x] **HDR tone-mapping** — `hdr.go`: `isFrameHDR` checks `color_trc` (SMPTE 2084 / ARIB STD B67). `renderSWFrame` applies `applyHDRTonemap` before `ensureSwsCtx`+`toRGBA`. Filter graph: `zscale(t=linear,npl=1000)→format(gbrpf32le)→tonemap(hable,desat=0.5)→zscale(t=bt709,m=bt709)→format(yuv420p)`. `hdrTonemapUnsupported` flag suppresses retry when zscale is unavailable. Applied in both `GrabFrame` and `videoDecodeLoop` SW and HW→SW-fallback paths via `renderSWFrame`.
- [x] **Per-codec HW deny-list** — `hwCodecDenyList` map + `SetHWCodecDenyList(s)`. `codecCanUseHWDevice` checks deny-list before allowlist. `PrefsConfig.HWCodecDenyList` (JSON) + Settings → Player text field. Loaded at startup.
- [x] **Error resilience flags** — `setVideoCodecErrorFlags()`: `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` set explicitly before `avcodec_open2` on both video codec init paths (`SelectVideoTrack`, `openFinalize`).
- [x] **Mid-playback audio track switching** — `SelectAudioTrack`: close old `AudioPlayer` before `avcodec_free_context` (was use-after-free); reinit codec `thread_count=1`; seek to current video PTS; resume if playing. Restores speed/volume/muted on new player.
- [x] **Mid-playback subtitle track switching** — `SelectSubtitleTrack`: flush `subtitleQueue`, free old `subtitleCodecCtx` under `subtitleCodecMu`, call `initSubtitleDecoder` for new stream, clear stale overlay. Added `subtitleCodecMu` to Engine; all subtitle codec access (demuxerLoop, NextFrame, decodeSubtitle, Close) guarded.
- [x] **HW decode default-on** — `hwDecodeEnabled` flipped to `true`. All FFmpeg call sites in the video decode path are SEH-wrapped. DegradeToSoftware() wired in.
- [x] **Error concealment (last-good-frame)** — `Engine.lastGoodFrame` stores the most recently displayed frame. On decode error EOF, NextFrame returns the frozen frame once instead of going black.
- [x] **ASS subtitle centiseconds wrong** — `formatASSTime`: `(int(d.Milliseconds()) % 1000) / 10`.
- [x] **ASS subtitle closing-brace over-escape** — `escapeASSText`: removed `}` → `\}`.
- [x] **P1-4: Speed + pitch correction** — atempo filter via AudioFilterGraph.
- [x] **P1-3: A/V offset** — `audioDelayBits` atomic; `WaitForPTS(pts + avDelay)`.
- [x] **P1-6 + P1-9: SeekAccuracy + Player Settings tab** — Dropdown + HW decode toggle in Settings → Player.
- [x] **P1-11: Clock drift** — SetTime monotonic ratchet with underrun recovery.
- [x] **P1-10: Growing-file** — poll + reload on EOF size growth.
- [x] **P1-8: Frame timing overlay** — per-frame PTS/delta displayed in overlay.
- [x] **P1-7: Bilinear scaling** — `sws_scale` uses SWS_BICUBIC confirmed.
- [x] **P1-5: A-B loop** — SetLoopPoints/SetABLoopEnabled wired into NextFrame.
- [x] **P1-2: Resume/watch-later** — auto-save/restore in InlineVideoPlayer.
- [x] **P1-1: Network streaming** — OpenURL with AVDictionary options.
- [x] **HW sws context per-frame allocation** — `hwSwsCtx` cached by (format, width, height) in `retrieveHWFrame`.
- [x] **Duration() lock race** — `lockFormatMu()` held in `Engine.Duration()`.
- [x] **P0-1–P0-5** — All critical fixes (DegradeToSoftware, NextFrame hang, backward step, error ring buffer, OpenAuto).

## Fixed (dev49)

- [x] **Frame pacing fix** — `WaitForPTS` in no-audio path; `WaitVsync` removed.
- [x] **Seek corruption** — accurate fallback uses `AVSEEK_FLAG_BACKWARD`.
- [x] **Player singleton consolidation** — 10 singletons → 2 (`GetPrimaryPlayer` / `GetPreviewPlayer`).
- [x] **Lock hierarchy formalised** — `mu → formatMu → videoCodecMu → framepoolMu`; lockdep build tag.
- [x] **Thread safety** — named lock helpers across all 6 engine files.

## Fixed (dev44)

- [x] **HW-fallback SW path wrong pixel format** — `e.frame.format` not `videoCodecCtx.pix_fmt`.
- [x] **SmoothScrubbing formatCtx race** — independent `AVFormatContext` per scrubber.
- [x] **Audio queue not flushed before scrub seek** — `handleSeek` calls `flushEngineQueues()`.
- [x] **Volume/mute not working** — `applyVolumeS16` in `Read()`; removed duplicate from decode loop.

## Fixed (dev43)

- [x] **SW decode SIGSEGV after ~5 frames** — `ensureSwsCtx(frame.format)` not `videoCodecCtx.pix_fmt`.
- [x] **Close()/demuxerLoop use-after-free** — `demuxerWg` gates codec/format teardown.
- [x] **NextFrame/Close codec race** — `videoCodecMu` held during teardown.
- [x] **seekLoop goroutine leak** — `seekCh` closed and reallocated per `Load()`.

## Fixed (dev42)

- [x] **D3D11VA get_format enum mismatch** — accept both `AV_PIX_FMT_D3D11` and `AV_PIX_FMT_D3D11VA_VLD`.
- [x] **AV_NOPTS_VALUE crash** — skip frames with invalid PTS or zero dimensions.
- [x] **GrabFrame deadlock** — removed redundant `Lock()` in skip path.
- [x] **A/V clock double-speed** — removed `pts * e.speed` in NextFrame; clock handles speed.
- [x] **Pause spin-loop** — 50ms sleep when paused.
- [x] **HW frame buffer race** — dedicated `hwRgbaFrame`/`hwRgbaBuffer` for HW path.
- [x] **Lazy swsCtx creation** — deferred until first frame.

---

## Videos Tested

| Video | Container | Codec | HW | dev50 | Notes |
|-------|-----------|-------|----|-------|-------|
| Gravity - 2K Trailer.mp4 | MP4 | H.264/AC3 | D3D11VA | needs re-test | dev44: improved; audio less choppy |
| ECW Terry Funk vs Cactus Jack.mp4 | MOV | H.264 | SW | needs re-test | dev43: pix_fmt fix applied |
| 2 Minutes.mp4 | MP4 | AV1 | SW | needs re-test | dev43: pix_fmt fix applied |
| Audio Sync.mp4 | MP4 | H.264 50fps | SW | needs re-test | dev43: A/V sync verified |

> All entries marked "needs re-test" should be smoke-tested before closing dev50.
> Add new rows here whenever a file is tested — include date, result, and any notes.

---

## Architecture Notes

### Lock Hierarchy

| Level | Mutex | Protects |
|-------|-------|----------|
| 1 | `mu` | General state: running, paused, speed, looping, chapters, deinterlaceEnabled |
| 2 | `formatMu` | `AVFormatContext` — av_read_frame vs avformat_seek_file |
| 3 | `videoCodecMu` | `videoCodecCtx` send/receive, `e.frame`, `e.swsCtx` |
| 4 | `framepoolMu` | framePool byte-slice reuse |

Always acquire in ascending order. `lockdep` build tag enables runtime verification.

### Decode Paths

**SW path (default):**
`avcodec_receive_frame` → `ensureSwsCtx(frame.format)` → `toRGBA()` → frameQueue

**HW path (D3D11VA/VAAPI/QSV):**
`avcodec_receive_frame` → `av_hwframe_transfer_data` → cached `hwSwsCtx` → `sws_scale` → frameQueue

**Degradation:** On HW SEH/fatal, `DegradeToSoftware()` clears `hw_device_ctx`, resets `get_format`, flushes codec. Loop continues on SW path.

**Error concealment:** `decodeErrored` flag + `lastGoodFrame` atomic pointer. On decode-error EOF, NextFrame returns the frozen last frame once before propagating io.EOF.

### Relevant Files

| File | Role |
|------|------|
| `internal/media/engine.go` | Engine struct, Open/Start/Close, openFinalize |
| `internal/media/playback.go` | videoDecodeLoop, NextFrame, GrabFrame, Seek, Step |
| `internal/media/hwdecode.go` | HW device detection, initHWDecode, retrieveHWFrame, codecCanUseHWDevice |
| `internal/media/clock.go` | MasterClock — PTS sync, speed, WaitForPTS, SyncVideo, SetTime ratchet |
| `internal/media/audio.go` | AudioPlayer — oto v3, decode bridge, atempo filter wiring |
| `internal/media/audio_filter.go` | AudioFilterGraph — libavfilter atempo, vt_atempo_process |
| `internal/media/safe_bridge.c` | SEH wrappers: safe_avcodec_send_packet/receive_frame, SafeHWFrameTransfer |
| `internal/media/deinterlace.go` | bwdif filter graph |
| `internal/media/subtitle_engine.go` | Subtitle decode + ASS/SRT render |
| `internal/media/scrub.go` | SmoothScrubbing — private AVFormatContext per scrubber |
| `internal/media/lock.go` | Lock level constants and named helpers |
| `internal/ui/inline_player.go` | InlineVideoPlayer API — Load/Play/Pause/Seek/Close, playbackLoop |
| `native_media.go` | Singleton getters, initNativeMediaAssets |

Final verification: scripts/windows/dev-verify.ps1 passed native build, vet and full tests. Race-enabled internal/ui and internal/media tests passed, including the late-EOF and paused-audio shutdown regressions.

## VT_Player transport and compact controls candidate — 2026-10-13

Branch `codex/vt-player-transport-ui`, based on VideoTools dev100; no new release or master acceptance claimed. The player heading now receives the actual source filename and truncates within a single row. Player tools uses a compact icon with a tooltip and the existing popup. Other module layouts are preserved.

Inline timeline commits user click/drag release to the playback source; programmatic progress cannot trigger seeks, overwrite edits or immediately bounce committed targets back. Fullscreen timeline paints no longer recursively seek; its frames are dispatched to the UI thread with a closed-window guard. Context controls: Space play/pause, Left/Right five-second seeks; fullscreen Escape/F exits. Paused seek/step publishes frames and progress to presentation observers.

Native seeks use installed FFmpeg BACKWARD flags with timestamp units and discard decoded video preroll to the exact target. Retire and join the packet-owning video decoder before codec/queue reset to prevent pre-seek packets reaching the reset codec. Independent review found no concrete join deadlock or fullscreen feedback loop; pending timeline suppression is bounded to three seconds so failed navigation cannot indefinitely suppress actual progress.

Real software-decode probe: paused 45.610 seconds -> 45.612; reverse 5.200 -> 5.205; playing 90.000 -> 90.023. Initial probe emitted an H.264 reference warning; decoder join repair removed that warning in the repeated probe. Final probe passed and source closed. This was muted and does not establish audible or visual acceptance. Temporary media probe was removed. Native build, vet and full tests passed. Player media/UI race tests and the non-native build passed. Human loaded-media title/tools/click-drag/fullscreen/context-key acceptance remains pending. Keep candidate off master until real UI playback and log review.

## Candidate feedback and seek-generation repair — 2026-10-18

Human check of candidate 2d5f96e0: forward seeks worked; backward seek failed, fullscreen cadence looked uneven, and rate changes made audio/video choppy. Normal installed VideoTools dev101 was built from master 57704aad, which does not contain this candidate. Do not describe candidate fixes as present in that release.

Runtime evidence: requested backward target 326 seconds after forward targets 439/621/885; first video frame afterward was 889.056 and preparation timed out. At 1.5x, repeated backward clock corrections were about 1.0 seconds. These establish discontinuity, not a measured fullscreen frame rate.

Repair: packet queues invalidate blocked/in-flight publication at Flush, and demux snapshots queue generations while reading under formatMu. Seek flushes queues before releasing that format boundary. EOF publication also rejects retired reads. Audio dequeues return their generation atomically; codec submission and PCM output reject retired generations, including future timestamps that pass a lower-bound preroll guard. Resampling stays within the codec lock. TimedGet timeout wakeups hold the condition mutex. Tempo processing is serialized with rate changes, an existing filter resets to 1x, and filter buffering consumes available input before returning silence. Fullscreen observes frames on the existing UI dispatch rather than enqueueing a redundant second UI turn.

Validation: native build/vet/full suite and media/UI race checks passed. Final muted real-media probe repeated 439 -> 621 -> 885 -> 326 -> 12 seconds, decoding and resuming twelve frames after each target; all landed within 100ms, readiness passed, and engine closed. Temporary probe removed. This proves the tested transport sequence, not audible rate quality or fullscreen smoothness. Candidate remains off master pending repeated visible/audible acceptance; seek after natural EOF and output-device buffering still need broader acceptance. No new VideoTools release is claimed.
