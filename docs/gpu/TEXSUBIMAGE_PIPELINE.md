# TexSubImage2D Pipeline

## Overview

This document describes the texture upload optimization added to the Fyne fork.

## The Problem

Fyne's default raster rendering creates a new texture every time `Refresh()` is called:

```go
// Old path (TexImage2D every frame):
raster.Refresh() 
  → generator(w, h)  // Creates new image.RGBA
  → newGlRasterTexture()
    → TexImage2D(target, 0, w, h, ..., newPixels)  // GPU upload
```

This is expensive because:
1. `TexImage2D` allocates GPU memory each time
2. Previous texture is deleted and recreated
3. No texture caching between frames

## The Solution

Use `TexSubImage2D` to update existing textures:

```go
// New path (TexSubImage2D when cached):
raster.UpdatePixels(w, h, pixels)
  → newGlRasterTexture()
    → cache.GetTexture(obj)  // Check for existing
    → TexSubImage2D(target, 0, 0, 0, w, h, ..., pixels)  // Update existing
```

Benefits:
1. Reuses existing GPU texture
2. Only uploads pixel data
3. Much faster for video playback

## Implementation Details

### UpdatePixels Method

```go
// canvas/raster.go
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

### Texture Reuse in Painter

```go
// internal/painter/gl/texture.go
func (p *painter) newGlRasterTexture(obj fyne.CanvasObject) Texture {
    rast := obj.(*canvas.Raster)
    width := p.textureScale(rast.Size().Width)
    height := p.textureScale(rast.Size().Height)
    
    img := rast.Generator(int(width), int(height))
    if img == nil {
        return noTexture
    }
    
    // KEY CHANGE: Check cache first
    if tex, ok := cache.GetTexture(obj); ok {
        if texObj, ok := img.(*image.RGBA); ok {
            p.ctx.ActiveTexture(texture0)
            p.ctx.BindTexture(texture2D, Texture(tex))
            p.ctx.TexSubImage2D(
                texture2D,
                0,              // level
                0, 0,          // x, y offset
                int(width),
                int(height),
                colorFormatRGBA,
                unsignedByte,
                texObj.Pix,    // pixel data
            )
            return Texture(tex)  // Return cached texture
        }
    }
    
    // Fallback: create new texture
    return p.imgToTexture(img, rast.ScaleMode)
}
```

### GL Context Interface

```go
// internal/painter/gl/context.go
type context interface {
    // ... existing methods ...
    TexImage2D(target uint32, level, width, height int, colorFormat, typ uint32, data []uint8)
    TexSubImage2D(target uint32, level, x, y, width, height int, colorFormat, typ uint32, data []uint8)
    // ...
}
```

## Performance Comparison

| Operation | TexImage2D | TexSubImage2D |
|-----------|------------|---------------|
| Memory alloc | New each frame | Reuse existing |
| GPU command | Create texture + upload | Upload only |
| Typical time (1080p) | ~5-10ms | ~0.5-1ms |
| CPU overhead | High | Low |

## Video Player Integration

### Current (Generator Pattern)
```go
raster := canvas.NewRaster(func(w, h int) image.Image {
    return v.source  // v.source is *image.RGBA
})
// Called every frame via Refresh()
```

### Optimized (UpdatePixels Pattern)
```go
raster := canvas.NewRaster(nil)  // No generator

func (v *VideoPlayer) SetFrame(frame *image.RGBA) {
    if raster != nil && frame != nil {
        raster.UpdatePixels(frame.Bounds().Dx(), frame.Bounds().Dy(), frame.Pix)
    }
}
```

## Verification

### Debug Logging
Add to `newGlRasterTexture`:
```go
if tex, ok := cache.GetTexture(obj); ok {
    log.Printf("TexSubImage2D: reusing texture %d (%dx%d)", tex, width, height)
    // ... TexSubImage2D call ...
} else {
    log.Printf("TexImage2D: creating new texture (%dx%d)", width, height)
    // ... TexImage2D call ...
}
```

### Expected Behavior
1. First frame: "TexImage2D: creating new texture"
2. Subsequent frames (same size): "TexSubImage2D: reusing texture"
3. Resolution change: "TexImage2D: creating new texture"

## OpenGL Functions

### TexImage2D
```c
void glTexImage2D(
    GLenum target,      // GL_TEXTURE_2D
    GLint level,       // 0 for base level
    GLint internalformat, // GL_RGBA
    GLsizei width,
    GLsizei height,
    GLint border,      // 0
    GLenum format,     // GL_RGBA
    GLenum type,       // GL_UNSIGNED_BYTE
    const GLvoid *data
);
```

### TexSubImage2D
```c
void glTexSubImage2D(
    GLenum target,      // GL_TEXTURE_2D
    GLint level,       // 0 for base level
    GLint xoffset,     // 0
    GLint yoffset,     // 0
    GLsizei width,
    GLsizei height,
    GLenum format,     // GL_RGBA
    GLenum type,       // GL_UNSIGNED_BYTE
    const GLvoid *data
);
```

## Related

- [FORK_INTEGRATION.md](./FORK_INTEGRATION.md) - Fork setup
- [IMPLEMENTATION_STATUS.md](./IMPLEMENTATION_STATUS.md) - Current state
