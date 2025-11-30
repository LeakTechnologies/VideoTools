#!/bin/bash
# VideoTools Run Script
# Builds (if needed) and runs the application

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_OUTPUT="$PROJECT_ROOT/VideoTools"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools - Run Script"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check if binary exists
if [ ! -f "$BUILD_OUTPUT" ]; then
    echo "⚠️  Binary not found. Building..."
    echo ""
    bash "$PROJECT_ROOT/scripts/build.sh"
    echo ""
fi

# Verify binary exists
if [ ! -f "$BUILD_OUTPUT" ]; then
    echo "❌ ERROR: Build failed, cannot run."
    exit 1
fi

echo "🚀 Starting VideoTools..."
echo "════════════════════════════════════════════════════════════════"
echo ""

# Run the application
"$BUILD_OUTPUT" "$@"
