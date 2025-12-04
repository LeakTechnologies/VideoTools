#!/bin/bash
# VideoTools Universal Build Script
# Auto-detects platform and builds accordingly

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Universal Build Script"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Detect platform
PLATFORM="$(uname -s)"
case "${PLATFORM}" in
    Linux*)
        OS="Linux"
        ;;
    Darwin*)
        OS="macOS"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        OS="Windows"
        ;;
    *)
        echo "❌ Unknown platform: ${PLATFORM}"
        exit 1
        ;;
esac

echo "🔍 Detected platform: $OS"
echo ""

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "❌ ERROR: Go is not installed. Please install Go 1.21 or later."
    echo ""
    echo "Download from: https://go.dev/dl/"
    exit 1
fi

echo "📦 Go version:"
go version
echo ""

# Route to appropriate build script
case "$OS" in
    Linux)
        echo "→ Building for Linux..."
        echo ""
        exec "$SCRIPT_DIR/build-linux.sh"
        ;;

    macOS)
        echo "→ Building for macOS..."
        echo ""
        # macOS uses same build process as Linux (native build)
        exec "$SCRIPT_DIR/build-linux.sh"
        ;;

    Windows)
        echo "→ Building for Windows..."
        echo ""

        # Check if running in Git Bash or similar
        if command -v go.exe &> /dev/null; then
            # Windows native build
            cd "$PROJECT_ROOT"

            echo "🧹 Cleaning previous builds..."
            rm -f VideoTools.exe 2>/dev/null || true
            echo "✓ Cache cleaned"
            echo ""

            echo "⬇️  Downloading dependencies..."
            go mod download
            echo "✓ Dependencies downloaded"
            echo ""

            echo "🔨 Building VideoTools for Windows..."
            export CGO_ENABLED=1

            # Build with Windows GUI flags
            if go build -ldflags="-H windowsgui -s -w" -o VideoTools.exe .; then
                echo "✓ Build successful!"
                echo ""

                # Run setup script to get FFmpeg
                if [ -f "setup-windows.bat" ]; then
                    echo "════════════════════════════════════════════════════════════════"
                    echo "✅ BUILD COMPLETE"
                    echo "════════════════════════════════════════════════════════════════"
                    echo ""
                    echo "Output: VideoTools.exe"
                    if [ -f "VideoTools.exe" ]; then
                        SIZE=$(du -h VideoTools.exe 2>/dev/null | cut -f1 || echo "unknown")
                        echo "Size: $SIZE"
                    fi
                    echo ""
                    echo "Next step: Get FFmpeg"
                    echo "  Run: setup-windows.bat"
                    echo "  Or:  .\scripts\setup-windows.ps1 -Portable"
                    echo ""

                    # Offer to run setup automatically
                    echo "Would you like to download FFmpeg now? (y/n)"
                    read -r response
                    if [[ "$response" =~ ^[Yy]$ ]]; then
                        if command -v powershell &> /dev/null; then
                            powershell -ExecutionPolicy Bypass -File "$SCRIPT_DIR/setup-windows.ps1" -Portable
                        else
                            cmd.exe /c setup-windows.bat
                        fi
                    else
                        echo "You can run setup-windows.bat later to get FFmpeg."
                    fi
                else
                    echo "✓ Build complete: VideoTools.exe"
                fi
            else
                echo "❌ Build failed!"
                exit 1
            fi
        else
            echo "❌ ERROR: go.exe not found."
            echo "Please ensure Go is properly installed on Windows."
            echo "Download from: https://go.dev/dl/"
            exit 1
        fi
        ;;
esac
