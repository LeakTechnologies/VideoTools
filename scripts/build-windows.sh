#!/bin/bash
# VideoTools Windows Build Script
# Cross-compiles VideoTools for Windows from Linux

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools.exe"
DIST_DIR="$PROJECT_ROOT/dist/windows"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Windows Build Script (Cross-Compilation)"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

echo "Go version:"
go version
echo ""

# Check if MinGW-w64 is installed
if ! command -v x86_64-w64-mingw32-gcc &> /dev/null; then
    echo "ERROR: MinGW-w64 cross-compiler not found!"
    echo ""
    echo "To install on Fedora/RHEL:"
    echo "  sudo dnf install mingw64-gcc mingw64-winpthreads-static"
    echo ""
    echo "To install on Debian/Ubuntu:"
    echo "  sudo apt-get install gcc-mingw-w64"
    echo ""
    exit 1
fi

echo "MinGW-w64 detected:"
x86_64-w64-mingw32-gcc --version | head -1
echo ""

# Change to project directory
cd "$PROJECT_ROOT"

echo "Cleaning previous Windows builds..."
rm -f "$BUILD_OUTPUT" 2>/dev/null || true
rm -rf "$DIST_DIR" 2>/dev/null || true
echo "Previous builds cleaned"
echo ""

echo "Downloading and verifying dependencies..."
go mod download
go mod verify
echo "Dependencies verified"
echo ""

echo "Cross-compiling for Windows (amd64)..."
echo "   Target: windows/amd64"
echo "   Compiler: x86_64-w64-mingw32-gcc"
echo ""

# Set Windows build environment
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++

# Build flags
# -H windowsgui: Hide console window (GUI application)
# -s -w: Strip debug symbols (smaller binary)
LDFLAGS="-H windowsgui -s -w"

if go build -ldflags="$LDFLAGS" -o "$BUILD_OUTPUT" .; then
    echo "Cross-compilation successful!"
    echo ""
else
    echo "Build failed!"
    exit 1
fi

echo "Creating distribution package..."
mkdir -p "$DIST_DIR"

# Copy executable
cp "$BUILD_OUTPUT" "$DIST_DIR/"
echo "Copied VideoTools.exe"

# Copy documentation
cp README.md "$DIST_DIR/" 2>/dev/null || echo "WARNING: README.md not found"
cp LICENSE "$DIST_DIR/" 2>/dev/null || echo "WARNING: LICENSE not found"

# Download and bundle FFmpeg automatically
if [ ! -f "ffmpeg.exe" ]; then
    echo "FFmpeg not found locally, downloading..."
    FFMPEG_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
    FFMPEG_ZIP="$PROJECT_ROOT/ffmpeg-windows.zip"

    if command -v wget &> /dev/null; then
        wget -q --show-progress "$FFMPEG_URL" -O "$FFMPEG_ZIP"
    elif command -v curl &> /dev/null; then
        curl -L "$FFMPEG_URL" -o "$FFMPEG_ZIP" --progress-bar
    else
        echo "WARNING: wget or curl not found. Cannot download FFmpeg automatically."
        echo "   Please download manually from: $FFMPEG_URL"
        echo "   Extract ffmpeg.exe and ffprobe.exe to project root"
        echo ""
    fi

    if [ -f "$FFMPEG_ZIP" ]; then
        echo "Extracting FFmpeg..."
        unzip -q "$FFMPEG_ZIP" "*/bin/ffmpeg.exe" "*/bin/ffprobe.exe" -d "$PROJECT_ROOT/ffmpeg-temp"

        # Find and copy the executables (they're nested in a versioned directory)
        find "$PROJECT_ROOT/ffmpeg-temp" -name "ffmpeg.exe" -exec cp {} "$PROJECT_ROOT/" \;
        find "$PROJECT_ROOT/ffmpeg-temp" -name "ffprobe.exe" -exec cp {} "$PROJECT_ROOT/" \;

        # Cleanup
        rm -rf "$PROJECT_ROOT/ffmpeg-temp" "$FFMPEG_ZIP"
        echo "FFmpeg downloaded and extracted"
    fi
fi

# Bundle FFmpeg with the distribution
if [ -f "ffmpeg.exe" ]; then
    cp ffmpeg.exe "$DIST_DIR/"
    echo "Bundled ffmpeg.exe"
else
    echo "WARNING: ffmpeg.exe not found - distribution will require separate FFmpeg installation"
fi

if [ -f "ffprobe.exe" ]; then
    cp ffprobe.exe "$DIST_DIR/"
    echo "Bundled ffprobe.exe"
else
    echo "WARNING: ffprobe.exe not found"
fi

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "WINDOWS BUILD COMPLETE"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Output directory: $DIST_DIR"
echo "Contents:"
ls -lh "$DIST_DIR"
echo ""
echo "Windows executable: $DIST_DIR/VideoTools.exe"
echo "Size: $(du -h "$DIST_DIR/VideoTools.exe" | cut -f1)"
echo ""
echo "Next steps:"
echo "  1. Test on Windows 10/11"
echo "  2. Create installer with NSIS (optional)"
echo "  3. Package with FFmpeg for distribution"
echo ""
echo "To download FFmpeg for Windows:"
echo "  wget https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
echo "  unzip ffmpeg-master-latest-win64-gpl.zip"
echo "  cp ffmpeg-master-latest-win64-gpl/bin/ffmpeg.exe ."
echo "  cp ffmpeg-master-latest-win64-gpl/bin/ffprobe.exe ."
echo "  ./scripts/build-windows.sh  # Rebuild to include FFmpeg"
echo ""
