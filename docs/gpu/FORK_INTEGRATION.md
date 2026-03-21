# Fyne Fork Integration

**Fork URL**: `https://git.leaktechnologies.dev/lt_mirror/fyne`
**Local Path**: `C:/Users/User/Desktop/Projects/fyne-fork/`
**Remote**: `ltmirror`

## Commits

| Commit | Description |
|--------|-------------|
| `d55e92004` | painter/gl: add debug logging for TexSubImage2D vs TexImage2D |
| `7a5a8895c` | driver/desktop: add MouseButton4 and MouseButton5 |
| `f56a582e8` | painter/gl: add efficient texture reuse in newGlRasterTexture |
| `f8658a052` | Add TexSubImage2D to GL context and UpdatePixels to Raster |

## Setup

### Initial Clone (done)
```bash
git clone https://git.leaktechnologies.dev/lt_mirror/fyne C:/Users/User/Desktop/Projects/fyne-fork
cd C:/Users/User/Desktop/Projects/fyne-fork
```

### Keeping Updated
```bash
cd C:/Users/User/Desktop/Projects/fyne-fork
git fetch origin  # Get upstream Fyne updates
git merge origin/master  # Merge into lt_mirror/master
git push ltmirror master  # Push to our mirror
```

## VideoTools Integration

### go.mod Replace Directive
```go
replace fyne.io/fyne/v2 v2.7.1 => C:/Users/User/Desktop/Projects/fyne-fork
```

**Note**: Uses local path for development. For CI, use:
```go
replace fyne.io/fyne/v2 v2.7.1 => git.leaktechnologies.dev/lt_mirror/fyne v2.7.4-videotools
```

## Changes Added

### 1. TexSubImage2D (f8658a052)

Added to GL context interface for efficient texture updates:

**`internal/painter/gl/context.go`**:
```go
TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8)
```

Implemented in all backends:
- `gl_core.go` - Desktop OpenGL
- `gl_es.go` - OpenGL ES
- `gl_gomobile.go` - Mobile
- `gl_wasm.go` - WebAssembly

### 2. UpdatePixels Method (f8658a052)

Added to `canvas.Raster`:

**`canvas/raster.go`**:
```go
func (r *Raster) UpdatePixels(width, height int, pixels []byte) {
    r.Generator = func(w, h int) image.Image {
        img := image.NewRGBA(image.Rect(0, 0, width, height))
        if len(pixels) >= width*height*4 {
            copy(img.Pix, pixels)
        }
        return img
    }
    Refresh(r)
}
```

### 3. Texture Reuse (f56a582e8)

In `newGlRasterTexture`, now reuses existing textures when size matches:

**`internal/painter/gl/texture.go`**:
```go
func (p *painter) newGlRasterTexture(obj fyne.CanvasObject) Texture {
    // ... get width/height ...
    
    // Check for existing cached texture
    if tex, ok := cache.GetTexture(obj); ok {
        if texObj, ok := img.(*image.RGBA); ok {
            // Reuse texture, just update pixels
            p.ctx.ActiveTexture(texture0)
            p.ctx.BindTexture(texture2D, Texture(tex))
            p.ctx.TexSubImage2D(..., texObj.Pix)
            return Texture(tex)
        }
    }
    
    // Create new texture if not cached
    return p.imgToTexture(img, rast.ScaleMode)
}
```

### 4. MouseButton4/5 (7a5a8895c)

Added for forward/backward navigation support:

**`driver/desktop/mouse.go`**:
```go
MouseButton4  // 4th button (typically back)
MouseButton5  // 5th button (typically forward)
```

## Building with Fork

```bash
cd C:/Users/User/Desktop/Projects/VideoTools
go mod tidy
go build -o VideoTools.exe .
```

## CI Integration

For CI builds, update `go.mod` to use remote:
```go
replace fyne.io/fyne/v2 v2.7.1 => git.leaktechnologies.dev/lt_mirror/fyne v2.7.4-videotools
```

Then run:
```bash
go mod download git.leaktechnologies.dev/lt_mirror/fyne
go build -o VideoTools.exe .
```

## Troubleshooting

### Build fails with "missing go.sum entry"
```bash
go mod tidy
```

### Wrong Fyne version used
Verify replace directive in `go.mod`:
```bash
grep "replace.*fyne" go.mod
```

### GL errors during rendering
Enable debug logging in GL implementations to verify `TexSubImage2D` is called.
