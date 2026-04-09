# Upscale Module Design & Roadmap

## Overview

The Upscale module provides video resolution enhancement using both traditional FFmpeg algorithms and AI-based neural upscaling via ncnn Vulkan binaries.

## Status

**Last Updated:** v0.1.1-dev40

**Key Changes in Dev40:**
- Added Real-CUGAN support with 3 new models
- Created extensible model catalog architecture
- Implemented dual-binary execution (ESRGAN/CUGAN)

### Implemented Features

| Feature | Status | Implementation |
|---------|--------|----------------|
| Traditional scaling (Lanczos, bicubic, etc.) | ✅ | FFmpeg scale filter |
| Real-ESRGAN ncnn upscaling | ✅ | realesrgan-ncnn-vulkan binary |
| Real-CUGAN ncnn upscaling | ✅ | realcugan-ncnn-vulkan binary (new) |
| Model selection | ✅ | 9 models (6 ESRGAN + 3 CUGAN) |
| Processing presets | ✅ | Ultra Fast → Maximum Quality |
| Scale factor selection | ✅ | 1x-8x or match target |
| Denoise controls | ✅ | AI denoise slider + hqdn3d filter |
| TTA (Test-Time Augmentation) | ✅ | Toggle in advanced settings |
| Tile size control | ✅ | Auto/256/512/800 |
| GPU selection | ✅ | Multi-GPU support |
| Thread tuning | ✅ | load/proc/save threads |
| Output format | ✅ | PNG/JPG/WEBP |
| Frame rate conversion | ✅ | With optional RIFE interpolation |
| Motion interpolation | ✅ | RIFE ncnn integration |
| Color accuracy | ✅ | Color space, depth, pixel format |
| Hardware encoding | ✅ | NVENC/VAAPI/QSV/Videotoolbox |
| Filters integration | ✅ | Pre-upscale filter chain |
| Queue system | ✅ | Background processing |

### Current Model Catalog

```
Real-ESRGAN Models (realesrgan-ncnn-vulkan):
├── General (RealESRGAN x4)          - Default, general content
├── Anime (RealESRGAN x4)            - Animated/illustrated content
├── Anime Video (RealESRGAN)         - Video animation optimization
├── General Fast (RealESRGAN)        - Fast processing with denoise
├── 2x General (RealESRGAN)          - 2x upscale only
└── Clean Restore (RealESRGAN)       - Artifact removal

Real-CUGAN Models (realcugan-ncnn-vulkan):
├── Pro (Real-CUGAN)                 - Highest quality, slower
├── Standard (Real-CUGAN)           - Balanced speed/quality
└── No Denoise (Real-CUGAN)         - Preserve original grain
```

## Known Issues & Improvements Needed

### Priority 1: Critical

1. **Skin tone handling**
   - Real-ESRGAN struggles with skin tones (plastic/artifacted look)
   - Real-CUGAN should help but needs validation
   - Consider adding skin-tone-aware post-processing

2. **Preprocessing pipeline missing**
   - No automatic deinterlacing detection for upscale input
   - No automatic denoise for grainy sources (VHS, 70s content)
   - Need "Archive Content" preset that chains: deinterlace → denoise → upscale

3. **Model selection UX**
   - Users don't know which model to choose
   - Need content-type recommendations (animation vs. live-action vs. archival)

### Priority 2: Important

4. **Scale factor guidance**
   - 4x on 360p → 1440p often looks worse than 2x → 720p
   - Need to recommend scale based on source resolution
   - Default should be 2x for sources below 480p

5. **Batch processing**
   - No way to upscale multiple files with same settings
   - Queue system exists but no batch-add UI for upscale

6. **Preview quality**
   - No real-time preview of AI upscale results
   - Can't preview before committing to long encode

### Priority 3: Nice to Have

7. **Additional models**
   - SPAN (newer architecture)
   - Waifu2x (anime-optimized alternative)
   - RealSR (NTIRE 2020 winner)

