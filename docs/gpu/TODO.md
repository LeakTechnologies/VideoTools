# GPU Rendering Pipeline - TODO

**Priority**: High
**Status**: Fyne fork complete, integration incomplete

## Immediate Tasks

### 1. Wire UpdatePixels() in VideoPlayer [HIGH]

**Problem**: VideoPlayer uses `canvas.Raster` with Generator/Refresh pattern, triggering TexImage2D every frame.

**Files to modify**: `internal/media/view.go`

**Changes needed**:
- [x] Add `raster` field to VideoPlayer struct
- [x] Add `currentWidth/currentHeight` fields to track texture size
- [x] Use `raster.UpdatePixels()` in SetFrame() when size matches
- [x] Update videoPlayerRenderer to use VideoPlayer.raster
- [x] Build passes

**Code change**:
```go
// Current (slow):
func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
    if r.raster == nil {
        r.raster = canvas.NewRaster(r.VideoPlayer.draw)
    }
    return []fyne.CanvasObject{r.raster, ...}
}

// Target (fast):
func (v *VideoPlayer) SetFrame(frame *image.RGBA) {
    if v.raster != nil && frame != nil {
        v.raster.UpdatePixels(frame.Bounds().Dx(), frame.Bounds().Dy(), frame.Pix)
    }
}
```

**Verification**:
- [ ] Add GL debug logging to verify TexSubImage2D called
- [ ] Profile frame render time (<16ms target for 60fps)
- [ ] Test with 1080p, 4K, various codecs

---

### 2. Wire Engine → VideoPlayer Frame Callback [HIGH]

**Problem**: Engine produces frames, but VideoPlayer doesn't receive them efficiently.

**Files to modify**: `internal/media/engine.go`, `internal/media/view.go`

**Changes needed**:
- [ ] Add `OnFrame(callback func(*image.RGBA))` to VideoPlayer
- [ ] Wire Engine to call VideoPlayer.SetFrame()
- [ ] Remove redundant frame copying

---

### 3. Test Texture Reuse [MEDIUM]

**Problem**: Need to verify TexSubImage2D is being used, not TexImage2D.

**Changes needed**:
- [ ] Add debug logging to `newGlRasterTexture`
- [ ] Log when TexSubImage2D vs TexImage2D is called
- [ ] Verify texture IDs don't change on same-size frames

**Test cases**:
- [ ] 1080p video playback (should reuse texture)
- [ ] Resolution change mid-video (should create new texture)
- [ ] Window resize (should handle gracefully)

---

### 4. Benchmark Frame Rendering [MEDIUM]

**Target**: <16ms per frame for 60fps

**Measure**:
- [ ] Average frame render time
- [ ] CPU usage during playback
- [ ] GPU usage during playback
- [ ] Memory allocation rate

---

## Future Tasks (Post dev35)

### 5. GPU Renderer Integration

**Problem**: `internal/media/gpu/` files are scaffolding, not integrated.

**Files**: `renderer.go`, `opengl.go`, `d3d11.go`, `texture.go`

**Scope**:
- [ ] Integrate GPU renderer with VideoPlayer
- [ ] Test OpenGL path
- [ ] Test D3D11 path on Windows
- [ ] Add fallback to software rendering

---

### 6. Direct GPU Texture Upload (Advanced)

**Problem**: Fyne-mediated path still has overhead.

**Goal**: Bypass Fyne, upload directly to GPU.

**Requirements**:
- [ ] Expose GL context from Fyne
- [ ] Create video-specific texture pool
- [ ] Upload via TexSubImage2D
- [ ] Render via custom GL draw

**Complexity**: High - requires deep Fyne fork changes

---

### 7. Shader-Based Scaling

**Problem**: Current scaling uses nearest-neighbor.

**Goal**: Bicubic/cubic interpolation in GPU.

**Files**: `internal/media/gpu/shaders/`

---

## Blocked By

- VideoPlayer refactor (Task #1)
- Engine callback wiring (Task #2)

## Dependencies

- Fyne fork at `C:/Users/User/Desktop/Projects/fyne-fork/`
- `go.mod` replace directive pointing to fork

## Related Docs

- [IMPLEMENTATION_STATUS.md](./IMPLEMENTATION_STATUS.md) - Current state
- [TEXSUBIMAGE_PIPELINE.md](./TEXSUBIMAGE_PIPELINE.md) - Technical details
- [FORK_INTEGRATION.md](./FORK_INTEGRATION.md) - Fork setup
