# Auto-Grey Incompatible Codecs Design

## Problem

When users select WebM format, they can select incompatible codecs (H.264, H.265, AAC, MP3) which causes encoding failures.

## Design Principle

**Format is the leading option** - when user selects a format, it determines what codecs/audio are available. This is the natural flow:
1. User picks container format (e.g., WebM, MP4, MKV)
2. Codec options are filtered to only show compatible choices
3. Incompatible codecs are greyed out

This is simpler and more intuitive than trying to fix invalid combinations after the fact.

## WebM Format Constraints

WebM container only supports:
- **Video**: VP9, AV1 (NOT H.264, H.265)
- **Audio**: Vorbis, Opus (NOT AAC, MP3, FLAC)

## Implementation Plan

### 1. Define Compatibility Maps

```go
// formatVideoCodecs maps format extension to compatible video codecs
var formatVideoCodecs = map[string][]string{
    ".mp4":  {"H.264", "H.265", "AV1", "MPEG-2", "Copy"},
    ".mkv":  {"H.264", "H.265", "AV1", "MPEG-2", "Copy"},
    ".mov":  {"H.264", "H.265", "AV1", "MPEG-2", "Copy"},
    ".webm": {"VP9", "AV1"},
    ".mpg":  {"MPEG-2", "Copy"},
}

// formatAudioCodecs maps format extension to compatible audio codecs
var formatAudioCodecs = map[string][]string{
    ".mp4":  {"AAC", "MP3", "FLAC", "Copy"},
    ".mkv":  {"AAC", "MP3", "FLAC", "Opus", "Copy"},
    ".mov":  {"AAC", "MP3", "FLAC", "Copy"},
    ".webm": {"Opus", "Vorbis"},
    ".mpg":  {"MP2", "Copy"},
}
```

### 2. Update UI Selectors

When format is selected:
1. Filter videoCodecSelect to only show compatible options
2. Filter audioCodecSelect to only show compatible options  
3. Auto-select the first compatible video codec
4. Auto-select the first compatible audio codec
5. Grey out options that aren't in the compatible list

### 3. Implementation in main.go

In the convert view (around line 8455 where formatSelect is created):

```go
// When format changes, update codec options
updateCodecCompatibility := func(format formatOption) {
    ext := strings.ToLower(format.Ext)
    compatibleVideo := formatVideoCodecs[ext]
    compatibleAudio := formatAudioCodecs[ext]
    
    // Update video codec select
    videoCodecSelect.UpdateOptions(videoCodecOptions, videoCodecColorMap)
    // Grey out incompatible: call Disable() on options not in compatibleVideo
    
    // Update audio codec select  
    audioCodecSelect.UpdateOptions(audioCodecOptions, audioCodecColorMap)
    // Grey out incompatible: call Disable() on options not in compatibleAudio
    
    // Auto-select compatible codec
    if len(compatibleVideo) > 0 {
        videoCodecSelect.SetSelected(compatibleVideo[0])
    }
    if len(compatibleAudio) > 0 {
        audioCodecSelect.SetSelected(compatibleAudio[0])
    }
}
```

### 4. ColoredSelect Enhancement

Add method to disable specific options:

```go
// DisableOption makes a specific option non-selectable but visible
func (cs *ColoredSelect) DisableOption(option string) {
    // Mark option as disabled in internal state
}

// EnableOption re-enables a previously disabled option
func (cs *ColoredSelect) EnableOption(option string) {
    // Mark option as enabled
}
```

### 5. Snippet Module Integration

The snippet module already has the fallback logic - this should be refactored to use the shared helper:

```go
// GetCompatibleVideoCodec returns a compatible codec for the format
// If current codec is incompatible, returns fallback compatible codec
func GetCompatibleVideoCodec(codec, formatExt string) string {
    compatible := formatVideoCodecs[formatExt]
    for _, c := range compatible {
        if c == codec {
            return codec
        }
    }
    if len(compatible) > 0 {
        return compatible[0]
    }
    return "H.264" // safe default
}
```

## Files to Modify

1. **`main.go`**
   - Add compatibility maps near formatOptions
   - Add GetCompatibleVideoCodec/GetCompatibleAudioCodec helpers
   - Update formatSelect callback to filter codec options
   - Update snippet job to use shared helpers

2. **`internal/ui/components.go`**
   - Add DisableOption/EnableOption to ColoredSelect

3. **`internal/i18n/`** (optional)
   - Add tooltip string for disabled options

## Testing Checklist

- [ ] Select WebM format → Video: only VP9, AV1 available
- [ ] Select WebM format → Audio: only Opus, Vorbis available  
- [ ] Select WebM format → VP9 auto-selected
- [ ] Select MP4 format → All video codecs available
- [ ] Select MP4 format → All audio codecs available
- [ ] Snippet with WebM → Uses VP9/Opus automatically
- [ ] Change format mid-conversion → Codecs update correctly
