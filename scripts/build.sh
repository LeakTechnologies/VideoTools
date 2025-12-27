#!/bin/bash
# VideoTools Universal Build Script (Linux/macOS/Windows via Git Bash)
set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Extract app version from main.go (avoid grep warnings on Git Bash)
APP_VERSION="$(grep -m1 'appVersion' "$PROJECT_ROOT/main.go" | sed -E 's/.*\"([^\"]+)\".*/\1/')"
[ -z "$APP_VERSION" ] && APP_VERSION="(version unknown)"

# Detect platform
PLATFORM="$(uname -s)"
case "$PLATFORM" in
    Linux*) OS="Linux" ;;
    Darwin*) OS="macOS" ;;
    CYGWIN*|MINGW*|MSYS*) OS="Windows" ;;
    *) echo "Unknown platform: $PLATFORM"; exit 1 ;;
esac

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools ${OS} Build"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Detected platform: $OS"
echo ""

# Go check
if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: Go is not installed. Please install Go 1.21+ (go version currently missing)."
    exit 1
fi

echo "Go version:"
go version
echo ""

diagnostics() {
    echo "Diagnostics: version=$APP_VERSION os=$OS arch=$(uname -m) go=$(go version | awk '{print $3}')"
}

case "$OS" in
    Linux|macOS)
        echo "→ Building VideoTools $APP_VERSION for $OS..."
        echo ""
        exec "$SCRIPT_DIR/build-linux.sh"
        ;;
    Windows)
        echo "→ Building VideoTools $APP_VERSION for Windows..."
        echo ""
        cd "$PROJECT_ROOT"

        echo "Cleaning previous builds..."
        rm -f VideoTools.exe 2>/dev/null || true
        # Clear Go cache to avoid permission issues
        go clean -cache -modcache -testcache 2>/dev/null || true
        echo "Cache cleaned"
        echo ""

        echo "Downloading dependencies..."
        go mod download
        echo "Dependencies downloaded"
        echo ""

        echo "Building VideoTools $APP_VERSION for Windows..."
        export CGO_ENABLED=1
        if go build -ldflags="-H windowsgui -s -w" -o VideoTools.exe .; then
            echo "Build successful! (VideoTools $APP_VERSION)"
            echo ""
            if [ -f "setup-windows.bat" ]; then
                echo "════════════════════════════════════════════════════════════════"
                echo "BUILD COMPLETE - $APP_VERSION"
                echo "════════════════════════════════════════════════════════════════"
                echo ""
                echo "Output: VideoTools.exe"
                if [ -f "VideoTools.exe" ]; then
                    SIZE=$(du -h VideoTools.exe 2>/dev/null | cut -f1 || echo "unknown")
                    echo "Size: $SIZE"
                fi
                diagnostics
                echo ""
                echo "Next step: Get FFmpeg"
                echo "  Run: setup-windows.bat"
                echo "  Or:  .\\scripts\\setup-windows.ps1 -Portable"
                echo ""
                if ffmpeg -version >/dev/null 2>&1 && ffprobe -version >/dev/null 2>&1; then
                    echo "FFmpeg detected on PATH. Skipping bundled download."
                else
                    echo "FFmpeg not detected on PATH."
                    echo "Next step: Get FFmpeg"
                    echo "  Run: setup-windows.bat"
                    echo "  Or:  .\\scripts\\setup-windows.ps1 -Portable"
                    echo "You can skip if FFmpeg is already installed elsewhere."
                fi
            else
                echo "Build complete: VideoTools.exe"
                diagnostics
            fi
        else
            echo "Build failed! (VideoTools $APP_VERSION)"
            diagnostics
            echo ""
            echo "Help: check the Go error messages above."
            echo " - Undefined symbol/identifier: usually a missing variable or typo in source; see the referenced file:line."
            echo " - \"C compiler not found\": install MinGW-w64 or MSYS2 toolchain so gcc is in PATH."
            echo " - Cache permission denied: run scripts/clear-go-cache.sh or delete/chown the Go build cache (e.g., %LOCALAPPDATA%\\go-build on Windows)."
            exit 1
        fi
        ;;
esac
