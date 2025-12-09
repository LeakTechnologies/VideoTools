# VT_Player Integration Notes for Lead Developer

## Project Context

**VideoTools Repository**: https://git.leaktechnologies.dev/Leak_Technologies/VideoTools.git
**VT_Player**: Forked video player component for independent development

VT_Player was forked from VideoTools to enable dedicated development of video playback controls and features without impacting the main VideoTools codebase.

## Current Integration Points

### VideoTools Modules Using VT_Player

1. **Convert Module** - Preview video before/during conversion
2. **Compare Module** - Side-by-side video comparison (2 players)
3. **Inspect Module** - Single video playback with metadata display
4. **Compare Fullscreen** - Larger side-by-side view (planned: synchronized playback)

### Current VT_Player Usage Pattern

```go
// VideoTools calls buildVideoPane() which creates player
videoPane := buildVideoPane(state, fyne.NewSize(320, 180), videoSource, updateCallback)

// buildVideoPane internally:
// - Creates player.Controller
// - Sets up playback controls
// - Returns fyne.CanvasObject with player UI
```

## Priority Features Needed in VT_Player

### 1. Keyframing API (HIGHEST PRIORITY)
**Required for**: Trim Module, Chapter Module

```go
// Proposed API
type KeyframeController interface {
    // Set keyframe markers
    SetInPoint(position time.Duration) error
    SetOutPoint(position time.Duration) error
    ClearInPoint()
    ClearOutPoint()
    ClearAllKeyframes()

    // Get keyframe data
    GetInPoint() (time.Duration, bool)  // Returns position and hasInPoint
    GetOutPoint() (time.Duration, bool)
    GetSegmentDuration() time.Duration  // Duration between In and Out

    // Visual feedback
    ShowKeyframeMarkers(show bool)      // Toggle marker visibility on timeline
    HighlightSegment(in, out time.Duration) // Highlight region between markers
}
```

**Use Case**: User scrubs video, presses `I` to set In point, scrubs to end, presses `O` to set Out point. Visual markers show on timeline. VideoTools reads timestamps for FFmpeg trim command.

### 2. Frame-Accurate Navigation (HIGH PRIORITY)
**Required for**: Trim Module, Compare sync

```go
type FrameNavigationController interface {
    // Step through video frame-by-frame
    StepForward() error   // Advance exactly 1 frame
    StepBackward() error  // Go back exactly 1 frame

    // Frame info
    GetCurrentFrame() int64              // Current frame number
    GetFrameAtTime(time.Duration) int64  // Frame number at timestamp
    GetTimeAtFrame(int64) time.Duration  // Timestamp of frame number
    GetTotalFrames() int64

    // Seek to exact frame
    SeekToFrame(frameNum int64) error
}
```

**Use Case**: User finds exact frame for cut point using arrow keys (←/→), sets In/Out markers precisely.

### 3. Synchronized Playback API (MEDIUM PRIORITY)
**Required for**: Compare Fullscreen, Compare Module sync

```go
type SyncController interface {
    // Link two players together
    SyncWith(otherPlayer player.Controller) error
    Unsync()
    IsSynced() bool
    GetSyncMaster() player.Controller

    // Callbacks for sync events
    OnPlayStateChanged(callback func(playing bool))
    OnPositionChanged(callback func(position time.Duration))

    // Sync with offset (for videos that don't start at same time)
    SetSyncOffset(offset time.Duration)
    GetSyncOffset() time.Duration
}
```

**Use Case**: Compare module loads two videos. User clicks "Play Both" button. Both players play in sync. When one player is paused/seeked, other follows.

### 4. Playback Speed Control (MEDIUM PRIORITY)
**Required for**: Trim Module, general UX improvement

```go
type PlaybackSpeedController interface {
    SetPlaybackSpeed(speed float64) error  // 0.25x to 2.0x
    GetPlaybackSpeed() float64
    GetSupportedSpeeds() []float64         // [0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 2.0]
}
```

**Use Case**: User slows playback to 0.25x to find exact frame for trim point.

## Integration Architecture

### Current Pattern
```
VideoTools (main.go)
    └─> buildVideoPane()
        └─> player.New()
        └─> player.Controller interface
        └─> Returns fyne.CanvasObject
```

