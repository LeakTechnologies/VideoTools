# Upscale Module

The Upscale module raises video resolution using traditional FFmpeg scaling or AI-based Real-ESRGAN (ncnn).

## Status
- AI upscaling is wired through the Real-ESRGAN ncnn backend.
- Traditional scaling is always available.
- Filters and frame rate conversion can be applied before AI upscaling.

## AI Upscaling (Real-ESRGAN ncnn)

### Requirements
- `realesrgan-ncnn-vulkan` in `PATH`.
- Vulkan-capable GPU recommended.

### Pipeline
1. Extract frames from the source video (filters and fps conversion applied here if enabled).
2. Run `realesrgan-ncnn-vulkan` on extracted frames.
3. Reassemble frames into a lossless MKV with the original audio.

### AI Controls
- **Model Preset**
  - General (RealESRGAN_x4plus)
  - Anime/Illustration (RealESRGAN_x4plus_anime_6B)
  - Anime Video (realesr-animevideov3)
  - General Tiny (realesr-general-x4v3)
  - 2x General (RealESRGAN_x2plus)
  - Clean Restore (realesrnet-x4plus)
- **Processing Preset**
  - Ultra Fast, Fast, Balanced (default), High Quality, Maximum Quality
  - Presets tune tile size and TTA.
- **Upscale Factor**
  - Match Target or fixed 1x/2x/3x/4x/8x.
- **Output Adjustment**
  - Post-scale multiplier (0.5x–2.0x).
- **Denoise**
  - Available for `realesr-general-x4v3` (General Tiny).
- **Tile Size**
  - Auto/256/512/800.
- **Output Frames**
  - PNG/JPG/WEBP for frame extraction.
- **Advanced**
  - GPU selection, threads (load/proc/save), and TTA toggle.

### Notes
- Face enhancement requires the Python/GFPGAN backend and is currently not executed.
- AI upscaling is heavier than traditional scaling; use smaller tiles for low VRAM.

## Traditional Scaling
- **Algorithms:** Lanczos, Bicubic, Spline, Bilinear.
- **Target:** Match Source, 2x/4x, or fixed resolutions (720p → 8K).
- **Output:** Lossless MKV by default (copy audio).

## Filters and Frame Rate
- Filters configured in the Filters module can be applied before upscaling.
- Frame rate conversion can be applied with or without motion interpolation.

## Logging
- Each upscale job writes a conversion log in the `logs/` folder next to the executable.
