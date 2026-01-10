#!/bin/bash
# VideoTools Debug Run Script
# Runs VideoTools with debug logging enabled and outputs to both console and log file

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/logs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$LOG_DIR/videotools_debug_${TIMESTAMP}.log"

# Create logs directory if it doesn't exist
mkdir -p "$LOG_DIR"

echo "════════════════════════════════════════════════════════════════"
echo "  VideoTools Debug Mode"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Debug output will be saved to:"
echo "  $LOG_FILE"
echo ""
echo "Press Ctrl+C to stop VideoTools"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Run VideoTools with debug flag, output to both console and log file
cd "$PROJECT_ROOT"
./VideoTools --debug 2>&1 | tee "$LOG_FILE"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "VideoTools stopped. Debug log saved to:"
echo "  $LOG_FILE"
echo "════════════════════════════════════════════════════════════════"
