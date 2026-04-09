# Subtitle Handling Design

## Overview

This document covers subtitle handling in VideoTools, including passthrough, burn-in, and track selection. Users need fine-grained control over which subtitles are embedded in the output vs. kept as separate tracks.

## Current State

VideoTools currently supports:
- ✅ Subtitle passthrough (soft subtitles)
- ✅ Forced subtitle detection (partial)

Not implemented:
- ❌ Track selection UI for burn-in vs. passthrough
- ❌ Explicit burn-in of specific tracks

---

## Design Vision

### Three Subtitle Modes

1. **Passthrough** — Keep subtitles as separate tracks (soft subs)
2. **Burn-in** — Hardcode specific subtitle track into video
3. **None** — Remove all subtitles

### Track Selection Rules

For each detected subtitle track:
- **Track info:** Language, format, track name
- **Options:** Passthrough, Burn-in, None
- **Forced:** Mark as forced (display even when off)

---

## UI Layout

### Subtitle Tab in Convert

```
┌─────────────────────────────────────────┐
│ Subtitles                         [+]  │
├─────────────────────────────────────────┤
│ Mode: [○ Passthrough ● Burn-in ○ None]  │
├─────────────────────────────────────────┤
│ Detected Tracks:                         │
│ ┌─────────────────────────────────────┐ │
│ │ [1] English - SRT     [Passthrough▼]│ │
│ │ [2] English - SSA     [Passthrough▼]│ │
│ │ [3] Spanish - SRT     [  None   ▼]  │ │
│ │ [4] Commentary - SRT  [  None   ▼]  │ │
│ └─────────────────────────────────────┘ │
├─────────────────────────────────────────┤
│ [ ] Burn forced subtitles only          │
│ [ ] Add audio description track         │
├─────────────────────────────────────────┤
│ Track Selection Behavior:               │
│ First audio: [English ▼]                │
│ When no match: [No subtitles ▼]         │
└─────────────────────────────────────────┘
```

### Track Dropdown Options

For each track:
- **Passthrough** — Keep as soft subtitle track
- **Burn-in** — Hardcode into video
- **None** — Remove from output

---

## Implementation Details

### Subtitle Track Structure

```go
type SubtitleTrack struct {
    Index       int    // Stream index
    ID          string // Unique ID
    Language    string // ISO 639-2 code
    Format      string // srt, ass, ssa, sup, dvbsub
    Name        string // Track name if available
    Forced      bool   // Forced subtitle flag
    Default     bool   // Default track flag
    Selected    string // "passthrough", "burnin", "none"
}
```

### FFmpeg Passthrough (Soft Subs)

```go
func buildSubtitlePassthroughArgs(tracks []SubtitleTrack) []string {
    var args []string
    for _, t := range tracks {
        if t.Selected == "passthrough" {
            // Map subtitle stream directly
            args = append(args, "-map", fmt.Sprintf("0:s:%d", t.Index))
        }
    }
    return args
}
```

### FFmpeg Burn-in

```go
func buildSubtitleBurninArgs(tracks []SubtitleTrack) []string {
    var filters []string
    for _, t := range tracks {
        if t.Selected == "burnin" {
            // Use subtitles filter to burn in
            // For SRT: subtitles=filename=file.srt
            // For ASS/SSA: ass=filename=file.ass
            if t.Format == "srt" {
                filters = append(filters, fmt.Sprintf("subtitles=filename='%s'", t.Filename))
            } else if t.Format == "ass" || t.Format == "ssa" {
                filters = append(filters, fmt.Sprintf("ass=filename='%s'", t.Filename))
            }
        }
    }
    return filters
}
```

### Subtitle Detection

```go
func DetectSubtitles(path string) ([]SubtitleTrack, error) {
    // Use ffprobe to get subtitle streams
    cmd := exec.Command("ffprobe",
        "-v", "quiet",
        "-print_format", "json",
        "-show_streams",
        "-select_streams", "s",
        path)
    
    // Parse output to build SubtitleTrack list
    // Return tracks with language, format, etc.
}
```

---

## Track Selection Behavior

### Auto-selection Rules

Similar to common conversion tools' "Audio and Subtitle Defaults":

```go
type SubtitleBehavior struct {
    PreferredLanguage string // "eng", "fre", etc.
    WhenNoMatch       string // "first", "none", "preferred"
    BurnForcedOnly    bool   // Only burn forced tracks
}

func SelectSubtitleTracks(detected []SubtitleTrack, behavior SubtitleBehavior) []SubtitleTrack {
    var result []SubtitleTrack
    
    // Find preferred language track
    for _, t := range detected {
        if t.Language == behavior.PreferredLanguage {
            if behavior.BurnForcedOnly && t.Forced {
                t.Selected = "burnin"
            } else if !behavior.BurnForcedOnly {
                t.Selected = "passthrough"
            }
            result = append(result, t)
            break
        }
    }
    
    // If no match, apply fallback
    if len(result) == 0 {
        switch behavior.WhenNoMatch {
        case "first":
            if len(detected) > 0 {
                result = append(result, detected[0])
            }
        case "none":
            // Don't add any tracks
        case "preferred":
            // Add any track with forced flag
            for _, t := range detected {
                if t.Forced {
                    t.Selected = "burnin"
                    result = append(result, t)
                    break
                }
            }
        }
    }
    
    return result
}
```

---

## Supported Formats

| Format | FFmpeg Support | Passthrough | Burn-in |
|--------|-----------------|--------------|---------|
| SRT | ✅ | ✅ | ✅ (subtitles filter) |
| ASS/SSA | ✅ | ✅ | ✅ (ass filter) |
| SUP (DVD) | ✅ | ✅ | ✅ (dvbsub filter) |
| WebVTT | ✅ | ✅ | ❌ |
| TTML | ✅ | ✅ | ❌ |

---

## Files to Modify

1. **`main.go`**
   - Add subtitle tab to Convert UI
   - Add track selection dropdowns
   - Add mode toggle (passthrough/burn/none)

2. **`internal/convert/types.go`**
   - Add subtitle config fields

3. **`internal/ffmpeg/probe.go`**
   - Add subtitle detection with ffprobe

4. **`internal/convert/executor.go`**
   - Build subtitle arguments for FFmpeg

5. **i18n**
   - Add subtitle labels

---

## Testing Checklist

- [ ] Detect all subtitle tracks from file
- [ ] Passthrough keeps soft subtitle tracks
- [ ] Burn-in hardcodes selected track
- [ ] None removes all subtitles
- [ ] Forced subtitles detected correctly
- [ ] Language codes displayed correctly
- [ ] Track selection persists in config

---

## Reference

- HandBrake: https://handbrake.fr/docs/en/latest/advanced/subtitles.html

---

*Last updated: 2026-04-09*
*Status: Ready for implementation*