8. **Hybrid workflows**
   - Chain multiple models (e.g., denoise + upscale)
   - Combine multiple upscaling passes

9. **Custom model support**
   - Allow users to add custom ncnn models

10. **Progress granularity**
    - Per-frame progress feedback during AI upscale
    - ETA display

## Future Direction

### Phase 1: Content-Aware Presets (Near-term)

```
Proposed Presets:
├── Auto-Detect         - Analyze content, choose best model
├── Animation           - Anime/illustration optimized
├── Live Action         - Real-world footage
└── Archive/Restoration - VHS/old content (deinterlace + denoise + upscale)
```

Implementation:
1. Add content detection (interlaced, noise level, resolution)
2. Auto-recommend settings based on detection
3. Add "Archive" preset that chains preprocessing

### Phase 2: Model Improvements (Mid-term)

1. **Validate Real-CUGAN** for skin-tone content
2. **Add scale recommendations** based on source resolution
3. **Implement preview mode** - upscale single frame for quality check

### Phase 3: Extended Model Support (Long-term)

1. Add SPAN model to catalog
2. Add Waifu2x as alternative
3. Implement custom model loading
4. Support model chaining

## Architecture Notes

### Model Catalog System

The model catalog is designed for extensibility:

```go
type ModelFamily int

const (
    ModelFamilyRealESRGAN ModelFamily = iota
    ModelFamilyRealCUGAN
    // Future: ModelFamilySPAN, ModelFamilyWaifu2x, etc.
)

type ModelInfo struct {
    ID              string
    Label           string      // User-facing name
    Family          ModelFamily
    Binary          string      // Which ncnn binary to use
    ModelName       string      // Model name passed to binary
    SupportsDenoise bool
    SupportsTTA     bool
    DefaultScale    int
}
```

### Adding New Models

To add a new model family (e.g., SPAN):

1. Add to `ModelFamily` enum
2. Add model entries to `modelCatalog`
3. Add detection function (e.g., `DetectSPANAvailable()`)
4. Add binary handling in main.go `aiUpscale()` function
5. Update dependency registration in settings_module.go

### Binary Selection Logic

```go
if strings.HasPrefix(aiModel, "realcugan") {
    aiBinary = "realcugan-ncnn-vulkan"
    // Map UI model ID to binary model name
} else {
    aiBinary = "realesrgan-ncnn-vulkan"
}
```

## Testing Checklist

- [ ] Real-ESRGAN models produce correct output
- [ ] Real-CUGAN models produce correct output
- [ ] Binary selection works (ESRGAN vs CUGAN)
- [ ] Denoise works on supported models
- [ ] TTA produces quality improvement
- [ ] GPU selection works
- [ ] Thread tuning affects performance
- [ ] Frame rate conversion works
- [ ] RIFE interpolation chains after upscale
- [ ] Hardware encoding works
- [ ] Filters apply before upscale
- [ ] Queue system processes upscale jobs
- [ ] Deinterlacing improves interlaced source quality
- [ ] Archive preset chains preprocessing correctly

## Files to Modify

### For Content-Aware Presets
- `internal/app/modules/upscale/view.go` - Add preset UI
- `internal/app/modules/upscale/helpers.go` - Add preset logic
- `main.go` - Add preprocessing chain to upscale pipeline

### For Scale Recommendations
- `internal/app/modules/upscale/view.go` - Add scale recommendations
- `internal/app/modules/upscale/helpers.go` - Add scale calculation

### For Additional Models
- `internal/app/modules/upscale/helpers.go` - Add to modelCatalog
- `main.go` - Add binary handling
- `settings_module.go` - Add dependency registration

## References

- Real-ESRGAN: https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan
- Real-CUGAN: https://github.com/nihui/realcugan-ncnn-vulkan
- ncnn: https://github.com/Tencent/ncnn
- Waifu2x: https://github.com/nihui/waifu2x-ncnn-vulkan
- SPAN: https://github.com/TNTwise/SPAN-ncnn-vulkan