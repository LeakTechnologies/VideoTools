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
BINARY_NAME="VideoTools"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_INSTALL_PATH="/usr/local/bin"
USER_INSTALL_PATH="$HOME/.local/bin"

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
echo -e "${CYAN}[1/6]${NC} Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Error: Go is not installed or not in PATH${NC}"
    echo "Please install Go 1.21+ from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓${NC} Found Go version: $GO_VERSION"

# Step 2: Check authoring dependencies
echo ""
echo -e "${CYAN}[2/6]${NC} Checking authoring dependencies..."

if [ "$IS_WINDOWS" = true ]; then
    echo "Detected Windows environment."
    if command -v powershell.exe &> /dev/null; then
        powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PROJECT_ROOT/scripts/install-deps-windows.ps1"
        echo -e "${GREEN}✓${NC} Windows dependency installer completed"
    else
        echo -e "${RED}✗ powershell.exe not found.${NC}"
        echo "Please run: $PROJECT_ROOT\\scripts\\install-deps-windows.ps1"
        exit 1
    fi
else
    missing_deps=()
    if ! command -v ffmpeg &> /dev/null; then
        missing_deps+=("ffmpeg")
    fi
    if ! command -v dvdauthor &> /dev/null; then
        missing_deps+=("dvdauthor")
    fi
    if ! command -v mkisofs &> /dev/null && ! command -v genisoimage &> /dev/null && ! command -v xorriso &> /dev/null; then
        missing_deps+=("iso-tool")
    fi

    install_deps=false
    if [ ${#missing_deps[@]} -gt 0 ]; then
        echo -e "${YELLOW}WARNING${NC} Missing dependencies: ${missing_deps[*]}"
        read -p "Install missing dependencies now? [y/N]: " install_choice
        if [[ "$install_choice" =~ ^[Yy]$ ]]; then
            install_deps=true
        fi
    else
        echo -e "${GREEN}✓${NC} All authoring dependencies found"
    fi

    if [ "$install_deps" = true ]; then
        if command -v apt-get &> /dev/null; then
            sudo apt-get update
            sudo apt-get install -y ffmpeg dvdauthor genisoimage
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y ffmpeg dvdauthor genisoimage
        elif command -v pacman &> /dev/null; then
            sudo pacman -Sy --noconfirm ffmpeg dvdauthor cdrtools
        elif command -v zypper &> /dev/null; then
            sudo zypper install -y ffmpeg dvdauthor genisoimage
        elif command -v brew &> /dev/null; then
            brew install ffmpeg dvdauthor xorriso
        else
            echo -e "${RED}✗ No supported package manager found.${NC}"
            echo "Please install: ffmpeg, dvdauthor, and mkisofs/genisoimage/xorriso"
            exit 1
        fi
    fi

    if ! command -v ffmpeg &> /dev/null || ! command -v dvdauthor &> /dev/null; then
        echo -e "${RED}✗ Missing required dependencies after install attempt.${NC}"
        echo "Please install: ffmpeg and dvdauthor"
        exit 1
    fi
    if ! command -v mkisofs &> /dev/null && ! command -v genisoimage &> /dev/null && ! command -v xorriso &> /dev/null; then
        echo -e "${RED}✗ Missing ISO creation tool after install attempt.${NC}"
        echo "Please install: mkisofs (cdrtools), genisoimage, or xorriso"
        exit 1
    fi
fi

# Step 3: Build the binary
echo ""
echo -e "${CYAN}[3/6]${NC} Building VideoTools..."
cd "$PROJECT_ROOT"
CGO_ENABLED=1 go build -o "$BINARY_NAME" . > /tmp/videotools-build.log 2>&1 &
BUILD_PID=$!
spinner $BUILD_PID "Building $BINARY_NAME"

if wait $BUILD_PID; then
    echo -e "${GREEN}✓${NC} Build successful"
else
    echo -e "${RED}✗ Build failed${NC}"
    echo ""
    echo "Build log:"
    cat /tmp/videotools-build.log
    rm -f /tmp/videotools-build.log
    exit 1
fi
rm -f /tmp/videotools-build.log

# Step 4: Determine installation path
echo ""
echo -e "${CYAN}[4/6]${NC} Installation path selection"
echo ""
echo "Where would you like to install $BINARY_NAME?"
echo "  1) System-wide (/usr/local/bin) - requires sudo, available to all users"
echo "  2) User-local (~/.local/bin) - no sudo needed, available only to you"
echo ""
read -p "Enter choice [1 or 2, default 2]: " choice
choice=${choice:-2}

case $choice in
    1)
        INSTALL_PATH="$DEFAULT_INSTALL_PATH"
        NEEDS_SUDO=true
        ;;
    2)
        INSTALL_PATH="$USER_INSTALL_PATH"
        NEEDS_SUDO=false
        mkdir -p "$INSTALL_PATH"
        ;;
    *)
        echo -e "${RED}✗ Invalid choice. Exiting.${NC}"
        rm -f "$BINARY_NAME"
        exit 1
        ;;
