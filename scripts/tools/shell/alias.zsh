#!/usr/bin/env zsh
# VideoTools Convenience Script (zsh)
# Source this file in zsh to add the 'VideoTools' command

if [[ -z "$ZSH_VERSION" ]]; then
    echo "This script is for zsh. Use scripts/alias.sh or scripts/alias.fish instead."
    return 1
fi

SCRIPT_PATH="${(%):-%N}"
PROJECT_ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"

alias VideoTools="bash $PROJECT_ROOT/scripts/linux/run.sh"

VideoToolsRebuild() {
    echo "Rebuilding VideoTools..."
    bash "$PROJECT_ROOT/scripts/linux/build.sh"
}

VideoToolsClean() {
    echo "Cleaning VideoTools build artifacts..."
    cd "$PROJECT_ROOT" || return 1
    go clean -cache -modcache -testcache
    rm -f "$PROJECT_ROOT/VideoTools"
    echo "Clean complete"
}
