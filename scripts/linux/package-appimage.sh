#!/bin/bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

channel="${VT_BUILD_CHANNEL:-dev}"
case "${channel,,}" in
    stable|public|release) channel="stable" ;;
    *) channel="dev" ;;
esac

version_base="${VT_VERSION_BASE:-}"
if [ -z "$version_base" ]; then
    version_base="$(grep -m1 'appVersion' "$PROJECT_ROOT/main.go" | sed -E 's/.*"([^"]+)".*/\1/')"
fi
if [ -z "$version_base" ]; then
    version_base="v0.1.1-dev"
fi

appdir="$PROJECT_ROOT/.cache/appimage/AppDir"
rm -rf "$appdir"
mkdir -p "$appdir"

bin_src="$PROJECT_ROOT/VideoTools"
if [ ! -f "$bin_src" ]; then
    echo "ERROR: VideoTools binary not found at $bin_src"
    exit 1
fi

export PATH="$PROJECT_ROOT/.cache/appimage:$PATH"
export APPIMAGE_EXTRACT_AND_RUN=1
if ! command -v linuxdeploy-x86_64.AppImage >/dev/null 2>&1; then
    echo "ERROR: linuxdeploy-x86_64.AppImage not found in PATH"
    exit 1
fi
if ! command -v appimagetool-x86_64.AppImage >/dev/null 2>&1; then
    echo "ERROR: appimagetool-x86_64.AppImage not found in PATH"
    exit 1
fi

cp "$bin_src" "$PROJECT_ROOT/.cache/appimage/VideoTools"

LINUXDEPLOY="$PROJECT_ROOT/.cache/appimage/linuxdeploy-x86_64.AppImage"
APPIMAGETOOL="$PROJECT_ROOT/.cache/appimage/appimagetool-x86_64.AppImage"

"$LINUXDEPLOY" \
    --appdir "$appdir" \
    --executable "$PROJECT_ROOT/.cache/appimage/VideoTools" \
    --desktop-file "$PROJECT_ROOT/packaging/linux/VideoTools.desktop" \
    --icon-file "$PROJECT_ROOT/assets/logo/VT_Logo.png" \
    --output appimage

appimage_out="$PROJECT_ROOT/VideoTools-x86_64.AppImage"
if [ ! -f "$appimage_out" ]; then
    echo "ERROR: AppImage build failed"
    exit 1
fi

mkdir -p "$PROJECT_ROOT/dist/linux/$channel"
final_out="$PROJECT_ROOT/dist/linux/$channel/${version_base}_linux.AppImage"
rm -f "$final_out"
mv "$appimage_out" "$final_out"

chmod +x "$final_out"

echo "AppImage created: $final_out"
