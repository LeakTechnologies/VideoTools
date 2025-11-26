#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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
DEFAULT_INSTALL_PATH="/usr/local/bin"
USER_INSTALL_PATH="$HOME/.local/bin"

echo "========================================="
echo "  VideoTools Installation Script"
echo "========================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
    echo "Please install Go 1.21+ from https://go.dev/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓${NC} Found Go version: $GO_VERSION"

# Build the binary
echo ""
go build -o "$BINARY_NAME" . > /tmp/videotools-build.log 2>&1 &
BUILD_PID=$!
spinner $BUILD_PID "Building $BINARY_NAME"

if wait $BUILD_PID; then
    echo -e "${GREEN}✓${NC} Build successful"
else
    echo -e "${RED}Error: Build failed${NC}"
    cat /tmp/videotools-build.log
    rm -f /tmp/videotools-build.log
    exit 1
fi
rm -f /tmp/videotools-build.log

# Determine installation path
echo ""
echo "Where would you like to install $BINARY_NAME?"
echo "1) System-wide (/usr/local/bin) - requires sudo"
echo "2) User-local (~/.local/bin) - no sudo needed"
read -p "Enter choice [1 or 2]: " choice

case $choice in
    1)
        INSTALL_PATH="$DEFAULT_INSTALL_PATH"
        NEEDS_SUDO=true
        ;;
    2)
        INSTALL_PATH="$USER_INSTALL_PATH"
        NEEDS_SUDO=false
        # Create ~/.local/bin if it doesn't exist
        mkdir -p "$INSTALL_PATH"
        ;;
    *)
        echo -e "${RED}Invalid choice. Exiting.${NC}"
        rm -f "$BINARY_NAME"
        exit 1
        ;;
esac

# Install the binary
echo ""
if [ "$NEEDS_SUDO" = true ]; then
    sudo install -m 755 "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME" > /dev/null 2>&1 &
    INSTALL_PID=$!
    spinner $INSTALL_PID "Installing $BINARY_NAME to $INSTALL_PATH"

    if wait $INSTALL_PID; then
        echo -e "${GREEN}✓${NC} Installation successful"
    else
        echo -e "${RED}Error: Installation failed${NC}"
        rm -f "$BINARY_NAME"
        exit 1
    fi
else
    install -m 755 "$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME" > /dev/null 2>&1 &
    INSTALL_PID=$!
    spinner $INSTALL_PID "Installing $BINARY_NAME to $INSTALL_PATH"

    if wait $INSTALL_PID; then
        echo -e "${GREEN}✓${NC} Installation successful"
    else
        echo -e "${RED}Error: Installation failed${NC}"
        rm -f "$BINARY_NAME"
        exit 1
    fi
fi

# Clean up the local binary
rm -f "$BINARY_NAME"

# Check if install path is in PATH
echo ""
if [[ ":$PATH:" != *":$INSTALL_PATH:"* ]]; then
    echo -e "${YELLOW}Warning: $INSTALL_PATH is not in your PATH${NC}"
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo ""
    echo "    export PATH=\"$INSTALL_PATH:\$PATH\""
    echo ""
fi

echo "========================================="
echo -e "${GREEN}Installation complete!${NC}"
echo "========================================="
echo ""
echo "You can now run: $BINARY_NAME"
echo ""
