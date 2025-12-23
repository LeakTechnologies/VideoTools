# VideoTools - Quick Start Guide

Get VideoTools running in minutes!

---

## Windows Users

### Super Simple Setup (Recommended)

1. **Download the repository** or clone it:
   ```cmd
   git clone <repository-url>
   cd VideoTools
   ```

2. **Install dependencies and build** (Git Bash or similar):
   ```bash
   ./scripts/install.sh
   ```

   Or install Windows dependencies directly:
   ```powershell
   .\scripts\install-deps-windows.ps1
   ```

3. **Run VideoTools**:
   ```bash
   ./scripts/run.sh
   ```

### If You Need to Build

If `VideoTools.exe` doesn't exist yet:

**Option A - Get Pre-built Binary** (easiest):
- Check the Releases page for pre-built Windows binaries
- Download and extract
- Run `setup-windows.bat`

**Option B - Build from Source**:
1. Install Go 1.21+ from https://go.dev/dl/
2. Install MinGW-w64 from https://www.mingw-w64.org/
3. Run:
   ```cmd
   set CGO_ENABLED=1
   go build -ldflags="-H windowsgui" -o VideoTools.exe
   ```
4. Run `setup-windows.bat` to get FFmpeg

---

## Linux Users

### Simple Setup

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd VideoTools
   ```

2. **Install dependencies and build**:
   ```bash
   ./scripts/install.sh
   ```

3. **Run**:
   ```bash
   ./scripts/run.sh
   ```

### Cross-Compile for Windows from Linux

Want to build Windows version on Linux?

```bash
# Install MinGW cross-compiler
sudo dnf install mingw64-gcc mingw64-winpthreads-static  # Fedora/RHEL
# OR
sudo apt install gcc-mingw-w64  # Ubuntu/Debian

# Build for Windows (will auto-download FFmpeg)
./scripts/build-windows.sh

# Output will be in dist/windows/
```

---

## macOS Users

### Simple Setup

1. **Install Homebrew** (if not installed):
   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

2. **Clone and install dependencies/build**:
   ```bash
   git clone <repository-url>
   cd VideoTools
   ./scripts/install.sh
   ```

3. **Run**:
   ```bash
   ./scripts/run.sh
   ```

---

## Verify Installation

After setup, you can verify everything is working:

### Check FFmpeg

**Windows**:
```cmd
ffmpeg -version
```

**Linux/macOS**:
```bash
ffmpeg -version
```

### Check VideoTools

Enable debug mode to see what's detected:

**Windows**:
```cmd
VideoTools.exe -debug
```

**Linux/macOS**:
```bash
./VideoTools -debug
```

You should see output like:
```
[SYS] Platform detected: windows/amd64
[SYS] FFmpeg path: C:\...\ffmpeg.exe
[SYS] Hardware encoders: [nvenc]
```

---

## What Gets Installed?

### Portable Installation (Windows Default)
```
VideoTools/
└── dist/
    └── windows/
        ├── VideoTools.exe     ← Main application
        ├── ffmpeg.exe         ← Video processing
        └── ffprobe.exe        ← Video analysis
```

All files in one folder - can run from USB stick!

### System Installation (Optional)
- FFmpeg installed to: `C:\Program Files\ffmpeg\bin`
- Added to Windows PATH
- VideoTools can run from anywhere

### Linux/macOS
- FFmpeg: System package manager
- VideoTools: Built in project directory
- No installation required

---

## Troubleshooting

### Windows: "FFmpeg not found"
- Run `setup-windows.bat` again
- Or manually download from: https://github.com/BtbN/FFmpeg-Builds/releases
- Place `ffmpeg.exe` next to `VideoTools.exe`

### Windows: SmartScreen Warning
- Click "More info" → "Run anyway"
- This is normal for unsigned applications

### Linux: "cannot open display"
- Make sure you're in a graphical environment (not SSH without X11)
- Install required packages: `sudo dnf install libX11-devel libXrandr-devel libXcursor-devel libXinerama-devel libXi-devel mesa-libGL-devel`

### macOS: "Application is damaged"
- Run: `xattr -cr VideoTools`
- This removes quarantine attribute

### Build Errors
- Make sure Go 1.21+ is installed: `go version`
- Make sure CGO is enabled: `export CGO_ENABLED=1`
- On Windows: Make sure MinGW is in PATH

---

## Next Steps

Once VideoTools is running:

1. **Load a video**: Drag and drop any video file
2. **Choose a module**:
   - **Convert**: Change format, codec, resolution
   - **Compare**: Side-by-side comparison
   - **Inspect**: View video properties
3. **Start processing**: Click "Convert Now" or "Add to Queue"

See the full README.md for detailed features and documentation.

---

## Getting Help

- **Issues**: Report at <repository-url>/issues
- **Debug Mode**: Run with `-debug` flag for detailed logs
- **Documentation**: See `docs/` folder for guides

---

**Enjoy VideoTools!** 🎬
