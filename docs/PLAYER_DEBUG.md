# Player Debug Status

Rolling checklist of known issues, fixes applied, and remaining work for the native media player.
Update this file whenever a player issue is found or fixed.

---

## Current Status (dev50)

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
| HDR tone-mapping | ❌ Not implemented — HDR content washed-out on SDR displays |
| Mid-playback audio track switching | ❌ Track locked at open time |
| Mid-playback subtitle track switching | ❌ Track locked at open time |
| VFR (variable frame rate) | ⚠️ PTS-based timing handles it in principle; not stress-tested |
| Error resilience (libavcodec) | ❌ Not set; FF_EC_GUESS_MVS/FF_EC_DEBLOCK not enabled |
| Playlist / sequential play | ❌ Not implemented |
| Per-codec HW blacklist UI | ❌ Allowlist hardcoded (h264/hevc/vp9/av1/vp8) |

---

## Known Issues (dev50)

### P1 — Playback correctness

- [ ] **HDR content washed-out on SDR displays** — No `AV_FRAME_DATA_MASTERING_DISPLAY_METADATA` / `AV_FRAME_DATA_CONTENT_LIGHT_LEVEL` detection. No tone-mapping via libavfilter `zscale` or `tonemap`. All HDR10/HLG sources render without tone-mapping.
- [ ] **Audio track cannot be switched mid-playback** — `audioStreamIdx` is set in `openFinalize()`. Switching requires flushing the audio codec, creating a new `AudioPlayer`, and re-wiring the clock. No API or UI for this yet.
- [ ] **Subtitle track cannot be switched mid-playback** — Same constraint as audio. `subtitleStreamIdx` fixed at open time.
- [ ] **VFR not stress-tested** — PTS-based WaitForPTS should handle variable frame rates correctly in theory. Needs testing with screen recordings and web video.

### P2 — Quality / performance

- [ ] **Error resilience not set** — `videoCodecCtx->error_concealment` defaults to `FF_EC_GUESS_MVS | FF_EC_DEBLOCK`. VT does not explicitly configure this. On corrupt/streamed content, libavcodec may silently drop rather than conceal. Consider setting `FF_EC_GUESS_MVS | FF_EC_DEBLOCK` explicitly.
- [ ] **Per-codec HW blacklist hardcoded** — `codecCanUseHWDevice()` allows only h264/hevc/vp9/av1/vp8. User cannot override. Consider PrefsConfig deny-list.
- [ ] **No playlist / sequential playback** — `InlineVideoPlayer.Enqueue(path)` not implemented. Files must be loaded one at a time.
- [ ] **QSV (Intel Quick Sync) less tested** — Detection works; frame transfer and decode path not specifically validated.

---

## Fixed (dev50)

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
