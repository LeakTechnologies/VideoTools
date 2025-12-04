#!/bin/bash
# VideoTools Build Script
# Cleans dependencies and builds the application with proper error handling

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Build Script"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "❌ ERROR: Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

echo "📦 Go version:"
go version
echo ""

# Change to project directory
cd "$PROJECT_ROOT"

echo "🧹 Cleaning previous builds and cache..."
go clean -cache -modcache -testcache 2>/dev/null || true
rm -f "$BUILD_OUTPUT" 2>/dev/null || true
echo "✓ Cache cleaned"
echo ""

echo "⬇️  Downloading and verifying dependencies..."
go mod download
go mod verify
echo "✓ Dependencies verified"
echo ""

echo "🔨 Building VideoTools..."
# Fyne needs cgo for GLFW/OpenGL bindings; build with CGO enabled.
export CGO_ENABLED=1
if go build -o "$BUILD_OUTPUT" .; then
    echo "✓ Build successful!"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "✅ BUILD COMPLETE"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "Output: $BUILD_OUTPUT"
    echo "Size: $(du -h "$BUILD_OUTPUT" | cut -f1)"
    echo ""
    echo "To run:"
    echo "  $PROJECT_ROOT/VideoTools"
    echo ""
    echo "Or use the convenience script:"
    echo "  source $PROJECT_ROOT/scripts/alias.sh"
    echo "  VideoTools"
    echo ""
else
    echo "❌ Build failed!"
    exit 1
fi
