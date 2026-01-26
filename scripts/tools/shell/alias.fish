#!/usr/bin/env fish
# VideoTools Convenience Script (fish)
# Source this file in fish to add the 'VideoTools' command

if not set -q FISH_VERSION
    echo "This script is for fish. Use scripts/alias.sh or scripts/alias.zsh instead."
    return 1
end

set -l script_path (status -f)
set -l project_root (dirname (dirname $script_path))

alias VideoTools="bash $project_root/scripts/run.sh"

function VideoToolsRebuild
    echo "Rebuilding VideoTools..."
    bash "$project_root/scripts/build.sh"
end

function VideoToolsClean
    echo "Cleaning VideoTools build artifacts..."
    cd "$project_root"
    go clean -cache -modcache -testcache
    rm -f "$project_root/VideoTools"
    echo "Clean complete"
end
