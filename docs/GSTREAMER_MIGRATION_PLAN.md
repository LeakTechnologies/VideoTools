# GStreamer Player Migration Plan

**Goal:** Replace the broken FFmpeg pipe-based player with the robust GStreamer implementation.

**Timeline:** 1-2 days (vs. weeks of debugging pipes)

---

## Phase 1: Make GStreamer a Core Dependency ✅ COMPLETE

### What We Did
- Updated `install.sh` to always install GStreamer dev libraries
- Added verification checks for GStreamer presence
- Updated `build-linux.sh` to require GStreamer and fail if missing
- Updated `build.sh` to always use `-tags gstreamer`

### Verification
```bash
# Check GStreamer is installed
pkg-config --modversion gstreamer-1.0

# Should show version like: 1.24.x
```

### Status
✅ **COMPLETE** - Scripts updated, GStreamer is now mandatory

---

## Phase 2: Build and Verify GStreamer Works

### Tasks
1. **Install GStreamer** (if not already done)
   ```bash
   sudo dnf install -y \
       gstreamer1-devel \
       gstreamer1-plugins-base-devel \
       gstreamer1-plugins-good \
       gstreamer1-plugins-bad-free \
       gstreamer1-plugins-ugly-free \
       gstreamer1-libav
   ```

2. **Build with GStreamer**
   ```bash
   cd /home/stu/Projects/VideoTools
   ./scripts/build.sh
   ```

   **Expected output:**
   ```
   Checking for GStreamer (required for player)...
   GStreamer found (1.24.x)

   Building VideoTools with GStreamer player...
   Build successful!
   ```

3. **Test basic playback**
   ```bash
   ./VideoTools
   # Go to Player module
   # Load a test video
   # Click Play
   ```

### Success Criteria
- ✅ Build completes without GStreamer errors
- ✅ VideoTools launches without crashes
- ✅ Player module loads without errors
- ✅ Can load a video file
- ✅ Basic play/pause works

### Troubleshooting
**Build Error: "Package gstreamer-1.0 was not found"**
- Solution: Run `./scripts/install.sh` to install GStreamer

**Runtime Error: "gstreamer playbin unavailable"**
- Solution: Install GStreamer plugins: `sudo dnf install gstreamer1-plugins-base`

---

## Phase 3: Remove UnifiedPlayer Completely

### Tasks
1. **Delete broken FFmpeg pipe player**
   ```bash
   git rm internal/player/unified_ffmpeg_player.go
   git rm internal/player/unified_player_adapter.go
   ```

2. **Update frame_player_default.go**
   ```go
   // Remove build tag (GStreamer is now always used)
   package player

   func newFramePlayer(config Config) (framePlayer, error) {
       return NewGStreamerPlayer(config)
   }
   ```

3. **Remove unused VTPlayer interface (if applicable)**
   - Check if `vtplayer.go` interface is still needed
   - If not, remove it

4. **Clean up imports**
   - Remove any references to UnifiedPlayer
   - Run `gofmt` and verify build still works

### Success Criteria
- ✅ UnifiedPlayer files deleted
- ✅ No references to UnifiedPlayer in codebase
- ✅ Build still succeeds
- ✅ Player still works

### Verification
```bash
# Search for any remaining UnifiedPlayer references
grep -r "UnifiedPlayer" internal/player/
# Should return nothing (or only comments)

# Rebuild and test
./scripts/build.sh
./VideoTools
```

---

## Phase 4: Fill Gaps in GStreamer Implementation

Your GStreamer player is already 90% complete, but let's verify and add missing pieces.

### Current Status (from gstreamer_player.go)
| Feature | Status | Line # |
|---------|--------|--------|
| Load video | ✅ Complete | 73-162 |
| Play/Pause | ✅ Complete | 164-186 |
| SeekToTime | ✅ Complete | 188-204 |
| SeekToFrame | ✅ Complete | 206-214 |
| GetFrameImage | ✅ Complete | 229-289 |
| SetVolume | ✅ Complete | 291-301 |
| GetCurrentTime | ✅ Complete | 216-227 |
| Close/cleanup | ✅ Complete | 303-319 |

### Missing Features to Add

#### 4.1: Add GetDuration()
```go
func (p *GStreamerPlayer) GetDuration() time.Duration {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.pipeline == nil {
        return 0
    }
    var dur C.gint64
    if C.gst_element_query_duration(p.pipeline, C.GST_FORMAT_TIME, &dur) == 0 {
        return 0
    }
    return time.Duration(dur)
}
```

#### 4.2: Add GetFrameRate()
```go
func (p *GStreamerPlayer) GetFrameRate() float64 {
    p.mu.Lock()
    defer p.mu.Unlock()
    return p.fps
}
```

#### 4.3: Add Stop() method
```go
func (p *GStreamerPlayer) Stop() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.pipeline != nil {
        C.gst_element_set_state(p.pipeline, C.GST_STATE_NULL)
    }
    return nil
}
```

### Tasks
1. Add missing methods above to `gstreamer_player.go`
2. Ensure `framePlayer` interface in `frame_player.go` matches
3. Update `UnifiedPlayerAdapter` if needed (or remove it - see Phase 3)
4. Test each new method

