# Video Player Fork Plan

## Status: COMPLETED ✅
**VT_Player has been forked as a separate project for independent development.**

## Overview
The video player component has been extracted into a separate project (VT_Player) to allow independent development and improvement of video playback controls while keeping VideoTools focused on video processing.

## Current Player Integration
The player is used in VideoTools at:
- Convert module - Video preview and playback
- Compare module - Side-by-side video comparison (as of dev13)
- Inspect module - Single video playback with metadata (as of dev13)
- Preview frame display
- Playback controls (play/pause, seek, volume)

## Fork Goals

### 1. Independent Development
- Develop player features without affecting VideoTools
- Faster iteration on playback controls
- Better testing of player-specific features
- Can be used by other projects

### 2. Improved Controls
Features to develop in VT_Player:
- **Keyframing** - Mark in/out points for trimming and chapter creation
- Tighten up video controls
- Better seek bar with thumbnails on hover
- Improved timeline scrubbing
- Keyboard shortcuts for playback
- Frame-accurate stepping (←/→ keys for frame-by-frame)
- Playback speed controls (0.25x to 2x)
- Better volume control UI
- Timeline markers for chapters
- Visual in/out point indicators

### 3. Clean API
VT_Player should expose a clean API for VideoTools integration:
```go
type Player interface {
    Load(path string) error
    Play()
    Pause()
    Seek(position time.Duration)
    GetFrame(position time.Duration) (image.Image, error)
    SetVolume(level float64)

    // Keyframing support for Trim/Chapter modules
    SetInPoint(position time.Duration)
    SetOutPoint(position time.Duration)
    GetInPoint() time.Duration
    GetOutPoint() time.Duration
    ClearKeyframes()

    Close()
}
```

## VT_Player Development Strategy

### Phase 1: Core Player Features ✅
- [x] Basic playback controls (play/pause/seek)
- [x] Volume control
- [x] Frame preview display
- [x] Integration with VideoTools modules

### Phase 2: Enhanced Controls (Current Focus)
Priority features for Trim/Chapter module integration:
- [ ] **Keyframe markers** - Set In/Out points visually on timeline
- [ ] **Frame-accurate stepping** - ←/→ keys for frame-by-frame navigation
- [ ] **Visual timeline with markers** - Show In/Out points on seek bar
- [ ] **Keyboard shortcuts** - I (in), O (out), Space (play/pause), ←/→ (step)
- [ ] **Export keyframe data** - Return In/Out timestamps to VideoTools

### Phase 3: Advanced Features (Future)
- [ ] Thumbnail preview on seek bar hover
- [ ] Playback speed controls (0.25x to 2x)
- [ ] Improved volume slider with visual feedback
- [ ] Chapter markers on timeline
- [ ] Subtitle support
- [ ] Multi-audio track switching
- [ ] Zoom timeline for precision editing

## Technical Considerations

### Dependencies
Current dependencies to maintain:
- Fyne for UI rendering
- FFmpeg for video decoding
- CGO for FFmpeg bindings

### Cross-Platform Support
Player must work on:
- Linux (GNOME, KDE, etc.)
- Windows

### Performance
- Hardware acceleration where available
- Efficient frame buffering
- Low CPU usage during playback
- Fast seeking

## VideoTools Module Integration

### Modules Using VT_Player
1. **Convert Module** - Preview video before conversion
2. **Compare Module** - Side-by-side video playback for comparison
3. **Inspect Module** - Single video playback with detailed metadata
4. **Trim Module** (planned) - Keyframe-based trimming with In/Out points
5. **Chapter Module** (planned) - Mark chapter points on timeline

### Integration Requirements for Trim/Chapter
The Trim and Chapter modules will require:
- Keyframe API to set In/Out points
- Visual markers on timeline showing trim regions
- Frame-accurate seeking for precise cuts
- Ability to export timestamp data for FFmpeg commands
- Preview of trimmed segment before processing

## Benefits
- **VideoTools**: Leaner codebase, focus on video processing
- **VT_Player**: Independent evolution, reusable component, dedicated feature development
- **Users**: Professional-grade video controls, precise editing capabilities
- **Developers**: Easier to contribute, clear separation of concerns

## Development Philosophy
- **VT_Player**: Focus on playback, navigation, and visual controls
- **VideoTools**: Focus on video processing, encoding, and batch operations
- Clean API boundary allows independent versioning
- VT_Player features can be tested independently before VideoTools integration

## Notes
- VT_Player repo: Separate project with independent development cycle
- VideoTools will import VT_Player as external dependency
- Keyframing features are priority for Trim/Chapter module development
- Compare module demonstrates successful multi-player integration