esac

# Step 5: Install the binary
echo ""
echo -e "${CYAN}[5/6]${NC} Installing binary to $INSTALL_PATH..."
if [ "$NEEDS_SUDO" = true ]; then
    echo "Installing $BINARY_NAME (sudo required)..."
    if sudo install -m 755 "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Installation successful"
    else
        echo -e "${RED}✗ Installation failed${NC}"
        rm -f "$BINARY_NAME"
        exit 1
    fi
else
    install -m 755 "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME" > /dev/null 2>&1 &
    INSTALL_PID=$!
    spinner $INSTALL_PID "Installing $BINARY_NAME"

    if wait $INSTALL_PID; then
        echo -e "${GREEN}✓${NC} Installation successful"
    else
        echo -e "${RED}✗ Installation failed${NC}"
        rm -f "$BINARY_NAME"
        exit 1
    fi
fi

rm -f "$BINARY_NAME"

# Step 6: Setup shell aliases and environment
echo ""
echo -e "${CYAN}[6/6]${NC} Setting up shell environment..."

# Detect shell
if [ -n "$ZSH_VERSION" ]; then
    SHELL_RC="$HOME/.zshrc"
    SHELL_NAME="zsh"
elif [ -n "$BASH_VERSION" ]; then
    SHELL_RC="$HOME/.bashrc"
    SHELL_NAME="bash"
else
    # Default to bash
    SHELL_RC="$HOME/.bashrc"
    SHELL_NAME="bash"
fi

# Create alias setup script
ALIAS_SCRIPT="$PROJECT_ROOT/scripts/alias.sh"

# Add installation path to PATH if needed
if [[ ":$PATH:" != *":$INSTALL_PATH:"* ]]; then
    # Check if PATH export already exists
    if ! grep -q "export PATH.*$INSTALL_PATH" "$SHELL_RC" 2>/dev/null; then
        echo "" >> "$SHELL_RC"
        echo "# VideoTools installation path" >> "$SHELL_RC"
        echo "export PATH=\"$INSTALL_PATH:\$PATH\"" >> "$SHELL_RC"
        echo -e "${GREEN}✓${NC} Added $INSTALL_PATH to PATH in $SHELL_RC"
    fi
fi

# Add alias sourcing if not already present
if ! grep -q "source.*alias.sh" "$SHELL_RC" 2>/dev/null; then
    echo "" >> "$SHELL_RC"
    echo "# VideoTools convenience aliases" >> "$SHELL_RC"
    echo "source \"$ALIAS_SCRIPT\"" >> "$SHELL_RC"
    echo -e "${GREEN}✓${NC} Added VideoTools aliases to $SHELL_RC"
fi

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "Installation Complete!"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Next steps:"
echo ""
echo "1. Reload your shell configuration:"
echo "   source $SHELL_RC"
echo ""
echo "2. Run VideoTools:"
echo "   VideoTools"
echo ""
echo "3. Available commands:"
echo "   - VideoTools              - Run the application"
echo "   - VideoToolsRebuild       - Force rebuild from source"
echo "   - VideoToolsClean         - Clean build artifacts and cache"
echo ""
echo "For more information, see BUILD_AND_RUN.md and DVD_USER_GUIDE.md"
echo ""
