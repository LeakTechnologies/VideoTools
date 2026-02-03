#!/bin/bash
# VideoTools Convenience Script (bash)
# Source this file in bash to add the 'VideoTools' command

if [ -z "$BASH_VERSION" ]; then
    echo "This script is for bash. Use scripts/alias.zsh or scripts/alias.fish instead."
    return 1
fi

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Create alias and function for VideoTools
alias VideoTools="bash $PROJECT_ROOT/scripts/linux/run.sh"

# Also create a rebuild function for quick rebuilds
VideoToolsRebuild() {
    echo "Rebuilding VideoTools..."
    bash "$PROJECT_ROOT/scripts/linux/build.sh"
}

# Create a clean function
VideoToolsClean() {
    echo "Cleaning VideoTools build artifacts..."
    cd "$PROJECT_ROOT"
    go clean -cache -modcache -testcache
    rm -f "$PROJECT_ROOT/VideoTools"
    echo "Clean complete"
}

# VideoTools commands loaded silently
# Available commands: VideoTools, VideoToolsRebuild, VideoToolsClean