### Proposed Enhanced Pattern
```
VideoTools (main.go)
    └─> buildVideoPane()
        └─> player.NewEnhanced()
            ├─> player.Controller (basic playback)
            ├─> player.KeyframeController (trim support)
            ├─> player.FrameNavigationController (frame stepping)
            ├─> player.SyncController (multi-player sync)
            └─> player.PlaybackSpeedController (speed control)
        └─> Returns fyne.CanvasObject
```

### Backward Compatibility
- Keep existing `player.Controller` interface unchanged
- Add new optional interfaces
- VideoTools checks if player implements enhanced interfaces:

```go
if keyframer, ok := player.(KeyframeController); ok {
    // Use keyframe features
}
```

## Technical Requirements

### 1. Timeline Visual Enhancements

Current timeline needs:
- **In/Out Point Markers**: Visual indicators (⬇️ symbols or colored bars)
- **Segment Highlight**: Show region between In and Out with different color
- **Frame Number Display**: Show current frame number alongside timestamp
- **Marker Drag Support**: Allow dragging markers to adjust In/Out points

### 2. Keyboard Shortcuts

Essential shortcuts for VT_Player:

| Key | Action | Notes |
|-----|--------|-------|
| `Space` | Play/Pause | Standard |
| `←` | Step backward 1 frame | Frame-accurate |
| `→` | Step forward 1 frame | Frame-accurate |
| `Shift+←` | Jump back 1 second | Quick navigation |
| `Shift+→` | Jump forward 1 second | Quick navigation |
| `I` | Set In Point | Trim support |
| `O` | Set Out Point | Trim support |
| `C` | Clear keyframes | Reset markers |
| `K` | Pause | Video editor standard |
| `J` | Rewind | Video editor standard |
| `L` | Fast forward | Video editor standard |
| `0-9` | Seek to % | 0=start, 5=50%, 9=90% |

### 3. Performance Considerations

- **Frame stepping**: Must be instant, no lag
- **Keyframe display**: Update timeline without stuttering
- **Sync**: Maximum 1-frame drift between synced players
- **Memory**: Don't load entire video into RAM for frame navigation

### 4. FFmpeg Integration

VT_Player should expose frame-accurate timestamps that VideoTools can use:

```bash
# Example: VideoTools gets In=83.456s, Out=296.789s from VT_Player
ffmpeg -ss 83.456 -to 296.789 -i input.mp4 -c copy output.mp4
```

Frame-accurate seeking requires:
- Seek to nearest keyframe before target
- Decode frames until exact target reached
- Display correct frame with minimal latency

## Data Flow Examples

### Trim Module Workflow
```
1. User loads video in Trim module
2. VideoTools creates VT_Player with keyframe support
3. User navigates with arrow keys (VT_Player handles frame stepping)
4. User presses 'I' → VT_Player sets In point marker
5. User navigates to end point
6. User presses 'O' → VT_Player sets Out point marker
7. User clicks "Preview Trim" → VT_Player plays segment between markers
8. User clicks "Add to Queue"
9. VideoTools reads keyframes: in = player.GetInPoint(), out = player.GetOutPoint()
10. VideoTools builds FFmpeg command with timestamps
11. FFmpeg trims video
```

### Compare Sync Workflow
```
1. User loads 2 videos in Compare module
2. VideoTools creates 2 VT_Player instances
3. User clicks "Play Both"
4. VideoTools calls: player1.SyncWith(player2)
5. VideoTools calls: player1.Play()
6. VT_Player automatically plays player2 in sync
7. User pauses player1 → VT_Player pauses player2
8. User seeks player1 → VT_Player seeks player2 to same position
```

## Testing Requirements

VT_Player should include tests for:

1. **Keyframe Accuracy**
   - Set In/Out points, verify exact timestamps returned
   - Clear markers, verify they're removed
   - Test edge cases (In > Out, negative times, beyond duration)

2. **Frame Navigation**
   - Step forward/backward through entire video
   - Verify frame numbers are sequential
   - Test at video start (can't go back) and end (can't go forward)

3. **Sync Reliability**
   - Play two videos for 30 seconds, verify max drift < 1 frame
   - Pause/seek operations propagate correctly
   - Unsync works properly

4. **Performance**
   - Frame step latency < 50ms
   - Timeline marker updates < 16ms (60fps)
   - Memory usage stable during long playback sessions

