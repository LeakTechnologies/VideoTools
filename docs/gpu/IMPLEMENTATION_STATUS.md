# GPU Rendering Pipeline - Implementation Status

**Last Updated**: 2026-03-20
**Cycle**: v0.1.1-dev35

## Summary

The GPU rendering pipeline has **partial implementation**. Fyne fork additions are complete, but VideoTools integration is incomplete.

| Component | Status | Notes |
|-----------|--------|-------|
| Fyne Fork (TexSubImage2D) | ✅ Complete | All GL backends implemented |
| Fyne Fork (Texture Reuse) | ✅ Complete | Reuses textures when size matches |
| VideoPlayer → UpdatePixels | ✅ Done | SetFrame() uses UpdatePixels when size matches |
| GPU Renderer Integration | ⚠️ Scaffold Only | Not integrated with VideoPlayer |
| Direct GPU Texture Upload | ❌ Not Implemented | Would bypass Fyne entirely |

---

## Component Status

### 1. Fyne Fork (`C:/Users/User/Desktop/Projects/fyne-fork/`)

| Feature | Status | Commits |
|---------|--------|---------|
| `TexSubImage2D` interface | ✅ Done | `f8658a0` |
| `TexSubImage2D` desktop (gl_core.go) | ✅ Done | `f8658a0` |
| `TexSubImage2D` GLES (gl_es.go) | ✅ Done | `f8658a0` |
| `TexSubImage2D` mobile (gl_gomobile.go) | ✅ Done | `f8658a0` |
| `TexSubImage2D` wasm (gl_wasm.go) | ✅ Done | `f8658a0` |
| `UpdatePixels()` method on Raster | ✅ Done | `f8658a0` |
| Texture reuse in `newGlRasterTexture` | ✅ Done | `f56a582` |
| `MouseButton4`, `MouseButton5` | ✅ Done | `7a5a8895c` |

**Remote**: Not pushed (local only)

### 2. VideoTools GPU Scaffolding (`internal/media/gpu/`)

| File | Status | Notes |
|------|--------|-------|
| `renderer.go` | ⚠️ Scaffold | Interface defined, not integrated |
| `opengl.go` | ⚠️ Scaffold | Basic implementation, not wired |
| `d3d11.go` | ⚠️ Scaffold | D3D11 renderer stub |
| `texture.go` | ⚠️ Scaffold | Utility functions, not used |
| `gputhread.go` | ⚠️ Partial | Has TexSubImage2D usage |
| `shortcuts.go` | ✅ Done | Full shortcut handler |
| `seekbar.go` | ✅ Done | ThumbnailCache, VolumeControl |
| `shaders/` | ⚠️ Scaffold | Shader definitions, not compiled |

### 3. VideoPlayer (`internal/media/view.go`)

| Feature | Status | Implementation |
|---------|--------|----------------|
| Frame display | ✅ Working | `canvas.Raster` + `Refresh()` |
| Overlay controls | ✅ Working | Play/pause, seek, volume |
| Keyboard shortcuts | ✅ Working | Space, arrows, F, M, 0-9 |
| Chapter markers | ✅ Working | Green tick marks on seekbar |
| Thumbnail preview | ⚠️ Partial | ThumbnailCache exists |
| **UpdatePixels() usage** | ❌ **Not Used** | Still uses Generator/Refresh |

### 4. Media Engine (`internal/media/engine.go`)

| Feature | Status | Notes |
|---------|--------|-------|
| FFmpeg decode | ✅ Working | HW accel via libav* |
| Frame cache | ✅ Working | `PlaybackFrameCache` |
| Audio sync | ✅ Working | `MasterClock` |
| Seeking | ✅ Working | Frame-accurate |
| Buffer management | ✅ Working | `PacketQueue` |

---

## What's NOT Working (Critical)

### 1. VideoPlayer doesn't use UpdatePixels()

**Current code** (`internal/media/view.go:972-976`):
```go
func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
    if r.raster == nil {
        r.raster = canvas.NewRaster(r.VideoPlayer.draw)  // Generator pattern
    }
    return []fyne.CanvasObject{r.raster, ...}
}
```

**Problem**: Creates Raster with Generator, calls `Refresh()` each frame. Generator runs every frame, creating new RGBA images.

**Solution**: Use `UpdatePixels()` to pass frame data directly:
```go
func (v *VideoPlayer) SetFrame(frame *image.RGBA) {
    if v.raster != nil {
        v.raster.UpdatePixels(frame.Bounds().Dx(), frame.Bounds().Dy(), frame.Pix)
    }
}
```

### 2. GPU Renderer Not Integrated

The `internal/media/gpu/` files are scaffolding but not connected to VideoPlayer:
- `renderer.go` defines interfaces
- `opengl.go`/`d3d11.go` have basic implementations
- But VideoPlayer doesn't use them

### 3. No Direct GPU Path

For true ~60fps, we need either:
- **Option A**: Wire `UpdatePixels()` in VideoPlayer (simplest, Fyne-mediated)
- **Option B**: Bypass Fyne, render directly to OpenGL/D3D11 (complex, requires GL context access)

---

## Integration Points Needed

### For UpdatePixels() (Option A)

1. **VideoPlayer** (`internal/media/view.go`)
   - Change `source` field from `*image.RGBA` to public
   - Add `SetFrame(frame *image.RGBA)` method
   - Use `UpdatePixels()` instead of Refresh

2. **Engine** (`internal/media/engine.go`)
   - Wire `Engine.onFrame` callback to call `VideoPlayer.SetFrame()`

### For Direct GPU Path (Option B - Future)

1. Expose GL context from Fyne fork
2. Create video-specific texture pool
3. Upload frames via `TexSubImage2D`
4. Render via custom GL draw call

---

## Testing Checklist

- [ ] Verify `TexSubImage2D` is called (add GL debug logging)
- [ ] Verify texture reuse (check texture IDs don't change)
- [ ] Profile frame rendering time (should be <16ms for 60fps)
- [ ] Test with various video resolutions
- [ ] Verify GL context is current during render

---

## Dependencies

- **Fyne Fork**: `C:/Users/User/Desktop/Projects/fyne-fork/`
- **go.mod replace**: `fyne.io/fyne/v2 => C:/Users/User/Desktop/Projects/fyne-fork`

---

## Related Documentation

- [PLAYER_ENHANCEMENTS.md](../PLAYER_ENHANCEMENTS.md) - Feature roadmap
- [TODO.md](./TODO.md) - Remaining work items
- [TEXSUBIMAGE_PIPELINE.md](./TEXSUBIMAGE_PIPELINE.md) - Technical details
