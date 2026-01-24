# UnifiedPlayer Testing Checklist

## 🎬 Video Functionality Testing
- [ ] Basic video playback starts without crashing
- [ ] Video frames display correctly in Fyne canvas  
- [ ] Frame rate matches source video (30fps, 24fps, 60fps)
- [ ] Resolution scaling works properly
- [ ] No memory leaks during video playback
- [ ] Clean video-only playback (no audio stream files)

## 🔊 Audio Functionality Testing
- [ ] Audio plays through system speakers
- [ ] Audio volume controls work correctly (0-100%)
- [ ] Mute/unmute functionality works
- [ ] A/V synchronization stays in sync (no drift)
- [ ] Audio works with different sample rates
- [ ] Audio stops cleanly on Stop()/Pause()

## ⚡ Play/Pause/Seek Controls
- [ ] Play() starts both video and audio immediately
- [ ] Pause() freezes both video and audio frames
- [ ] Seek() jumps to correct timestamp instantly
- [ ] Frame stepping works frame-by-frame
- [ ] Resume after pause continues from paused position

## 🛠️ Error Handling & Edge Cases
- [ ] Missing video file shows user-friendly error
- [ ] Corrupted video file handles gracefully  
- [ ] Unsupported format shows clear error message
- [ ] Audio-only files handled without crashes
- [ ] Resource cleanup on player.Close()

## 📊 Performance & Resource Usage
- [ ] CPU usage is reasonable (<50% on modern hardware)
- [ ] Memory usage stays stable (no growing leaks)
- [ ] Smooth playback without stuttering
- [ ] Fast seeking without rebuffering delays
- [ ] Frame extraction is performant at target resolution

## 📋 Cross-Platform Testing  
- [ ] Works on different video codecs (H.264, H.265, VP9)
- [ ] Handles different container formats (MP4, MKV, AVI)
- [ ] Works with various resolutions (720p, 1080p, 4K)
- [ ] Audio works with stereo/mono sources
- [ ] No platform-specific crashes (Linux/Windows/Mac)

## 🔧 Technical Validation
- [ ] FFmpeg process starts with correct args
- [ ] Pipe communication works (video + audio)
- [ ] RGB24 → RGBA conversion is correct
- [ ] oto audio context initializes successfully
- [ ] Frame display loop runs at correct timing
- [ ] A/V sync timing calculations are accurate

## 🎯 Key Success Metrics
- [ ] Video plays without crashes
- [ ] Audio is audible and in sync
- [ ] Seeking is frame-accurate and responsive  
- [ ] Frame stepping works perfectly
- [ ] Resource usage is optimal
- [ ] No memory leaks or resource issues

## 📝 Testing Notes
- **File**: [Test video file used]
- **Duration**: [Video length tested]
- **Resolution**: [Input and output resolutions]
- **Issues Found**: [List any problems discovered]
- **Performance**: [CPU/Memory usage observations]
- **A/V Sync**: [Any sync issues noted]
- **Seek Accuracy**: [Seek performance observations]

## 🔍 Debug Information
- **FFmpeg Args**: [Command line arguments used]
- **Audio Context**: [Sample rate, channels, format]
- **Buffer Sizes**: [Video frame and audio buffer sizes]
- **Error Logs**: [Any error messages during testing]
- **Pipe Status**: [Video/audio pipe communication status]