## Communication Protocol

### VideoTools → VT_Player

VideoTools will request features through interface methods:

```go
// Example: VideoTools wants to enable trim mode
if trimmer, ok := player.(TrimController); ok {
    trimmer.EnableTrimMode(true)
    trimmer.OnInPointSet(func(t time.Duration) {
        // Update VideoTools UI to show In point timestamp
    })
    trimmer.OnOutPointSet(func(t time.Duration) {
        // Update VideoTools UI to show Out point timestamp
    })
}
```

### VT_Player → VideoTools

VT_Player communicates state changes through callbacks:

```go
player.OnPlaybackStateChanged(func(playing bool) {
    // VideoTools updates UI (play button ↔ pause button)
})

player.OnPositionChanged(func(position time.Duration) {
    // VideoTools updates position display
})

player.OnKeyframeSet(func(markerType string, position time.Duration) {
    // VideoTools logs keyframe for FFmpeg command
})
```

## Migration Strategy

### Phase 1: Core API (Immediate)
- Define interfaces for keyframe, frame nav, sync
- Implement basic keyframe markers (In/Out points)
- Add frame stepping (←/→ keys)
- Document API for VideoTools integration

### Phase 2: Visual Enhancements (Week 2)
- Enhanced timeline with marker display
- Segment highlighting between In/Out
- Frame number display
- Keyboard shortcuts

### Phase 3: Sync Features (Week 3)
- Implement synchronized playback API
- Master-slave pattern for linked players
- Offset compensation for non-aligned videos

### Phase 4: Advanced Features (Week 4+)
- Playback speed control
- Timeline zoom for precision editing
- Thumbnail preview on hover
- Chapter markers

## Notes for VT_Player Developer

1. **Keep backward compatibility**: Existing VideoTools code using basic player.Controller should continue working

2. **Frame-accurate is critical**: Trim module requires exact frame positioning. Off-by-one frame errors are unacceptable.

3. **Performance over features**: Frame stepping must be instant. Users will hold arrow keys to scrub through video.

4. **Visual feedback matters**: Keyframe markers must be immediately visible. Timeline updates should be smooth.

5. **Cross-platform testing**: VT_Player must work on Linux (GNOME/X11/Wayland) and Windows

6. **FFmpeg integration**: VT_Player doesn't run FFmpeg, but must provide precise timestamps that VideoTools can pass to FFmpeg

7. **Minimize dependencies**: Keep VT_Player focused on playback/navigation. VideoTools handles video processing.

## Questions to Consider

1. **Keyframe storage**: Should keyframes be stored in VT_Player or passed back to VideoTools immediately?

2. **Sync drift handling**: If synced players drift apart, which one is "correct"? Should we periodically resync?

3. **Frame stepping during playback**: Can user step frame-by-frame while video is playing, or must they pause first?

4. **Memory management**: For long videos (hours), how do we efficiently support frame-accurate navigation without excessive memory?

5. **Hardware acceleration**: Should frame stepping use GPU decoding, or is CPU sufficient for single frames?

## Current VideoTools Status

### Working Modules
- ✅ Convert - Video conversion with preview
- ✅ Compare - Side-by-side comparison (basic)
- ✅ Inspect - Single video with metadata
- ✅ Compare Fullscreen - Larger view (sync placeholders added)

### Planned Modules Needing VT_Player Features
- ⏳ Trim - **Needs**: Keyframing + frame navigation
- ⏳ Chapter - **Needs**: Multiple keyframe markers on timeline
- ⏳ Merge - May need synchronized preview of multiple clips

### Auto-Compare Feature (NEW)
- ✅ Checkbox in Convert module: "Compare After"
- ✅ After conversion completes, automatically loads:
  - File 1 (Original) = source video
  - File 2 (Converted) = output video
- ✅ User can immediately inspect conversion quality

## Contact & Coordination

For questions about VideoTools integration:
- Review this document
- Check `/docs/VIDEO_PLAYER_FORK.md` for fork strategy
- Check `/docs/TRIM_MODULE_DESIGN.md` for detailed trim module requirements
- Check `/docs/COMPARE_FULLSCREEN.md` for sync requirements

VideoTools will track VT_Player changes and update integration code as new features become available.
