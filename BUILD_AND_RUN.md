# VideoTools - Build and Run Guide

## Quick Start (2 minutes)

### Option 1: Using the Convenience Script (Recommended)

```bash
cd /home/stu/Projects/VideoTools
source scripts/alias.sh
VideoTools
```

This will:
1. Load the convenience commands
2. Build the application (if needed)
3. Run VideoTools GUI

**Available commands after sourcing alias.sh:**
- `VideoTools` - Run the application
- `VideoToolsRebuild` - Force a clean rebuild
- `VideoToolsClean` - Clean all build artifacts

### Option 2: Using build.sh Directly

```bash
cd /home/stu/Projects/VideoTools
bash scripts/build.sh
./VideoTools
```

### Option 3: Using run.sh

```bash
cd /home/stu/Projects/VideoTools
bash scripts/run.sh
```

### Option 4: Windows Cross-Compilation

```bash
cd /home/stu/Projects/VideoTools
bash scripts/build-windows.sh
# Output: dist/windows/VideoTools.exe
```

**Requirements for Windows build:**
- Fedora/RHEL: `sudo dnf install mingw64-gcc mingw64-winpthreads-static`
- Debian/Ubuntu: `sudo apt-get install gcc-mingw-w64`

---

## Making VideoTools Permanent (Optional)

To use `VideoTools` command from anywhere in your terminal:

### For Bash users:
Add this line to `~/.bashrc`:
```bash
source /home/stu/Projects/VideoTools/scripts/alias.sh
```

Then reload:
```bash
source ~/.bashrc
```

### For Zsh users:
Add this line to `~/.zshrc`:
```bash
source /home/stu/Projects/VideoTools/scripts/alias.sh
```

Then reload:
```bash
source ~/.zshrc
```

### After setting up:
From any directory, you can simply type:
```bash
VideoTools
```

---

## What Each Script Does

### build.sh
```bash
bash scripts/build.sh
```

**Purpose:** Builds VideoTools from source with full dependency management

**What it does:**
1. Checks if Go is installed
2. Displays Go version
3. Cleans previous builds and cache
4. Downloads and verifies all dependencies
5. Builds the application
6. Shows output file location and size

**When to use:**
- First time building
- After major code changes
- When you want a clean rebuild
- When dependencies are out of sync

**Exit codes:**
- `0` = Success
- `1` = Build failed (check errors above)

### run.sh
```bash
bash scripts/run.sh
```

**Purpose:** Runs VideoTools, building first if needed

**What it does:**
1. Checks if binary exists
2. If binary missing, runs `build.sh`
3. Verifies binary was created
4. Launches the application

**When to use:**
- Every time you want to run VideoTools
- When you're not sure if it's built
- After code changes (will rebuild if needed)

**Advantages:**
- Automatic build detection
- No manual steps needed
- Always runs the latest code

### alias.sh
```bash
source scripts/alias.sh
```

**Purpose:** Creates convenient shell commands

**What it does:**
1. Adds `VideoTools` command (alias for `scripts/run.sh`)
2. Adds `VideoToolsRebuild` function
3. Adds `VideoToolsClean` function
4. Prints help text

**When to use:**
- Once per shell session
- Add to ~/.bashrc or ~/.zshrc for permanent access

**Commands created:**
```
VideoTools              # Run the app
VideoToolsRebuild       # Force rebuild
VideoToolsClean         # Remove build artifacts
```

---

## Build Requirements

### Required:
- **Go 1.21 or later**
  ```bash
  go version
  ```
  If not installed: https://golang.org/dl

### Recommended:
- At least 2 GB free disk space (for dependencies)
- Stable internet connection (for downloading dependencies)

### Optional:
- FFmpeg (for actual video encoding)
  ```bash
  ffmpeg -version
  ```

## Platform Support

### Linux ✅ (Primary Platform)
- Full support with native build scripts
- Hardware acceleration (VAAPI, NVENC, QSV)
- X11 and Wayland display server support

### Windows ✅ (New in dev14)
- Cross-compilation from Linux: `bash scripts/build-windows.sh`
- Requires MinGW-w64 toolchain for cross-compilation
- Native Windows builds planned for future release
- Hardware acceleration (NVENC, QSV, AMF)

**For detailed Windows setup, see:** [Windows Compatibility Guide](docs/WINDOWS_COMPATIBILITY.md)

---

## Troubleshooting

### Problem: "Go is not installed"
**Solution:**
1. Install Go from https://golang.org/dl
2. Add Go to PATH: Add `/usr/local/go/bin` to your `$PATH`
3. Verify: `go version`

### Problem: Build fails with "CGO_ENABLED" error
**Solution:** The script already handles this with `CGO_ENABLED=0`. If you still get errors:
```bash
export CGO_ENABLED=0
bash scripts/build.sh
```

