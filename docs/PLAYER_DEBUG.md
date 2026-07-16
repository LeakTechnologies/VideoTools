# Player Debug Status

Rolling checklist of known issues, fixes applied, and remaining work for the native media player.
Update this file whenever a player issue is found or fixed.

---

## Current Status (dev54)

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

---

## Known Issues (dev54)

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
