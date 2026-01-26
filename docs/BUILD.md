# Building VideoTools

VideoTools uses a universal build script that automatically detects your platform and builds accordingly.

---

## Quick Start (All Platforms)

```bash
./scripts/build.sh
```

That's it! The script will:
- ✅ Detect your platform (Linux/macOS/Windows)
- ✅ Build the appropriate executable
- ✅ On Windows: Offer to download FFmpeg automatically

---

## Platform-Specific Details

### Linux

**Prerequisites:**
- Go 1.21+
- FFmpeg (system package)
- CGO build dependencies

**Install FFmpeg:**
```bash
# Fedora/RHEL
sudo dnf install ffmpeg

# Ubuntu/Debian
sudo apt install ffmpeg

# Arch Linux
sudo pacman -S ffmpeg
```

**Build:**
```bash
./scripts/build.sh
```

**Output:** `VideoTools` (native executable)

**Run:**
```bash
./VideoTools
```

---

### macOS

**Prerequisites:**
- Go 1.21+
- FFmpeg (via Homebrew)
- Xcode Command Line Tools

**Install FFmpeg:**
```bash
brew install ffmpeg
```

**Build:**
```bash
./scripts/build.sh
```

**Output:** `VideoTools` (native executable)

**Run:**
```bash
./VideoTools
```

---

### Windows

**Prerequisites:**
- Go 1.21+
- MinGW-w64 (for CGO)
- Git Bash or similar (to run shell scripts)

**Build:**
```bash
./scripts/build.sh
```

The script will:
1. Build `VideoTools.exe`
2. Prompt to download FFmpeg automatically
3. Set up everything in `dist/windows/`

**Output:** `VideoTools.exe` (Windows GUI executable)

**Run:**
- Double-click `VideoTools.exe` in `dist/windows/`
- Or: `./VideoTools.exe` from Git Bash

**Automatic FFmpeg Setup:**
```bash
# The build script will offer this automatically, or run manually:
./scripts/_internal/setup-windows.bat

# Or in PowerShell:
.\scripts\_internal\setup-windows.ps1 -Portable
```

---

## Advanced: Manual Platform-Specific Builds

### Linux/macOS Native Build
```bash
./scripts/build-linux.sh
```

### Windows Cross-Compile (from Linux)
```bash
# Install MinGW first
sudo dnf install mingw64-gcc mingw64-winpthreads-static  # Fedora
# OR
sudo apt install gcc-mingw-w64  # Ubuntu/Debian

# Cross-compile
./scripts/build-windows.sh

# Output: dist/windows/VideoTools.exe (with FFmpeg bundled)
```

---

## Build Options

### Clean Build
```bash
# The build script automatically cleans cache
./scripts/build.sh
```

### Debug Build
```bash
# Standard build includes debug info by default
CGO_ENABLED=1 go build -o VideoTools

# Run with debug logging
./VideoTools -debug
```

### Release Build (Smaller Binary)
```bash
# Strip debug symbols
go build -ldflags="-s -w" -o VideoTools
```

---

## Troubleshooting

### "go: command not found"
Install Go 1.21+ from https://go.dev/dl/

### "CGO_ENABLED must be set"
CGO is required for Fyne (GUI framework):
```bash
export CGO_ENABLED=1
./scripts/build.sh
```

### "ffmpeg not found" (Linux/macOS)
Install FFmpeg using your package manager (see above).

### Windows: "x86_64-w64-mingw32-gcc not found"
Install MinGW-w64:
- MSYS2: https://www.msys2.org/
- Or standalone: https://www.mingw-w64.org/

### macOS: "ld: library not found"
Install Xcode Command Line Tools:
```bash
xcode-select --install
```

---

## Build Artifacts

After building, you'll find:

### Linux/macOS:
```
VideoTools/
└── VideoTools          # Native executable
```

### Windows:
```
VideoTools/
├── VideoTools.exe      # Main executable
└── dist/
    └── windows/
        ├── VideoTools.exe
        ├── ffmpeg.exe      # (after setup)
        └── ffprobe.exe     # (after setup)
```

---

## Development Builds

For faster iteration during development:

```bash
# Quick build (no cleaning)
go build -o VideoTools

# Run directly
./VideoTools

# With debug output
./VideoTools -debug
```

---

## CI/CD

The build scripts are designed to work in CI/CD environments:

```yaml
# Example GitHub Actions
- name: Build VideoTools
  run: ./scripts/build.sh
```

---

**For more details, see:**
- `QUICKSTART.md` - Simple setup guide
- `WINDOWS_SETUP.md` - Windows-specific instructions
- `docs/WINDOWS_COMPATIBILITY.md` - Cross-platform implementation details
