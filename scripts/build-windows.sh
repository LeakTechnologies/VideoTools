#!/bin/bash
# VideoTools Windows Build Script
# Cross-compiles VideoTools for Windows from Linux

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools.exe"
APP_VERSION="$(grep -m1 'appVersion' "$PROJECT_ROOT/main.go" | sed -E 's/.*\"([^\"]+)\".*/\1/')"
[ -z "$APP_VERSION" ] && APP_VERSION="(version unknown)"
GIT_COMMIT=""
if command -v git >/dev/null 2>&1; then
    GIT_COMMIT="$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || true)"
fi
if [ -n "$GIT_COMMIT" ]; then
    FULL_VERSION="${APP_VERSION}_${GIT_COMMIT}"
else
    FULL_VERSION="$APP_VERSION"
fi

channel="${VT_BUILD_CHANNEL:-dev}"
case "${channel,,}" in
    stable|public|release) channel="stable" ;;
    *) channel="dev" ;;
esac

version="$APP_VERSION"
if [ "$channel" = "stable" ]; then
    version="$(echo "$APP_VERSION" | sed -E 's/-dev[0-9]+$//')"
fi
if [ -z "$GIT_COMMIT" ]; then
    GIT_COMMIT="nogit"
fi

os_tag="win"
dist_dir="$PROJECT_ROOT/dist/windows/$channel"
artifact_name="${version}-${GIT_COMMIT}_${os_tag}.zip"

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
rm -rf "$dist_dir" 2>/dev/null || true
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

# Generate Windows resource file for app icon (if windres is available)
RC_FILE="$PROJECT_ROOT/scripts/videotools.rc"
SYMBOL_FILE="$PROJECT_ROOT/videotools_windows_amd64.syso"
if [ -f "$RC_FILE" ]; then
    if command -v x86_64-w64-mingw32-windres &> /dev/null; then
        x86_64-w64-mingw32-windres "$RC_FILE" -O coff -o "$SYMBOL_FILE" || true
    elif command -v windres &> /dev/null; then
        windres "$RC_FILE" -O coff -o "$SYMBOL_FILE" || true
    else
        echo "WARNING: windres not found; Windows icon will not be embedded in the EXE"
    fi
    if [ ! -f "$SYMBOL_FILE" ]; then
        echo "WARNING: windres did not produce $SYMBOL_FILE; icon may be missing"
    fi
fi

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
if [ -n "$GIT_COMMIT" ]; then
    LDFLAGS="$LDFLAGS -X main.buildCommit=$GIT_COMMIT"
fi

if go build -ldflags="$LDFLAGS" -o "$BUILD_OUTPUT" .; then
    echo "Cross-compilation successful!"
    echo ""
else
    echo "Build failed!"
    exit 1
fi

echo "Creating distribution package..."
mkdir -p "$dist_dir"

# Copy executable
cp "$BUILD_OUTPUT" "$dist_dir/"
echo "Copied VideoTools.exe"

# Copy documentation
cp README.md "$dist_dir/" 2>/dev/null || echo "WARNING: README.md not found"
cp LICENSE "$dist_dir/" 2>/dev/null || echo "WARNING: LICENSE not found"

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
    cp ffmpeg.exe "$dist_dir/"
    echo "Bundled ffmpeg.exe"
else
    echo "WARNING: ffmpeg.exe not found - distribution will require separate FFmpeg installation"
fi

if [ -f "ffprobe.exe" ]; then
    cp ffprobe.exe "$dist_dir/"
    echo "Bundled ffprobe.exe"
else
    echo "WARNING: ffprobe.exe not found"
fi

echo "Packaging build artifacts..."
pkg_dir="$(mktemp -d)"
cp "$dist_dir/VideoTools.exe" "$pkg_dir/"
if [ -f "$PROJECT_ROOT/README.md" ]; then
    cp "$PROJECT_ROOT/README.md" "$pkg_dir/"
fi
if [ -f "$PROJECT_ROOT/LICENSE" ]; then
    cp "$PROJECT_ROOT/LICENSE" "$pkg_dir/"
fi
if [ -f "$dist_dir/ffmpeg.exe" ]; then
    cp "$dist_dir/ffmpeg.exe" "$pkg_dir/"
fi
if [ -f "$dist_dir/ffprobe.exe" ]; then
    cp "$dist_dir/ffprobe.exe" "$pkg_dir/"
fi

if command -v python3 >/dev/null 2>&1; then
    python3 - <<PY
import os
import zipfile
pkg_dir = r"$pkg_dir"
artifact = r"$dist_dir/$artifact_name"
with zipfile.ZipFile(artifact, "w", zipfile.ZIP_DEFLATED) as zf:
    for root, _, files in os.walk(pkg_dir):
        for name in files:
            full = os.path.join(root, name)
            rel = os.path.relpath(full, pkg_dir)
            zf.write(full, rel)
PY
elif command -v zip >/dev/null 2>&1; then
    (cd "$pkg_dir" && zip -qr "$dist_dir/$artifact_name" .)
else
    echo "WARNING: zip/python3 not found; skipping artifact packaging"
fi

published_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
cat > "$dist_dir/build.json" <<EOF
{
  "channel": "$channel",
  "version": "$version",
  "git": "$GIT_COMMIT",
  "published_at": "$published_at",
  "artifact": "$artifact_name"
}
EOF

rm -rf "$pkg_dir"
echo "Build package: $dist_dir/$artifact_name"
echo "Build metadata: $dist_dir/build.json"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "WINDOWS BUILD COMPLETE"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Output directory: $dist_dir"
echo "Contents:"
ls -lh "$dist_dir"
echo ""
echo "Windows executable: $dist_dir/VideoTools.exe"
echo "Size: $(du -h "$dist_dir/VideoTools.exe" | cut -f1)"
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
