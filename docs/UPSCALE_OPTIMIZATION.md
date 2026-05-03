# Upscale Module Optimization Guide

## Overview

This guide helps hobbyists and semi-professionals get the best results from VideoTools' upscale module.

## One-Click Presets

Choose a preset from the **Workflow Preset** dropdown at the top of the Upscale panel:

| Preset | Input → Output | Best For | AI Model | RIFE |
|--------|----------------|----------|----------|-------|
| Hobbyist SD→HD | 480p → 1080p | Home videos, old footage | Real-ESRGAN x4+ | No |
| Hobbyist HD→HD | 720p → 1080p | Sharpening existing HD | Real-ESRGAN x4+ | No |
| Semi-Pro 1080p→4K | 1080p → 4K | Professional-looking output | Real-ESRGAN General x4v3 | Yes (2x) |
| Anime Optimised | 720p/1080p → 1080p | Anime-specific content | Real-ESRGAN Anime | Yes (2x) |
| Restoration | Any → Match Source | Old/scratchy footage repair | Real-ESRGAN General x4v3 | No |
| Social Media Ready | 720p → 1080p 30fps | YouTube/TikTok/Instagram | Real-ESRGAN x4+ | Yes (2x) |

## AI Model Selection

### Real-ESRGAN (Spatial Upscaling)

**When to use:**
- Increasing resolution (2x, 3x, 4x, 8x)
- General content (Real-ESRGAN x4+)
- Anime content (Anime models)
- Restoration (General x4v3 with denoise)

**Models:**
- `realesrgan-x4plus` — General purpose, balanced
- `realesrgan-x4plus-anime` — Optimised for anime
- `realesr-general-x4v3` — Fast, includes denoise
- `realesrnet-x4plus` — Clean restore, less artifacting

**Tips:**
- Use **AI Scale = Match Target** to respect your resolution preset
- **Output Adjust** (1.0 = default, 0.5 = softer, 2.0 = sharper)
- Enable **Denoise** for old/scratchy footage (General model only)

### Real-CUGAN (Alternative Spatial Upscaling)

**When to use:**
- Alternative to Real-ESRGAN
- Different aesthetic (sometimes sharper, sometimes softer)
- Three variants: `pro`, `se` (standard), `no-denoise`

**Models:**
- `realcugan-pro` — Higher quality, slower
- `realcugan-se` — Standard speed/quality
- `realcugan-no-denoise` — Skip denoise step

### RIFE (Frame Interpolation)

**When to use:**
- Increasing frame rate (23.976 → 60fps, 30 → 60fps)
- Creating smooth slow-motion
- Making 30fps content look like 60fps

**Models:**
- `rife-v4.6` — General purpose
- `rife-v4.13-lite` — Faster, slightly lower quality
- `rife-anime` — Optimised for anime

**Multiplier:**
- **2x** — 30fps → 60fps, 24fps → 48fps
- **4x** — 30fps → 120fps (may look unnatural)

**Tips:**
- RIFE runs **after** AI upscaling (Real-ESRGAN/CUGAN)
- Use with **Motion Interpolation** checkbox
- Best for: sports, gaming, anime, smooth motion

## Encoding Optimization

### Video Codec

| Codec | Use Case | Quality | Speed | File Size |
|-------|----------|---------|-------|------------|
| H.264 | YouTube, compatibility | Good | Fast | Medium |
| H.265 | archival, smaller files | Better | Slower | Small |
| VP9 | Web/video platforms | Good | Very Slow | Small |
| AV1 | Next-gen, future-proof | Best | Extremely Slow | Smallest |

### Encoder Preset

| Preset | Speed | Quality | Use For |
|--------|-------|---------|---------|
| ultrafast | Fastest | Lowest | Testing only |
| superfast/fast | Fast | Low | Quick previews |
| medium | Medium | Good | **Default for most users** |
| slow/slower | Slow | Better | Final renders |
| veryslow | Very Slow | Best | Archival/master copies |

### CRF (Constant Rate Factor)

- **0** — Lossless (huge files)
- **16-18** — Near-lossless, high quality (default)
- **20-23** — High quality, smaller files
- **24-28** — Good quality, much smaller

### Hardware Acceleration

