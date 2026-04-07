# Inspect Module - Feature Gap Analysis

## Status: Complete (dev40)

The Inspect module is now fully featured with all major capabilities implemented:
- ✅ Video metadata display (codec, resolution, framerate, bitrate)
- ✅ Audio stream information
- ✅ Subtitle stream information  
- ✅ Container metadata
- ✅ Chapter information (tabbed UI)
- ✅ Interlacing detection (quick analyze)
- ✅ Native video player (InlineVideoPlayer wired)
- ✅ Error logging (CatInspect)
- ✅ HDR color metadata (transfer, primaries)
- ✅ Cover art export
- ✅ JSON export
- ✅ Metadata editing via FFmpeg

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
| Color transfer (HDR) | ✅ | view.go - PQ, HLG, Rec.1886 |
| Color primaries | ✅ | view.go - Rec.2020, Rec.709 |
| Audio streams | ✅ | view.go |
| Subtitle streams | ✅ | view.go |
| Chapter list (tabbed) | ✅ | view.go - AppTabs |
| Chapter timestamps | ✅ | view.go |
| Cover art export | ✅ | view.go - Export Cover button |
| JSON export | ✅ | view.go - Export JSON button |
| Metadata editing | ✅ | inspect_module.go - SaveMetadata |
| Interlace detection | ✅ | detector.QuickAnalyze |
| Native player | ✅ | GetInspectPlayer() |
| Error logging | ✅ | dev40 |
| GOP size display | ✅ | view.go GetGOPSize() |
| Field order display | ✅ | view.go GetFieldOrder() |

### Completed in dev40
- Chapters moved to dedicated tab with timestamps
- HDR metadata (color_transfer, color_primaries) fully displayed
- Cover art export button wired
- JSON export with full metadata structure
- Metadata editing via FFmpeg (-metadata flags)

## Documentation Alignment

`docs/inspect/README.md` should be updated to reflect:
- Chapters in tab UI
- HDR color transfer/primaries display
- Export Cover button
- Export JSON button  
- Edit Metadata button

## Notes

- All features use only FFmpeg/FFprobe (no external deps)
- Color data critical for AI upscaling - preserves original color characteristics
- Metadata editing uses stream copy to preserve quality