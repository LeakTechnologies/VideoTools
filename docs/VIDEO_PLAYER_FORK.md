# Video Player Fork Plan

## Overview
The video player component will be extracted into a separate project to allow independent development and improvement of video playback controls while keeping VideoTools focused on video processing.

## Current Player Integration
The player is currently embedded in VideoTools at:
- `internal/player/` - Player implementation
- `main.go` - Player state and controls in Convert module
- Preview frame display
- Playback controls (play/pause, seek, volume)

## Fork Goals

### 1. Independent Development
- Develop player features without affecting VideoTools
- Faster iteration on playback controls
- Better testing of player-specific features
- Can be used by other projects

### 2. Improved Controls
Current limitations to address:
- Tighten up video controls
- Better seek bar with thumbnails on hover
- Improved timeline scrubbing
- Keyboard shortcuts for playback
- Frame-accurate stepping
- Playback speed controls
- Better volume control UI

### 3. Clean API
The forked player should expose a clean API:
```go
type Player interface {
    Load(path string) error
    Play()
    Pause()
    Seek(position time.Duration)
    GetFrame(position time.Duration) (image.Image, error)
    SetVolume(level float64)
    Close()
}
```

## Migration Strategy

### Phase 1: Extract to Separate Module
1. Create new repository: `github.com/yourusername/fyne-videoplayer`
2. Copy `internal/player/` to new repo
3. Extract player dependencies
4. Create clean API surface
5. Add comprehensive tests

### Phase 2: Update VideoTools
1. Import fyne-videoplayer as dependency
2. Replace internal/player with external package
3. Update player instantiation
4. Verify all playback features work
5. Remove old internal/player code

### Phase 3: Enhance Player (Post-Fork)
Features to add after fork:
- [ ] Thumbnail preview on seek bar hover
- [ ] Frame-accurate stepping (←/→ keys)
- [ ] Playback speed controls (0.25x to 2x)
- [ ] Improved volume slider
- [ ] Keyboard shortcuts (Space, K, J, L, etc.)
- [ ] Timeline markers
- [ ] Subtitle support
- [ ] Multi-audio track switching

## Technical Considerations

### Dependencies
Current dependencies to maintain:
- Fyne for UI rendering
- FFmpeg for video decoding
- CGO for FFmpeg bindings

### Cross-Platform Support
Player must work on:
- Linux (GNOME, KDE, etc.)
- macOS
- Windows

### Performance
- Hardware acceleration where available
- Efficient frame buffering
- Low CPU usage during playback
- Fast seeking

## Timeline
1. **Week 1-2**: Extract player code, create repo, clean API
2. **Week 3**: Integration testing, update VideoTools
3. **Week 4+**: Enhanced controls and features

## Benefits
- **VideoTools**: Leaner codebase, focus on processing
- **Player**: Independent evolution, reusable component
- **Users**: Better video controls, more reliable playback
- **Developers**: Easier to contribute to either project

## Notes
- Keep player dependency minimal in VideoTools
- Player should be optional - frame display can work without playback
- Consider using player in Compare module for side-by-side playback (future)
