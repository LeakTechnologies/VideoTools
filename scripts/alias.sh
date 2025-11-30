#!/bin/bash
# VideoTools Convenience Script
# Source this file in your shell to add the 'VideoTools' command

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Create alias and function for VideoTools
alias VideoTools="bash $PROJECT_ROOT/scripts/run.sh"

# Also create a rebuild function for quick rebuilds
VideoToolsRebuild() {
    echo "🔨 Rebuilding VideoTools..."
    bash "$PROJECT_ROOT/scripts/build.sh"
}

# Create a clean function
VideoToolsClean() {
    echo "🧹 Cleaning VideoTools build artifacts..."
    cd "$PROJECT_ROOT"
    go clean -cache -modcache -testcache
    rm -f "$PROJECT_ROOT/VideoTools"
    echo "✓ Clean complete"
}

echo "════════════════════════════════════════════════════════════════"
echo "✅ VideoTools Commands Available"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Commands:"
echo "  VideoTools              - Run VideoTools (auto-builds if needed)"
echo "  VideoToolsRebuild       - Force rebuild of VideoTools"
echo "  VideoToolsClean         - Clean build artifacts and cache"
echo ""
echo "To make these permanent, add this line to your ~/.bashrc or ~/.zshrc:"
echo "  source $PROJECT_ROOT/scripts/alias.sh"
echo ""