| Option | When to Use | pros | Cons |
|--------|--------------|------|------|
| auto | **Default** — let VideoTools choose | Easy, usually correct | May not pick optimal |
| none | Compatibility, troubleshooting | Works everywhere | Slowest |
| nvenc | NVIDIA GPU present | Very fast, good quality | Requires NVIDIA GPU |
| qsv | Intel integrated graphics | Fast, low power | Intel only |
| vaapi | Linux with AMD/Intel GPU | Fast on Linux | Linux only |

## Workflow Examples

### Example 1: Home Video 480p → 1080p

1. Load your 480p video
2. Select preset: **Hobbyist SD→HD**
3. Click **Upscale Now**

Result: 1080p video, light AI upscale, H.264 encoding.

### Example 2: Anime 720p → 1080p + 60fps

1. Load your anime video
2. Select preset: **Anime Optimised**
3. Click **Upscale Now**

Result: 1080p + 60fps, anime-optimised model + RIFE interpolation.

### Example 3: Professional 1080p → 4K

1. Load your 1080p video
2. Select preset: **Semi-Pro 1080p→4K**
3. (Optional) Tweak: CRF 16, Encoder Preset: slow
4. Click **Upscale Now**

Result: 4K video, high-quality upscale + RIFE interpolation.

### Example 4: Old Footage Restoration

1. Load old/scratchy video
2. Select preset: **Restoration (Old Footage)**
3. Enable **AI Denoise** slider (0.3-0.7)
4. Click **Upscale Now**

Result: Repaired footage, denoised, upscaled.

### Example 5: Social Media Clip (YouTube/TikTok)

1. Load your video
2. Select preset: **Social Media Ready**
3. Verify: 1080p + 30fps output
4. Click **Upscale Now**

Result: Optimised for upload, 1080p 30fps, H.264.

## Troubleshooting

### "AI upscaling tool not found"

**Fix:**
1. Open **Settings → FFmpeg/AI Tools**
2. Click **Install** next to "Real-ESRGAN ncnn Vulkan"
3. Wait for installation to complete
4. Restart VideoTools

### "RIFE interpolation failed"

**Fix:**
1. Open **Settings → FFmpeg/AI Tools**
2. Click **Install** next to "RIFE ncnn Vulkan"
3. Wait for installation to complete
4. Restart VideoTools

### Output video looks "oily" or over-processed

**Fix:**
- Reduce **AI Output Adjust** (try 0.7-0.8)
- Or select a different AI model

### Upscale is very slow

**Fixes:**
- Use **Encoder Preset: fast/medium** instead of slow
- Reduce AI scale (2x instead of 4x)
- Enable **Hardware Acceleration: nvenc** (if NVIDIA GPU)
- Close other GPU-intensive apps

### RIFE makes motion look unnatural

**Fix:**
- Use **2x multiplier** instead of 4x
- Or disable RIFE, use **Motion Interpolation** checkbox only

## Advanced: Custom Workflows

### Combining AI Models + RIFE

1. Choose AI model (Real-ESRGAN or Real-CUGAN)
2. Set **AI Scale** (2x, 3x, 4x, 8x)
3. Enable **RIFE Enabled** checkbox
4. Choose RIFE model and multiplier (2x or 4x)

The pipeline runs: **Input → AI Upscale → RIFE Interpolation → Output**

### Filter Chain Before AI

Enable filters in **Filters module**, then send to Upscale:

1. Open **Filters** module
2. Adjust brightness/contrast/saturation
3. Click **Send to Upscale**
4. Upscale applies filters + AI upscale in one job

## Performance Expectations

| Workflow | Input | Output | Time (1h video) | GPU VRAM |
|-----------|--------|---------|-------------------|----------|
| Hobbyist SD→HD | 480p | 1080p | ~30 min | 2GB |
| Semi-Pro 1080p→4K | 1080p | 4K + RIFE | ~2 hours | 4GB |
| Anime + RIFE | 720p | 1080p 60fps | ~1 hour | 3GB |
| Restoration | Any | Match Source | ~1.5 hours | 4GB |

*Times are estimates for modern GPU (NVIDIA RTX 3060 or better). CPU-only may be 5-10x slower.*

## Summary

- **Start with presets** — they're tuned for common use cases
- **Real-ESRGAN** = spatial upscale (resolution increase)
- **RIFE** = temporal interpolation (frame rate increase)
- **Use both** for best results (4K + 60fps)
- **Hardware acceleration** (nvenc/qsv) speeds up encoding
- **Test with short clips** before processing full videos
