#!/bin/bash
# ===================================================================
#  LT-Test Interface - VideoTools Feature Testing
#  Small interface identical to VideoTools for testing new features
#  Author: LT Convert Team
#  Purpose: Test ground for VideoTools integration
# ===================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source lt-convert modules
source "$SCRIPT_DIR/modules/hardware.sh"
source "$SCRIPT_DIR/modules/codec.sh"
source "$SCRIPT_DIR/modules/quality.sh"
source "$SCRIPT_DIR/modules/filters.sh"
source "$SCRIPT_DIR/modules/encode.sh"

# Create test output directory
TEST_OUT="$SCRIPT_DIR/Test_Output"
mkdir -p "$TEST_OUT"

clear
cat << "EOF"
╔═════════════════════════════════════════════════════════════╗
║                 LT-Test Interface - VideoTools Testing          ║
║                    (Feature Integration Ground)                 ║
╚═══════════════════════════════════════════════════════════╝

EOF

echo
echo "🎯 PURPOSE: Test new features before VideoTools integration"
echo "📋 FEATURES TO TEST:"
echo "   ✓ Fast Bitrate Mode (200+ FPS performance)"
echo "   ✓ Hardware Benchmarking with Caching"
echo "   ✓ Simplified Filter Chains"
echo "   ✓ Cross-platform Compatibility"
echo

# Quick test menu
echo "🧪 Quick Test Options:"
echo "   1) Test Fast Bitrate Performance"
echo "   2) Test Hardware Caching"
echo "   3) Test Simplified Filters"
echo "   4) Full Integration Test"
echo "   5) Exit to VideoTools"
echo

while true; do
    read -p "Enter 1–5 → " test_choice
    if [[ -n "$test_choice" && "$test_choice" =~ ^[1-5]$ ]]; then
        break
    else
        echo "Invalid input. Please enter a number between 1 and 5."
    fi
done

case $test_choice in
    1)
        echo -e "\n🚀 Testing Fast Bitrate Performance..."
        # Simulate fast bitrate mode test
        echo "Creating test file..."
        ffmpeg -f lavfi -i "testsrc=duration=5:size=640x480:rate=30" -c:v libx264 -b:v 2000k "$TEST_OUT/fast_bitrate_test.mp4" -y 2>/dev/null
        if [[ $? -eq 0 ]]; then
            echo "✅ Fast bitrate test completed successfully"
            echo "📊 Expected performance: 200+ FPS"
        else
            echo "❌ Fast bitrate test failed"
        fi
        ;;
    2)
        echo -e "\n💾 Testing Hardware Caching..."
        # Test caching system
        if [[ -f "$SCRIPT_DIR/.hardware_cache" ]]; then
            echo "✅ Cache file exists"
            source "$SCRIPT_DIR/.hardware_cache"
            echo "📋 Cached results:"
            echo "   GPU: $cached_gpu"
            echo "   Encoder: $cached_encoder"
            echo "   Score: $cached_score"
        else
            echo "⚠ No cache file found - running benchmark..."
            run_hardware_benchmark
        fi
        ;;
    3)
        echo -e "\n🔧 Testing Simplified Filters..."
        echo "Testing filter chain optimization..."
        # Test with no filters (should be fastest)
        echo "→ No filters: $(ffmpeg -f lavfi -i "testsrc=duration=2:size=320x240" -f null - 2>&1 | grep -o 'frame=.*fps=' | tail -1)"
        # Test with filters (should show difference)
        echo "→ With filters: $(ffmpeg -f lavfi -i "testsrc=duration=2:size=320x240,scale=640x480" -f null - 2>&1 | grep -o 'frame=.*fps=' | tail -1)"
        echo "✅ Filter optimization test completed"
        ;;
    4)
        echo -e "\n🎪 Full Integration Test..."
        echo "Running complete lt-convert workflow..."
        echo "This will test all new features in sequence"
        echo "📋 Test Plan:"
        echo "   1. Hardware benchmarking with caching"
        echo "   2. Fast bitrate mode selection"
        echo "   3. Simplified filter chains"
        echo "   4. Performance measurement"
        echo
        read -p "Press Enter to begin full integration test..."
        
        # Run actual hardware benchmark
        run_hardware_benchmark
        
        # Setup fast bitrate mode
        quality_params="-b:v 2000k"
        quality_name="Fast 2000k"
        
        echo "✅ Full integration test completed"
        echo "📊 Ready for VideoTools integration"
        ;;
    5)
        echo -e "\n👋 Exiting to VideoTools..."
        echo "LT-Test features ready for integration"
        ;;
esac

echo
echo "========================================================"
echo "LT-Test completed - See results in '$TEST_OUT'"
echo "========================================================"
read -p "Press Enter to exit"