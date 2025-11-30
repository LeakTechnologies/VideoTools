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
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_INSTALL_PATH="/usr/local/bin"
USER_INSTALL_PATH="$HOME/.local/bin"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Professional Installation"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Step 1: Check if Go is installed
echo -e "${CYAN}[1/5]${NC} Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Error: Go is not installed or not in PATH${NC}"
    echo "Please install Go 1.21+ from https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓${NC} Found Go version: $GO_VERSION"

# Step 2: Build the binary
echo ""
echo -e "${CYAN}[2/5]${NC} Building VideoTools..."
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

# Step 3: Determine installation path
echo ""
echo -e "${CYAN}[3/5]${NC} Installation path selection"
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

# Step 4: Install the binary
echo ""
echo -e "${CYAN}[4/5]${NC} Installing binary to $INSTALL_PATH..."
if [ "$NEEDS_SUDO" = true ]; then
    sudo install -m 755 "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME" > /dev/null 2>&1 &
    INSTALL_PID=$!
    spinner $INSTALL_PID "Installing $BINARY_NAME"

    if wait $INSTALL_PID; then
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

# Step 5: Setup shell aliases and environment
echo ""
echo -e "${CYAN}[5/5]${NC} Setting up shell environment..."

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
echo -e "${GREEN}Installation Complete!${NC}"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Next steps:"
echo ""
echo "1. ${CYAN}Reload your shell configuration:${NC}"
echo "   source $SHELL_RC"
echo ""
echo "2. ${CYAN}Run VideoTools:${NC}"
echo "   VideoTools"
echo ""
echo "3. ${CYAN}Available commands:${NC}"
echo "   • VideoTools              - Run the application"
echo "   • VideoToolsRebuild       - Force rebuild from source"
echo "   • VideoToolsClean         - Clean build artifacts and cache"
echo ""
echo "For more information, see BUILD_AND_RUN.md and DVD_USER_GUIDE.md"
echo ""
