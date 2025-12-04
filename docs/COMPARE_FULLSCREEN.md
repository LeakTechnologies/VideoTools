# Compare Module - Fullscreen Mode

## Overview
The Compare module now includes a **Fullscreen Compare** mode that displays two videos side-by-side in a larger view, optimized for detailed visual comparison.

## Features

### Current (v0.1)
- ✅ Side-by-side fullscreen layout
- ✅ Larger video players for better visibility
- ✅ Individual playback controls for each video
- ✅ File labels showing video names
- ✅ Back button to return to regular Compare view
- ✅ Pink colored header/footer matching Compare module

### Planned (Future - requires VT_Player enhancements)
- ⏳ **Synchronized playback** - Play/Pause both videos simultaneously
- ⏳ **Linked seeking** - Seek to same timestamp in both videos
- ⏳ **Frame-by-frame sync** - Step through both videos in lockstep
- ⏳ **Volume link** - Adjust volume on both players together
- ⏳ **Playback speed sync** - Change speed on both players at once

## Usage

### Accessing Fullscreen Mode
1. Load two videos in the Compare module
2. Click the **"Fullscreen Compare"** button
3. Videos will display side-by-side in larger players

### Controls
- **Individual players**: Each video has its own play/pause/seek controls
- **"Play Both" button**: Placeholder for future synchronized playback
- **"Pause Both" button**: Placeholder for future synchronized pause
- **"< BACK TO COMPARE"**: Return to regular Compare view

## Use Cases

### Visual Quality Comparison
Compare encoding settings or compression quality:
- Original vs. compressed
- Different codec outputs
- Before/after color grading
- Different resolution scaling

### Frame-Accurate Comparison
When VT_Player sync is implemented:
- Compare edits side-by-side
- Check for sync issues in re-encodes
- Validate frame-accurate cuts
- Compare different filter applications

### A/B Testing
Test different processing settings:
- Different deinterlacing methods
- Upscaling algorithms
- Noise reduction levels
- Color correction approaches

## Technical Notes

### Current Implementation
- Uses standard `buildVideoPane()` for each side
- 640x360 minimum player size (scales with window)
- Independent playback state per video
- No shared controls between players yet

### VT_Player API Requirements for Sync
For synchronized playback, VT_Player will need:

```go
// Playback state access
player.IsPlaying() bool
player.GetPosition() time.Duration

// Event callbacks
player.OnPlaybackStateChanged(callback func(playing bool))
player.OnPositionChanged(callback func(position time.Duration))

// Synchronized control
player.SyncWith(otherPlayer *Player)
player.Unsync()
```

### Synchronization Strategy
When VT_Player supports it:
1. **Master-Slave Pattern**: One player is master, other follows
2. **Linked Events**: Play/pause/seek events trigger on both
3. **Position Polling**: Periodically check for drift and correct
4. **Frame-Accurate Sync**: Step both players frame-by-frame together

## Keyboard Shortcuts (Planned)
When implemented in VT_Player:
- `Space` - Play/Pause both videos
- `J` / `L` - Rewind/Forward both videos
- `←` / `→` - Step both videos frame-by-frame
- `K` - Pause both videos
- `0-9` - Seek to percentage (0% to 90%) in both
- `Esc` - Exit fullscreen mode

## UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ < BACK TO COMPARE                                            │ ← Pink header
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Side-by-side fullscreen comparison. Use individual...      │
│                                                              │
│        [▶ Play Both]  [⏸ Pause Both]                        │
│  ─────────────────────────────────────────────────────────  │
│                                                              │
│  ┌─────────────────────────┬─────────────────────────────┐  │
│  │   File 1: video1.mp4    │   File 2: video2.mp4        │  │
│  ├─────────────────────────┼─────────────────────────────┤  │
│  │                         │                             │  │
│  │    Video Player 1       │    Video Player 2           │  │
│  │    (640x360 min)        │    (640x360 min)            │  │
│  │                         │                             │  │
│  │  [Play] [Pause] [Seek]  │  [Play] [Pause] [Seek]      │  │
│  │                         │                             │  │
│  └─────────────────────────┴─────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                                                          ← Pink footer
```

## Future Enhancements

### v0.2 - Synchronized Playback
- Implement master-slave sync between players
- Add "Link" toggle button to enable/disable sync
- Visual indicator when players are synced

### v0.3 - Advanced Sync
- Offset compensation (e.g., if videos start at different times)
- Manual sync adjustment (nudge one video forward/back)
- Sync validation indicator (shows if videos are in sync)

### v0.4 - Comparison Tools
- Split-screen view with adjustable divider
- A/B quick toggle (show only one at a time)
- Difference overlay (highlight changed regions)
- Frame difference metrics display

## Notes
- Fullscreen mode is accessible from regular Compare view
- Videos must be loaded before entering fullscreen mode
- Synchronized controls are placeholders until VT_Player API is enhanced
- Window can be resized freely - players will scale
- Each player maintains independent state for now
