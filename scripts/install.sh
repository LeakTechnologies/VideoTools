#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Spinner function
spinner() {
    local pid=$1
    local task=$2
    local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0

    while kill -0 $pid 2>/dev/null; do
        i=$(( (i+1) %10 ))
        printf "\r${BLUE}${spin:$i:1}${NC} %s..." "$task"
        sleep 0.1
    done
    printf "\r"
}

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Args
DVDSTYLER_URL=""
DVDSTYLER_ZIP=""
SKIP_DVD_TOOLS=""
SKIP_AI_TOOLS=""
SKIP_WHISPER=""
while [ $# -gt 0 ]; do
	case "$1" in
		--dvdstyler-url=*)
			DVDSTYLER_URL="${1#*=}"
			shift
			;;
		--dvdstyler-url)
			DVDSTYLER_URL="$2"
			shift 2
			;;
		--dvdstyler-zip=*)
			DVDSTYLER_ZIP="${1#*=}"
			shift
			;;
		--dvdstyler-zip)
			DVDSTYLER_ZIP="$2"
			shift 2
			;;
		--skip-dvd)
			SKIP_DVD_TOOLS=true
			shift
			;;
		--skip-ai)
			SKIP_AI_TOOLS=true
			shift
			;;
		--skip-whisper)
			SKIP_WHISPER=true
			shift
			;;
		*)
			echo "Unknown option: $1"
			echo "Usage: $0 [--dvdstyler-url URL] [--dvdstyler-zip PATH] [--skip-dvd] [--skip-ai] [--skip-whisper]"
			exit 1
			;;
	esac
done

# Platform detection
UNAME_S="$(uname -s)"
IS_WINDOWS=false
IS_DARWIN=false
IS_LINUX=false
case "$UNAME_S" in
	MINGW*|MSYS*|CYGWIN*)
		IS_WINDOWS=true
		;;
	Darwin*)
		IS_DARWIN=true
		;;
	Linux*)
		IS_LINUX=true
		;;
esac

INSTALL_TITLE="VideoTools Installation"
if [ "$IS_WINDOWS" = true ]; then
	INSTALL_TITLE="VideoTools Windows Installation"
elif [ "$IS_DARWIN" = true ]; then
	INSTALL_TITLE="VideoTools macOS Installation"
elif [ "$IS_LINUX" = true ]; then
	INSTALL_TITLE="VideoTools Linux Installation"
fi

echo "════════════════════════════════════════════════════════════════"
echo "  $INSTALL_TITLE"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Step 1: Check if Go is installed
echo -e "${CYAN}[1/2]${NC} Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo -e "${RED}[ERROR] Error: Go is not installed or not in PATH${NC}"
    echo "Please install Go 1.21+ from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}[OK]${NC} Found Go version: $GO_VERSION"

# Step 2: Check authoring dependencies
echo ""
echo -e "${CYAN}[2/2]${NC} Checking authoring dependencies..."

if [ "$IS_WINDOWS" = true ]; then
    echo "Detected Windows environment."
    if [ -z "$SKIP_DVD_TOOLS" ]; then
        # Check if DVDStyler is already installed (Windows)
        if command -v dvdstyler &> /dev/null || [ -f "/c/Program Files/DVDStyler/DVDStyler.exe" ] || [ -f "C:\\Program Files\\DVDStyler\\DVDStyler.exe" ]; then
            echo -e "${GREEN}[OK]${NC} DVDStyler already installed"
            SKIP_DVD_TOOLS=true
        else
            echo ""
            read -p "Install DVD authoring tools (DVDStyler)? [y/N]: " dvd_choice
            if [[ "$dvd_choice" =~ ^[Yy]$ ]]; then
                SKIP_DVD_TOOLS=false
            else
                SKIP_DVD_TOOLS=true
            fi
        fi
    fi
    if command -v powershell.exe &> /dev/null; then
        PS_ARGS=()
        if [ "$SKIP_DVD_TOOLS" = true ]; then
            PS_ARGS+=("-SkipDvdStyler")
        fi
        if [ -n "$DVDSTYLER_ZIP" ]; then
            powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PROJECT_ROOT/scripts/install-deps-windows.ps1" -DvdStylerZip "$DVDSTYLER_ZIP" "${PS_ARGS[@]}"
        elif [ -n "$DVDSTYLER_URL" ]; then
            powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PROJECT_ROOT/scripts/install-deps-windows.ps1" -DvdStylerUrl "$DVDSTYLER_URL" "${PS_ARGS[@]}"
        else
            powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PROJECT_ROOT/scripts/install-deps-windows.ps1" "${PS_ARGS[@]}"
        fi
        if [ $? -ne 0 ]; then
            echo -e "${RED}[ERROR] Windows dependency installer failed.${NC}"
            echo "If DVDStyler download failed, retry with a direct mirror:"
            echo ""
            echo "Git Bash:"
            echo "  export VT_DVDSTYLER_URL=\"https://netcologne.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip\""
            echo "  ./scripts/install.sh"
            echo ""
            echo "PowerShell:"
            echo "  \$env:VT_DVDSTYLER_URL=\"https://netcologne.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip\""
            echo "  .\\scripts\\install-deps-windows.ps1"
            exit 1
        fi
        echo -e "${GREEN}[OK]${NC} Windows dependency installer completed"
    else
        echo -e "${RED}[ERROR] powershell.exe not found.${NC}"
        echo "Please run: $PROJECT_ROOT\\scripts\\install-deps-windows.ps1"
        exit 1
    fi
