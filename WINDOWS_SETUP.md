# VideoTools - Windows Setup Guide

This guide will help you get VideoTools running on Windows 10/11.

---

## Prerequisites

VideoTools requires **FFmpeg** to function. You have two options:

### Option 1: Install FFmpeg System-Wide (Recommended)

1. **Download FFmpeg**:
   - Go to: https://github.com/BtbN/FFmpeg-Builds/releases
   - Download: `ffmpeg-master-latest-win64-gpl.zip`

2. **Extract and Install**:
   ```cmd
   # Extract to a permanent location, for example:
   C:\Program Files\ffmpeg\
   ```

3. **Add to PATH**:
   - Open "Environment Variables" (Windows Key + type "environment")
   - Edit "Path" under System Variables
   - Add: `C:\Program Files\ffmpeg\bin`
   - Click OK

4. **Verify Installation**:
   ```cmd
   ffmpeg -version
   ```
   You should see FFmpeg version information.

### Option 2: Bundle FFmpeg with VideoTools (Portable)

1. **Download FFmpeg**:
   - Same as above: https://github.com/BtbN/FFmpeg-Builds/releases
   - Download: `ffmpeg-master-latest-win64-gpl.zip`

2. **Extract ffmpeg.exe**:
   - Open the zip file
   - Navigate to `bin/` folder
   - Extract `ffmpeg.exe` and `ffprobe.exe`

3. **Place Next to VideoTools**:
   ```
   VideoTools\
   ├── VideoTools.exe
   ├── ffmpeg.exe       ← Place here
   └── ffprobe.exe      ← Place here
   ```

This makes VideoTools portable - you can run it from a USB stick!

---

## Running VideoTools

### First Launch

1. Double-click `VideoTools.exe`
2. If you see a Windows SmartScreen warning:
   - Click "More info"
   - Click "Run anyway"
   - (This happens because the app isn't code-signed yet)

3. The main window should appear

### Troubleshooting

**"FFmpeg not found" error:**
- VideoTools looks for FFmpeg in this order:
  1. Same folder as VideoTools.exe
  2. FFMPEG_PATH environment variable
  3. System PATH
  4. Common install locations (Program Files)

**Error opening video files:**
- Make sure FFmpeg is properly installed (run `ffmpeg -version` in cmd)
- Check that video file path doesn't have special characters
- Try copying the video to a simple path like `C:\Videos\test.mp4`

**Application won't start:**
- Make sure you have Windows 10 or later
- Check that you downloaded the 64-bit version
- Verify your graphics drivers are up to date

**Black screen or rendering issues:**
- Update your GPU drivers (NVIDIA, AMD, or Intel)
- Try running in compatibility mode (right-click → Properties → Compatibility)

---

## Hardware Acceleration

VideoTools automatically detects and uses hardware acceleration when available:

- **NVIDIA GPUs**: Uses NVENC encoder (much faster)
- **Intel GPUs**: Uses Quick Sync Video (QSV)
- **AMD GPUs**: Uses AMF encoder

Check the debug output to see what was detected:
```cmd
VideoTools.exe -debug
```

Look for lines like:
```
[SYS] Detected NVENC (NVIDIA) encoder
[SYS] Hardware encoders: [nvenc]
```

---

## Building from Source (Advanced)

If you want to build VideoTools yourself on Windows:

### Prerequisites
- Go 1.21 or later
- MinGW-w64 (for CGO)
- Git

### Steps

1. **Install Go**:
   - Download from: https://go.dev/dl/
   - Install and verify: `go version`

2. **Install MinGW-w64**:
   - Download from: https://www.mingw-w64.org/
   - Or use MSYS2: https://www.msys2.org/
   - Add to PATH

3. **Clone Repository**:
   ```cmd
   git clone https://github.com/yourusername/VideoTools.git
   cd VideoTools
   ```

4. **Build**:
   ```cmd
   set CGO_ENABLED=1
   go build -ldflags="-H windowsgui" -o VideoTools.exe
   ```

5. **Run**:
   ```cmd
   VideoTools.exe
   ```

---

## Cross-Compiling from Linux

If you're building for Windows from Linux:

1. **Install MinGW**:
   ```bash
   # Fedora/RHEL
   sudo dnf install mingw64-gcc mingw64-winpthreads-static

   # Ubuntu/Debian
   sudo apt-get install gcc-mingw-w64
   ```

2. **Build**:
   ```bash
   ./scripts/build-windows.sh
   ```

3. **Output**:
   - Executable: `dist/windows/VideoTools.exe`
   - Bundle FFmpeg as described above

---

## Known Issues on Windows

1. **Console Window**: The app uses `-H windowsgui` flag to hide the console, but some configurations may still show it briefly

2. **File Paths**: Avoid very long paths (>260 characters) on older Windows versions

3. **Antivirus**: Some antivirus software may flag the executable. This is a false positive - the app is safe

4. **Network Drives**: UNC paths (`\\server\share\`) should work but may be slower

---

## Getting Help

If you encounter issues:

1. Enable debug mode: `VideoTools.exe -debug`
2. Check the error messages
3. Report issues at: https://github.com/yourusername/VideoTools/issues

Include:
- Windows version (10/11)
- GPU type (NVIDIA/AMD/Intel)
- FFmpeg version (`ffmpeg -version`)
- Full error message
- Debug log output

---

## Performance Tips

1. **Use Hardware Acceleration**: Make sure your GPU drivers are updated
2. **SSD Storage**: Work with files on SSD for better performance
3. **Close Other Apps**: Free up RAM and GPU resources
4. **Preset Selection**: Use faster presets for quicker encoding

---

**Last Updated**: 2025-12-04
**Version**: v0.1.0-dev14
