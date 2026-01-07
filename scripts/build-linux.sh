#!/bin/bash
# VideoTools Build Script
# Cleans dependencies and builds the application with proper error handling

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools"
# Extract app version from main.go (avoid grep warnings on Git Bash)
APP_VERSION="$(grep -m1 'appVersion' "$PROJECT_ROOT/main.go" | sed -E 's/.*\"([^\"]+)\".*/\1/')"
[ -z "$APP_VERSION" ] && APP_VERSION="(version unknown)"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Build Script"
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

# Change to project directory
cd "$PROJECT_ROOT"

echo "Cleaning previous builds and cache..."
go clean -cache -testcache 2>/dev/null || true
rm -f "$BUILD_OUTPUT" 2>/dev/null || true
# Also clear build cache directory to avoid permission issues
rm -rf "${GOCACHE:-$HOME/.cache/go-build}" 2>/dev/null || true
echo "Cache cleaned"
echo ""

echo "Downloading and verifying dependencies (skips if already cached)..."
if go list -m all >/dev/null 2>&1; then
    echo "Dependencies already present"
else
    if go mod download && go mod verify; then
        echo "Dependencies downloaded and verified"
    else
        echo "Failed to download/verify modules. Check network/GOPROXY or try again."
        exit 1
    fi
fi
echo ""

echo "Building VideoTools..."
# Build timer
build_start=$(date +%s)
# Fyne needs cgo for GLFW/OpenGL bindings; build with CGO enabled.
export CGO_ENABLED=1
export GOCACHE="$PROJECT_ROOT/.cache/go-build"
export GOMODCACHE="$PROJECT_ROOT/.cache/go-mod"
mkdir -p "$GOCACHE" "$GOMODCACHE"
if [ -d "$PROJECT_ROOT/vendor" ] && [ ! -f "$PROJECT_ROOT/vendor/modules.txt" ]; then
    export GOFLAGS="${GOFLAGS:-} -mod=mod"
fi
GST_TAG=""
if [ -n "$VT_GSTREAMER" ]; then
    GST_TAG="gstreamer"
elif command -v pkg-config &> /dev/null; then
    if pkg-config --exists gstreamer-1.0 gstreamer-app-1.0 gstreamer-video-1.0; then
        GST_TAG="gstreamer"
    fi
fi
if [ -n "$GST_TAG" ]; then
    export GOFLAGS="${GOFLAGS:-} -tags ${GST_TAG}"
fi
if go build -o "$BUILD_OUTPUT" .; then
    build_end=$(date +%s)
    build_secs=$((build_end - build_start))
    echo "Build successful! (VideoTools $APP_VERSION)"
    echo "Build time: ${build_secs}s"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "BUILD COMPLETE - $APP_VERSION"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "Output: $BUILD_OUTPUT"
    echo "Size: $(du -h "$BUILD_OUTPUT" | cut -f1)"
    echo "Diagnostics: version=$APP_VERSION os=$(uname -s) arch=$(uname -m) go=$(go version | awk '{print $3}')"
    echo ""
    echo "To run:"
    echo "  $PROJECT_ROOT/VideoTools"
    echo ""
    echo "Or use the convenience script:"
    echo "  source $PROJECT_ROOT/scripts/alias.sh"
    echo "  VideoTools"
    echo ""
else
    echo "Build failed! (VideoTools $APP_VERSION)"
    echo "Diagnostics: version=$APP_VERSION os=$(uname -s) arch=$(uname -m) go=$(go version | awk '{print $3}')"
    echo ""
    echo "Help: check the Go error messages above."
    echo " - Undefined symbol/identifier: usually a missing variable or typo in source; see the referenced file:line."
    echo " - \"C compiler not found\": install a C toolchain (e.g., build-essential on Ubuntu, Xcode CLT on macOS)."
    echo " - Cache permission denied: run scripts/clear-go-cache.sh or rm -rf ~/.cache/go-build / chown -R \$USER ~/.cache/go-build."
    exit 1
fi