### Problem: "Permission denied" on scripts
**Solution:**
```bash
chmod +x scripts/*.sh
bash scripts/build.sh
```

### Problem: Out of disk space
**Solution:** Clean the cache
```bash
bash scripts/build.sh
# Or manually:
go clean -cache -modcache
```

### Problem: Outdated dependencies
**Solution:** Clean and rebuild
```bash
rm -rf go.mod go.sum
go mod init git.leaktechnologies.dev/stu/VideoTools
bash scripts/build.sh
```

### Problem: Binary won't run
**Solution:** Check if it was built:
```bash
ls -lh VideoTools
file VideoTools
```

If missing, rebuild:
```bash
bash scripts/build.sh
```

---

## Development Workflow

### Making code changes and testing:

```bash
# After editing code, rebuild and run:
VideoToolsRebuild
VideoTools

# Or in one command:
bash scripts/build.sh && ./VideoTools
```

### Quick test loop:
```bash
# Terminal 1: Watch for changes and rebuild
while true; do bash scripts/build.sh; sleep 2; done

# Terminal 2: Test the app
VideoTools
```

---

## DVD Encoding Workflow

### To create a professional DVD video:

1. **Start the application**
   ```bash
   VideoTools
   ```

2. **Go to Convert module**
   - Click the Convert tile from main menu

3. **Load a video**
   - Drag and drop, or use file browser

4. **Select DVD format**
   - Choose "DVD-NTSC (MPEG-2)" or "DVD-PAL (MPEG-2)"
   - DVD options appear automatically

5. **Choose aspect ratio**
   - Select 4:3 or 16:9

6. **Name output**
   - Enter filename (without .mpg extension)

7. **Add to queue**
   - Click "Add to Queue"

8. **Start encoding**
   - Click "View Queue" → "Start Queue"

9. **Use output file**
   - Output: `filename.mpg`
   - Import into DVDStyler
   - Author and burn to disc

**Output specifications:**

NTSC:
- 720×480 @ 29.97fps
- MPEG-2 video
- AC-3 stereo audio @ 48 kHz
- Perfect for USA, Canada, Japan, Australia

PAL:
- 720×576 @ 25 fps
- MPEG-2 video
- AC-3 stereo audio @ 48 kHz
- Perfect for Europe, Africa, Asia

Both output region-free, DVDStyler-compatible, PS2-compatible video.

---

## Performance Notes

### Build time:
- First build: 30-60 seconds (downloads dependencies)
- Subsequent builds: 5-15 seconds (uses cached dependencies)
- Rebuild with changes: 10-20 seconds

### File sizes:
- Binary: ~35 MB (optimized)
- With dependencies in cache: ~1 GB total

### Runtime:
- Startup: 1-3 seconds
- Memory usage: 50-150 MB depending on video complexity
- Encoding speed: Depends on CPU and video complexity

---

## Cross-Platform Building

### Linux to Windows Cross-Compilation

```bash
# Install MinGW-w64 toolchain
# Fedora/RHEL:
sudo dnf install mingw64-gcc mingw64-winpthreads-static

# Debian/Ubuntu:
sudo apt-get install gcc-mingw-w64

# Cross-compile for Windows
bash scripts/build-windows.sh

# Output: dist/windows/VideoTools.exe
```

### Multi-Platform Build Script

### Multi-Platform Build Script

```bash
#!/bin/bash
# Build for all platforms

echo "Building for Linux..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o VideoTools-linux

echo "Building for Windows..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o VideoTools-windows.exe

echo "Building for macOS..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o VideoTools-mac

echo "Building for macOS ARM64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o VideoTools-mac-arm64

echo "All builds complete!"
ls -lh VideoTools-*
```

## Production Use

For production deployment:

```bash
# Create optimized binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o VideoTools

# Verify it works
./VideoTools

# File size will be smaller with -ldflags
ls -lh VideoTools
```

---

## Getting Help

### Check the documentation:
- `DVD_USER_GUIDE.md` - How to use DVD encoding
- `DVD_IMPLEMENTATION_SUMMARY.md` - Technical details
- `README.md` - Project overview

### Debug a build:
```bash
# Verbose output
bash scripts/build.sh 2>&1 | tee build.log

# Check go environment
go env

# Verify dependencies
go mod graph
```

### Report issues:
Include:
1. Output from `go version`
2. OS and architecture (`uname -a`)
3. Exact error message
4. Steps to reproduce

---

## Summary

**Easiest way:**
```bash
cd /home/stu/Projects/VideoTools
source scripts/alias.sh
VideoTools
```

**That's it!** The scripts handle everything else automatically.

