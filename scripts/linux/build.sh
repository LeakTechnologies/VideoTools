#!/bin/bash
# VideoTools Build Script (Linux/macOS)
# Delegates to platform-specific build scripts
set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect platform
PLATFORM="$(uname -s)"
case "$PLATFORM" in
    Linux*) OS="Linux" ;;
    Darwin*) OS="macOS" ;;
    *) echo "ERROR: Unknown platform: $PLATFORM"; exit 1 ;;
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

echo "→ Building VideoTools for $OS..."
echo ""

# Delegate to Linux build script (works for macOS too)
exec "$SCRIPT_DIR/build-linux.sh"
