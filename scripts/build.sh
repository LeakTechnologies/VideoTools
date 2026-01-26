#!/bin/bash
# VideoTools Universal Build Script (Linux/macOS/Windows via Git Bash)
set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
    echo "Diagnostics: version=$FULL_VERSION os=$OS arch=$(uname -m) go=$(go version | awk '{print $3}')"
}

case "$OS" in
    Linux|macOS)
        echo "→ Building VideoTools $FULL_VERSION for $OS..."
        echo ""
        exec "$SCRIPT_DIR/build-linux.sh"
        ;;
    Windows)
        echo "→ Building VideoTools $FULL_VERSION for Windows..."
        echo ""
        cd "$PROJECT_ROOT"
        build_start=$(date +%s)

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

        echo "Checking for GStreamer (required for player)..."
        if ! pkg-config --exists gstreamer-1.0 gstreamer-app-1.0 gstreamer-video-1.0 2>/dev/null; then
            echo "WARNING: GStreamer development libraries not found."
            echo "Player functionality will be limited. Install GStreamer for full functionality."
        else
            echo "GStreamer found ($(pkg-config --modversion gstreamer-1.0 2>/dev/null || echo 'version unknown'))"
        fi
        echo ""
        echo "Building VideoTools $FULL_VERSION for Windows..."
        export CGO_ENABLED=1
        # Set module flag if needed
        if [ -d "$PROJECT_ROOT/vendor" ] && [ ! -f "$PROJECT_ROOT/vendor/modules.txt" ]; then
            export GOFLAGS="${GOFLAGS:-} -mod=mod"
        fi
        LDFLAGS="-H windowsgui -s -w"
        if [ -n "$GIT_COMMIT" ]; then
            LDFLAGS="$LDFLAGS -X main.buildCommit=$GIT_COMMIT"
        fi
        if go build -tags gstreamer -ldflags="$LDFLAGS" -o VideoTools.exe .; then
            build_end=$(date +%s)
            build_secs=$((build_end - build_start))
            echo "Build successful! (VideoTools $FULL_VERSION)"
            echo "Build time: ${build_secs}s"
            echo ""
            if [ -f "scripts/_internal/setup-windows.bat" ]; then
                echo "════════════════════════════════════════════════════════════════"
                echo "BUILD COMPLETE - $FULL_VERSION"
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
                echo "  Run: scripts/_internal/setup-windows.bat"
                echo "  Or:  .\\scripts\\_internal\\setup-windows.ps1 -Portable"
                echo ""
                if ffmpeg -version >/dev/null 2>&1 && ffprobe -version >/dev/null 2>&1; then
                    echo "FFmpeg detected on PATH. Skipping bundled download."
                else
                    echo "FFmpeg not detected on PATH."
                    echo "Next step: Get FFmpeg"
                    echo "  Run: scripts/_internal/setup-windows.bat"
                    echo "  Or:  .\\scripts\\_internal\\setup-windows.ps1 -Portable"
                    echo "You can skip if FFmpeg is already installed elsewhere."
                fi
            else
                echo "Build complete: VideoTools.exe"
                diagnostics
            fi

            dist_dir="$PROJECT_ROOT/dist/windows/$channel"
            artifact_name="${version}-${GIT_COMMIT}_win.zip"
            mkdir -p "$dist_dir"
            pkg_dir="$(mktemp -d)"
            cp "$PROJECT_ROOT/VideoTools.exe" "$pkg_dir/"
            if [ -f "$PROJECT_ROOT/README.md" ]; then
                cp "$PROJECT_ROOT/README.md" "$pkg_dir/"
            fi
            if [ -f "$PROJECT_ROOT/LICENSE" ]; then
                cp "$PROJECT_ROOT/LICENSE" "$pkg_dir/"
            fi
            if [ -f "$PROJECT_ROOT/ffmpeg.exe" ]; then
                cp "$PROJECT_ROOT/ffmpeg.exe" "$pkg_dir/"
            fi
            if [ -f "$PROJECT_ROOT/ffprobe.exe" ]; then
                cp "$PROJECT_ROOT/ffprobe.exe" "$pkg_dir/"
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
