# GPU Rendering Pipeline Documentation

This folder contains comprehensive documentation for VideoTools' GPU-accelerated video rendering pipeline.

## Documents

| Document | Description |
|----------|-------------|
| [README.md](./README.md) | Overview and architecture |
| [IMPLEMENTATION_STATUS.md](./IMPLEMENTATION_STATUS.md) | Current status of all components |
| [TEXSUBIMAGE_PIPELINE.md](./TEXSUBIMAGE_PIPELINE.md) | Texture upload mechanism details |
| [FORK_INTEGRATION.md](./FORK_INTEGRATION.md) | Fyne fork setup and integration |
| [TODO.md](./TODO.md) | Remaining work items |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      VideoTools App                              │
├─────────────────────────────────────────────────────────────────┤
│  internal/media/                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  Engine    │  │  View       │  │  internal/media/gpu/    │  │
│  │  (decode)  │→ │  (display)  │  │  - renderer.go         │  │
│  └─────────────┘  └──────┬──────┘  │  - opengl.go           │  │
│                          │         │  - d3d11.go            │  │
│                          │         │  - texture.go           │  │
│                          │         │  - gputhread.go        │  │
│                          │         └─────────────────────────┘  │
│                          ↓                                       │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Fyne Canvas (fork.leaktechnologies.dev/fyne)   ││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │  internal/painter/gl/                                  │││
│  │  │  - texture.go (TexSubImage2D + texture reuse)          │││
│  │  │  - context.go (GL interface with TexSubImage2D)        │││
│  │  │  - gl_core.go, gl_es.go, gl_gomobile.go, gl_wasm.go    │││
│  │  └─────────────────────────────────────────────────────────┘││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Key Components

### VideoTools (this repo)

| File | Purpose | Status |
|------|---------|--------|
| `internal/media/engine.go` | FFmpeg decode, frame cache | ✅ Working |
| `internal/media/view.go` | VideoPlayer widget | ✅ Working (uses Refresh) |
| `internal/media/gpu/renderer.go` | GPU renderer interface | ⚠️ Scaffold only |
| `internal/media/gpu/opengl.go` | OpenGL implementation | ⚠️ Scaffold only |
| `internal/media/gpu/d3d11.go` | D3D11 implementation | ⚠️ Scaffold only |
| `internal/media/gpu/texture.go` | Texture utilities | ⚠️ Scaffold only |
| `internal/media/gpu/gputhread.go` | GPU thread helpers | ⚠️ Uses TexSubImage2D |

### Fyne Fork (`C:/Users/User/Desktop/Projects/fyne-fork/`)

| File | Purpose | Status |
|------|---------|--------|
| `canvas/raster.go` | Raster with UpdatePixels() | ✅ Done |
| `internal/painter/gl/context.go` | GL interface + TexSubImage2D | ✅ Done |
| `internal/painter/gl/texture.go` | Texture reuse logic | ✅ Done |
| `internal/painter/gl/gl_core.go` | Desktop OpenGL impl | ✅ Done |
| `internal/painter/gl/gl_es.go` | GLES impl | ✅ Done |
| `internal/painter/gl/gl_gomobile.go` | Mobile impl | ✅ Done |
| `internal/painter/gl/gl_wasm.go` | WebAssembly impl | ✅ Done |
| `driver/desktop/mouse.go` | MouseButton4/5 | ✅ Done |

## Current Rendering Path

```
FFmpeg Decode → *image.RGBA → VideoPlayer.source → canvas.Raster → Refresh() → Generator()
                                                                              ↓
                                                                      TexImage2D (every frame)
                                                                              ↓
                                                                      GL Texture Upload
```

**Problem**: VideoPlayer uses `canvas.NewRaster()` with a `draw` callback, calling `Refresh()` each frame. This triggers `Generator()` and `TexImage2D()` every time.

**Optimized Path** (not yet wired):
```
FFmpeg Decode → *image.RGBA → VideoPlayer → UpdatePixels() → TexSubImage2D (cached)
```

## Next Steps

See [TODO.md](./TODO.md) for detailed remaining work.