### Success Criteria
- ✅ All interface methods implemented
- ✅ Duration displays correctly in UI
- ✅ Frame rate is accurate
- ✅ Stop button works properly

---

## Phase 5: Test and Validate All Player Features

### Test Matrix

| Feature | Test Case | Expected Result |
|---------|-----------|----------------|
| **Load** | Drop video file | Video loads, shows duration |
| **Play** | Click play | Smooth playback, no stuttering |
| **Pause** | Click pause | Video freezes, audio stops |
| **Seek** | Drag timeline | Jumps to position accurately |
| **Frame Step** | Use arrow keys | Advances 1 frame at a time |
| **Volume** | Adjust slider | Volume changes smoothly |
| **Mute** | Click mute | Audio cuts off completely |
| **Fullscreen** | Press F | Video fills screen |
| **Multiple Formats** | Load MP4, MKV, AVI | All play correctly |
| **High Resolution** | Load 4K video | Plays without freezing |
| **Long Videos** | Load 2+ hour file | Seeking still accurate |

### Performance Tests
1. **CPU Usage** - Should be <20% during playback (check with `htop`)
2. **Memory Leaks** - Run for 30 minutes, memory should stay stable
3. **Frame Drops** - Monitor for dropped frames during playback

### Integration Tests
1. **Trim Module** - Load video, use frame-accurate seeking
2. **Filters Module** - Apply filter, see real-time preview
3. **Preview System** - Generate thumbnails quickly

### Success Criteria
- ✅ All test cases pass
- ✅ No crashes during extended playback
- ✅ Frame-accurate seeking works perfectly
- ✅ CPU/Memory usage is reasonable
- ✅ All video formats supported

---

## Timeline Estimate

| Phase | Time | Blockers |
|-------|------|----------|
| Phase 1 ✅ | Done | None |
| Phase 2 | 30 minutes | Installing GStreamer |
| Phase 3 | 15 minutes | None (just deleting code) |
| Phase 4 | 1-2 hours | Testing each method |
| Phase 5 | 2-3 hours | Thorough testing |
| **Total** | **4-6 hours** | **vs. weeks on pipes** |

---

## What Changed vs. Old Approach

### Old Way (UnifiedPlayer with FFmpeg pipes)
```
❌ Manual pipe management
❌ Manual A/V sync (never worked right)
❌ Audio disabled to "fix" issues
❌ Frame reading blocks UI
❌ Seeking requires process restart
❌ Weeks of debugging
```

### New Way (GStreamer)
```
✅ GStreamer handles pipes internally
✅ Built-in A/V synchronization
✅ Audio works out of the box
✅ Non-blocking frame extraction
✅ Native frame-accurate seeking
✅ Hours of implementation
```

---

## Rollback Plan (If Needed)

If GStreamer has issues (unlikely):

1. **Keep old code temporarily**
   ```bash
   git mv internal/player/unified_ffmpeg_player.go internal/player/unified_ffmpeg_player.go.bak
   ```

2. **Revert build scripts**
   ```bash
   git checkout HEAD -- scripts/build*.sh
   ```

3. **File issue with details**
   - GStreamer version: `pkg-config --modversion gstreamer-1.0`
   - Error message
   - Test video format

But honestly, your GStreamer code is solid. You won't need this.

---

## Key Decision Points

### Should We Keep UnifiedPlayerAdapter?
**Recommendation: DELETE IT**

- It's a compatibility shim for the old player
- GStreamerPlayer already implements the `framePlayer` interface
- Extra layer adds complexity and bugs
- Clean break is better

### What About VTPlayer Interface?
**Recommendation: SIMPLIFY**

Current:
```
framePlayer interface (8 methods) ✅ Used by GStreamer
VTPlayer interface (30+ methods) ❓ Overly complex
```

Keep `framePlayer`, remove or simplify `VTPlayer`.

---

## Post-Migration Cleanup

Once everything works:

1. **Update PROJECT_STATUS.md**
   ```markdown
   | Player | ✅ **Implemented** | GStreamer-based, stable playback |
   ```

2. **Update README.md**
   - Add GStreamer to requirements
   - Note improved player stability

3. **Archive old commits**
   ```bash
   git tag archive/ffmpeg-pipe-player HEAD~20
   git push origin archive/ffmpeg-pipe-player
   ```

4. **Unblock dependent modules**
   - Start Trim module implementation
   - Start Filters module implementation

---

## Emergency Contacts / Resources

- **GStreamer Docs**: https://gstreamer.freedesktop.org/documentation/
- **Go CGO Guide**: https://golang.org/cmd/cgo/
- **Similar Projects**:
  - Kdenlive (uses GStreamer with Qt)
  - Pitivi (uses GStreamer with Python)

---

## Success Definition

You'll know this migration is complete when:

1. ✅ Build always uses GStreamer (no fallback)
2. ✅ All player features work correctly
3. ✅ No UnifiedPlayer code remains
4. ✅ You can implement Trim module without player bugs
5. ✅ PROJECT_STATUS.md shows Player as "Implemented"

**Estimated completion: Tomorrow** (vs. weeks fighting pipes)