else
    missing_deps=()
    # Core dependencies (always required)
    if ! command -v ffmpeg &> /dev/null; then
        missing_deps+=("ffmpeg")
    fi
    # GStreamer is now mandatory for player functionality (replacing FFmpeg pipe-based player)
    if ! command -v gst-launch-1.0 &> /dev/null; then
        missing_deps+=("gstreamer")
    fi
    # Check for GStreamer development headers (required for Go CGO bindings)
    if ! pkg-config --exists gstreamer-1.0 2>/dev/null; then
        missing_deps+=("gstreamer-devel")
    fi
    if [ -z "$SKIP_DVD_TOOLS" ]; then
        # Check if DVD tools are already installed
        if command -v dvdauthor &> /dev/null && command -v xorriso &> /dev/null; then
            echo -e "${GREEN}[OK]${NC} DVD authoring tools already installed"
            SKIP_DVD_TOOLS=true
        else
            echo ""
            read -p "Install DVD authoring tools (dvdauthor + ISO tools)? [y/N]: " dvd_choice
            if [[ "$dvd_choice" =~ ^[Yy]$ ]]; then
                SKIP_DVD_TOOLS=false
            else
                SKIP_DVD_TOOLS=true
            fi
        fi
    fi
    if [ "$SKIP_DVD_TOOLS" = false ]; then
        if ! command -v dvdauthor &> /dev/null; then
            missing_deps+=("dvdauthor")
        fi
        if ! command -v xorriso &> /dev/null; then
            missing_deps+=("xorriso")
        fi
    fi

    # Ask about AI upscaling tools
    if [ -z "$SKIP_AI_TOOLS" ]; then
        # Check if Real-ESRGAN is already installed
        if command -v realesrgan-ncnn-vulkan &> /dev/null; then
            echo -e "${GREEN}[OK]${NC} Real-ESRGAN NCNN already installed"
            SKIP_AI_TOOLS=true
        else
            echo ""
            read -p "Install AI upscaling tools (Real-ESRGAN NCNN)? [y/N]: " ai_choice
            if [[ "$ai_choice" =~ ^[Yy]$ ]]; then
                SKIP_AI_TOOLS=false
            else
                SKIP_AI_TOOLS=true
            fi
        fi
    fi
    if [ "$SKIP_AI_TOOLS" = false ]; then
        if ! command -v realesrgan-ncnn-vulkan &> /dev/null; then
            missing_deps+=("realesrgan-ncnn-vulkan")
        fi
    fi

    # Whisper backend check (offline-only, no prompts)
    if command -v whisper &> /dev/null || command -v whisper.cpp &> /dev/null; then
        echo -e "${GREEN}[OK]${NC} Whisper backend found"
    else
        echo -e "${YELLOW}WARNING:${NC} Whisper backend not found; offline speech-to-text will be unavailable"
        echo "Install whisper.cpp manually and ensure its binary is on your PATH."
    fi

    install_deps=false
    if [ ${#missing_deps[@]} -gt 0 ]; then
        echo -e "${YELLOW}WARNING:${NC} Missing dependencies: ${missing_deps[*]}"
        echo "Installing missing dependencies..."
        install_deps=true
    else
        echo -e "${GREEN}[OK]${NC} All authoring dependencies found"
    fi

    if [ "$install_deps" = true ]; then
        if command -v apt-get &> /dev/null; then
            echo "Installing core dependencies (FFmpeg + GStreamer)..."
            sudo apt-get update
            # Core packages (always installed) - GStreamer is mandatory for player
            CORE_PKGS="ffmpeg gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev"
            if [ "$SKIP_DVD_TOOLS" = true ]; then
                sudo apt-get install -y $CORE_PKGS
            else
                sudo apt-get install -y $CORE_PKGS dvdauthor xorriso
            fi
        elif command -v dnf &> /dev/null; then
            echo "Installing core dependencies (FFmpeg + GStreamer)..."
            # Core packages (always installed) - GStreamer is mandatory for player
            CORE_PKGS="ffmpeg gstreamer1 gstreamer1-plugins-base gstreamer1-plugins-good gstreamer1-plugins-bad-free gstreamer1-plugins-ugly-free gstreamer1-libav gstreamer1-devel gstreamer1-plugins-base-devel"
            if [ "$SKIP_DVD_TOOLS" = true ]; then
                sudo dnf install -y $CORE_PKGS
            else
                sudo dnf install -y $CORE_PKGS dvdauthor xorriso
            fi
        elif command -v pacman &> /dev/null; then
            echo "Installing core dependencies (FFmpeg + GStreamer)..."
            # Core packages (always installed)
            CORE_PKGS="ffmpeg gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly gst-libav"
            if [ "$SKIP_DVD_TOOLS" = true ]; then
                sudo pacman -Sy --noconfirm $CORE_PKGS
            else
                sudo pacman -Sy --noconfirm $CORE_PKGS dvdauthor cdrtools
            fi
        elif command -v zypper &> /dev/null; then
            echo "Installing core dependencies (FFmpeg + GStreamer)..."
            # Core packages (always installed)
            CORE_PKGS="ffmpeg gstreamer gstreamer-plugins-base gstreamer-plugins-good gstreamer-plugins-bad gstreamer-plugins-ugly gstreamer-plugins-libav gstreamer-devel"
            if [ "$SKIP_DVD_TOOLS" = true ]; then
                sudo zypper install -y $CORE_PKGS
            else
                sudo zypper install -y $CORE_PKGS dvdauthor xorriso
            fi
        elif command -v brew &> /dev/null; then
            echo "Installing core dependencies (FFmpeg + GStreamer)..."
            # Core packages (always installed)
            CORE_PKGS="ffmpeg gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-plugins-ugly gst-libav"
            if [ "$SKIP_DVD_TOOLS" = true ]; then
                brew install $CORE_PKGS
            else
                brew install $CORE_PKGS dvdauthor xorriso
            fi
        else
            echo -e "${RED}[ERROR] No supported package manager found.${NC}"
            echo "Please install: ffmpeg, dvdauthor, and mkisofs/genisoimage/xorriso"
            exit 1
        fi

        # Install Real-ESRGAN NCNN if requested and not available
        if [ "$SKIP_AI_TOOLS" = false ] && ! command -v realesrgan-ncnn-vulkan &> /dev/null; then
            echo ""
            echo "Installing Real-ESRGAN NCNN..."

            # Detect architecture
            ARCH=$(uname -m)
            if [ "$ARCH" = "x86_64" ]; then
                ESRGAN_ARCH="ubuntu"
            elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
                echo -e "${YELLOW}WARNING:${NC} ARM architecture detected. You may need to build realesrgan-ncnn-vulkan from source."
                echo "See: https://github.com/xinntao/Real-ESRGAN-ncnn-vulkan"
                ESRGAN_ARCH=""
            else
                echo -e "${YELLOW}WARNING:${NC} Unsupported architecture: $ARCH"
                ESRGAN_ARCH=""
            fi

            if [ -n "$ESRGAN_ARCH" ]; then
                ESRGAN_URL="https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-ubuntu.zip"
                TEMP_DIR=$(mktemp -d)

                if command -v wget &> /dev/null; then
                    wget -q "$ESRGAN_URL" -O "$TEMP_DIR/realesrgan.zip"
                elif command -v curl &> /dev/null; then
                    curl -sL "$ESRGAN_URL" -o "$TEMP_DIR/realesrgan.zip"
                else
                    echo -e "${YELLOW}WARNING:${NC} Neither wget nor curl found. Cannot download Real-ESRGAN."
                    echo "Please install manually from: https://github.com/xinntao/Real-ESRGAN/releases"
                fi

                if [ -f "$TEMP_DIR/realesrgan.zip" ]; then
                    unzip -q "$TEMP_DIR/realesrgan.zip" -d "$TEMP_DIR"
                    sudo install -m 755 "$TEMP_DIR/realesrgan-ncnn-vulkan" /usr/local/bin/ 2>/dev/null || \
                        install -m 755 "$TEMP_DIR/realesrgan-ncnn-vulkan" "$HOME/.local/bin/" 2>/dev/null || \
                        echo -e "${YELLOW}WARNING:${NC} Could not install to /usr/local/bin or ~/.local/bin"
                    rm -rf "$TEMP_DIR"

                    if command -v realesrgan-ncnn-vulkan &> /dev/null; then
                        echo -e "${GREEN}[OK]${NC} Real-ESRGAN NCNN installed successfully"
                    fi
                fi
            fi
        fi

        # Whisper backend is offline-only; no auto-install here.
    fi

    # Seed whisper.cpp model (small model) - prefer bundled, otherwise download.
    whisper_model_src="$(cd "$(dirname "$0")/.." && pwd)/vendor/whisper/ggml-small.bin"
    whisper_model_dir="$HOME/.local/share/whisper.cpp/models"
    whisper_model_dest="$whisper_model_dir/ggml-small.bin"
    whisper_model_url="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"
    mkdir -p "$whisper_model_dir"
    if [ -f "$whisper_model_src" ]; then
        if [ ! -f "$whisper_model_dest" ]; then
            cp "$whisper_model_src" "$whisper_model_dest"
            echo -e "${GREEN}[OK]${NC} Whisper small model installed to $whisper_model_dir"
        fi
    else
        if [ ! -f "$whisper_model_dest" ]; then
            echo "Whisper small model not found locally. Downloading..."
            if command -v wget &> /dev/null; then
                wget -q --show-progress "$whisper_model_url" -O "$whisper_model_dest"
            elif command -v curl &> /dev/null; then
                curl -L "$whisper_model_url" -o "$whisper_model_dest" --progress-bar
            else
                echo -e "${RED}[ERROR]${NC} wget or curl is required to download ggml-small.bin"
                echo "Install one of them or place ggml-small.bin at $whisper_model_src"
                exit 1
            fi
            echo -e "${GREEN}[OK]${NC} Whisper small model downloaded to $whisper_model_dir"
        fi
    fi

    # Verify core dependencies were installed successfully
    if ! command -v ffmpeg &> /dev/null; then
        echo -e "${RED}[ERROR] Missing required dependency after install attempt.${NC}"
        echo "Please install: ffmpeg"
        exit 1
    fi
    if ! command -v gst-launch-1.0 &> /dev/null; then
        echo -e "${RED}[ERROR] Missing required dependency after install attempt.${NC}"
        echo "Please install: gstreamer"
        exit 1
    fi
    if ! pkg-config --exists gstreamer-1.0 2>/dev/null; then
        echo -e "${RED}[ERROR] Missing GStreamer development headers after install attempt.${NC}"
        echo "Please install: gstreamer-devel (or libgstreamer1.0-dev on Debian/Ubuntu)"
        exit 1
    fi
    if [ "$SKIP_DVD_TOOLS" = false ]; then
        if ! command -v dvdauthor &> /dev/null; then
            echo -e "${RED}[ERROR] Missing required dependencies after install attempt.${NC}"
            echo "Please install: dvdauthor"
            exit 1
        fi
        if ! command -v xorriso &> /dev/null; then
            echo -e "${RED}[ERROR] Missing xorriso after install attempt.${NC}"
            echo "Please install: xorriso (required for DVD ISO extraction)"
            exit 1
        fi
    fi
fi

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "Dependency Installation Complete!"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Next steps:"
echo ""
echo "1. Build VideoTools:"
echo "   ./scripts/build.sh"
echo ""
echo "2. Run VideoTools:"
echo "   ./scripts/run.sh"
echo ""
echo "For more information, see BUILD_AND_RUN.md and DVD_USER_GUIDE.md"
echo ""
