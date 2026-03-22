#!/bin/bash
# VideoTools Build Script
# Cleans dependencies and builds the application with proper error handling

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools"
# Extract app version from main.go (avoid grep warnings on Git Bash)
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

os_tag="linux"
if [ "$(uname -s)" = "Darwin" ]; then
    os_tag="macos"
fi

dist_dir="$PROJECT_ROOT/dist/$os_tag/$channel"
artifact_name="${version}-${GIT_COMMIT}_${os_tag}.zip"

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

echo "Checking for GStreamer (required for player)..."
# GStreamer is now mandatory - verify it's installed
if ! command -v pkg-config &> /dev/null; then
    echo "ERROR: pkg-config not found. Install pkg-config to build VideoTools."
    exit 1
fi
if ! pkg-config --exists gstreamer-1.0 gstreamer-app-1.0 gstreamer-video-1.0; then
    echo "ERROR: GStreamer development libraries not found."
    echo "Please run: ./scripts/linux/install.sh"
    echo "Or install manually:"
    echo "  Ubuntu/Debian: sudo apt-get install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev gstreamer1.0-plugins-base"
    echo "  Fedora:       sudo dnf install gstreamer1-devel gstreamer1-plugins-base-devel"
    echo "  Arch:        sudo pacman -S gstreamer gst-plugins-base"
    exit 1
fi
echo "GStreamer found ($(pkg-config --modversion gstreamer-1.0))"
echo ""

# Check if GStreamer should be used (optional)
GSTREAMER_TAG=""
if pkg-config --exists gstreamer-1.0 gstreamer-app-1.0 gstreamer-video-1.0 2>/dev/null; then
    echo "Building VideoTools with GStreamer player..."
    GSTREAMER_TAG="-tags gstreamer"
else
    echo "Building VideoTools with fallback players (MPV/FFplay/VLC)..."
    echo "Note: Install GStreamer for native player: sudo apt-get install libgstreamer1.0-dev"
fi
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
# GStreamer is always enabled now (mandatory dependency)
LDFLAGS=""
if [ -n "$GIT_COMMIT" ]; then
    LDFLAGS="-X main.buildCommit=$GIT_COMMIT"
fi
if go build -tags native_media $GSTREAMER_TAG -ldflags="$LDFLAGS" -o "$BUILD_OUTPUT" .; then
    build_end=$(date +%s)
    build_secs=$((build_end - build_start))
    echo "Build successful! (VideoTools $FULL_VERSION)"
    echo "Build time: ${build_secs}s"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "BUILD COMPLETE - $FULL_VERSION"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "Output: $BUILD_OUTPUT"
    echo "Size: $(du -h "$BUILD_OUTPUT" | cut -f1)"
    echo "Diagnostics: version=$FULL_VERSION os=$(uname -s) arch=$(uname -m) go=$(go version | awk '{print $3}')"
    echo ""
    echo "To run:"
    echo "  $PROJECT_ROOT/VideoTools"
    echo ""
    echo "Or use the convenience script:"
    echo "  source $PROJECT_ROOT/scripts/alias.sh"
    echo "  VideoTools"
    echo ""

    echo "Packaging build artifacts..."
    mkdir -p "$dist_dir"
    pkg_dir="$(mktemp -d)"
    cp "$BUILD_OUTPUT" "$pkg_dir/VideoTools"
    if [ -f "$PROJECT_ROOT/README.md" ]; then
        cp "$PROJECT_ROOT/README.md" "$pkg_dir/"
    fi
    if [ -f "$PROJECT_ROOT/LICENSE" ]; then
        cp "$PROJECT_ROOT/LICENSE" "$pkg_dir/"
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
else
    echo "Build failed! (VideoTools $FULL_VERSION)"
    echo "Diagnostics: version=$FULL_VERSION os=$(uname -s) arch=$(uname -m) go=$(go version | awk '{print $3}')"
    echo ""
    echo "Help: check the Go error messages above."
    echo " - Undefined symbol/identifier: usually a missing variable or typo in source; see the referenced file:line."
    echo " - \"C compiler not found\": install a C toolchain (e.g., build-essential on Ubuntu, Xcode CLT on macOS)."
    echo " - Cache permission denied: run scripts/clear-go-cache.sh or rm -rf ~/.cache/go-build / chown -R \$USER ~/.cache/go-build."
    exit 1
fi
