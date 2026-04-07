# Inspect Module - Feature Gap Analysis

## Status: Mostly Complete

The Inspect module is well-developed with recent improvements:
- ✅ Video metadata display (codec, resolution, framerate, bitrate)
- ✅ Audio stream information
- ✅ Subtitle stream information  
- ✅ Container metadata
- ✅ Chapter information
- ✅ Interlacing detection (quick analyze)
- ✅ Native video player (InlineVideoPlayer wired by Claude)
- ✅ Error logging (CatInspect added dev40)

## Feature Completeness by Category

### Implemented ✅
| Feature | Status | Location |
|---------|--------|-----------|
| Video codec/format | ✅ | view.go |
| Resolution/dimensions | ✅ | view.go |
| Frame rate | ✅ | view.go |
| Aspect ratio (DAR/PAR) | ✅ | view.go |
| Bitrate (video/audio) | ✅ | view.go |
| Duration | ✅ | view.go |
| Pixel format | ✅ | view.go |
| Color space | ✅ | view.go |
| Color range | ✅ | view.go |
| Audio streams | ✅ | view.go |
| Subtitle streams | ✅ | view.go |
| Chapter list | ✅ | view.go |
| Interlace detection | ✅ | detector.QuickAnalyze |
| Native player | ✅ | GetInspectPlayer() |
| Error logging | ✅ | dev40 |

### Partial/Missing ⚠️

| Feature | Status | Notes |
|---------|--------|-------|
| Cover art display | Partial | Embedded in UI, export not wired |
| HDR metadata | ⚠️ | Not fully displayed |
| Variable frame rate | ⚠️ | Detection not exposed |
| Export to text | ⚠️ | Not implemented |
| Export to JSON | ⚠️ | Not implemented |
| Chapter thumbnails | ⚠️ | Not implemented |
| GOP size display | ✅ | view.go GetGOPSize() |
| Field order display | ✅ | view.go GetFieldOrder() |

## Recommended Next Steps (Priority Order)

1. **High Priority**
   - Wire up cover art export functionality
   - Add HDR metadata display fields
   
2. **Medium Priority**
   - Implement text report export
   - Implement JSON export
   - Add chapter thumbnail extraction

3. **Low Priority**
   - Variable frame rate analysis UI
   - MediaInfo integration

## Documentation Alignment

`docs/inspect/README.md` is comprehensive and covers:
- All implemented features ✅
- Future features (marked as planned) ✅
- Usage guide ✅
- Export options (not yet implemented) ⚠️

The documentation is well-aligned with current implementation. No immediate updates needed unless new features are added.