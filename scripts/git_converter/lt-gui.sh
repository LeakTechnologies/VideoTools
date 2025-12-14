#!/bin/bash
# ===================================================================
#  LT-Convert GUI - Simple VideoTools Interface
#  Author: Jake (LT Convert)
#  Purpose: Simple GUI for lt-convert.sh configuration
# ===================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "LT-Convert GUI - VideoTools Interface"
echo "=================================="
echo

# Check if lt-convert.sh exists
if [[ ! -f "$SCRIPT_DIR/lt-convert.sh" ]]; then
    echo "ERROR: lt-convert.sh not found"
    echo "Please ensure lt-convert.sh is in the same directory"
    exit 1
fi

echo "Configuration Options:"
echo "1) Run lt-convert.sh with current settings"
echo "2) Configure new settings"
echo "3) Exit"
echo

read -p "Enter choice [1-3]: " choice

case $choice in
    1)
        echo "Running lt-convert.sh..."
        bash "$SCRIPT_DIR/lt-convert.sh" "$@"
        ;;
    2)
        echo "Configuration wizard coming soon..."
        echo "For now, please run lt-convert.sh directly"
        ;;
    3)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice"
        ;;
esac

echo "Done."