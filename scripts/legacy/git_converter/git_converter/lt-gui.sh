#!/bin/bash
# ===================================================================
#  LT-Convert GUI - VideoTools Style Interface
#  Author: LeakTechnologies
#  Fyne-based GUI matching VideoTools design
# ===================================================================

# Check if Fyne is available
if ! command -v fyne &> /dev/null; then
    echo "❌ Fyne not found. Please install Fyne:"
    echo "   go install fyne.io/fyne/v2/cmd/fyne@latest"
    exit 1
fi

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/modules/hardware.sh"
source "$SCRIPT_DIR/modules/codec.sh"
source "$SCRIPT_DIR/modules/quality.sh"
source "$SCRIPT_DIR/modules/filters.sh"
source "$SCRIPT_DIR/modules/encode.sh"

# VideoTools-style colors
GRID_COLOR="#2C3E50"
TEXT_COLOR="#FFFFFF"
ACCENT_COLOR="#00BCD4"
SUCCESS_COLOR="#4CAF50"
WARNING_COLOR="#FF9800"
ERROR_COLOR="#F44336"

# Create main GUI
create_main_gui() {
    echo "🔨 Building Go GUI application..."
    
    # Create temporary directory for build
    mkdir -p /tmp/lt-gui-build
    cd /tmp/lt-gui-build
    
    cat > lt_convert_gui.go << 'EOF'
package main

import (
    "fmt"
    "image/color"
    "os"
    "path/filepath"
    "strings"
    
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/storage"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
)

type ConversionConfig struct {
    VideoFiles     []string
    SelectedEncoder string
    SelectedQuality string
    SelectedResolution string
    SelectedFPS     string
    SelectedColor   string
    OutputContainer string
}

var config ConversionConfig

func main() {
    config = ConversionConfig{
        VideoFiles:     getVideoFiles(),
        SelectedEncoder: "Auto-detect",
        SelectedQuality: "High Quality (CRF 18)",
        SelectedResolution: "Source",
        SelectedFPS:     "Original",
        SelectedColor:   "No color correction",
        OutputContainer: "mkv",
    }
    
    a := app.New()
    w := a.NewWindow("LT-Convert - VideoTools Style")
    w.Resize(fyne.NewSize(1000, 700))
    w.SetContent(buildMainUI())
    w.ShowAndRun()
}

func getVideoFiles() []string {
    // Get video files from current directory
    files, err := os.ReadDir(".")
    if err != nil {
        return []string{}
    }
    
    var videos []string
    for _, file := range files {
        if !file.IsDir() && isVideoFile(file.Name()) {
            videos = append(videos, file.Name())
        }
    }
    return videos
}

func isVideoFile(filename string) bool {
    ext := strings.ToLower(filepath.Ext(filename))
    videoExts := []string{".mp4", ".mkv", ".mov", ".avi", ".wmv", ".ts", ".m2ts", ".flv", ".webm", ".m4v"}
    for _, ve := range videoExts {
        if ext == ve {
            return true
        }
    }
    return false
}

func buildMainUI() fyne.CanvasObject {
    // Title
    title := canvas.NewText("LT-CONVERT", color.White)
    title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
    title.TextSize = 28
    
    // File list
    fileLabel := widget.NewLabel("VIDEO FILES")
    fileLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    var fileList *widget.List
    if len(config.VideoFiles) > 0 {
        fileList = widget.NewList([]string{"No video files found"}, func(i widget.ListItemID, o widget.ListItemID, v widget.ListItemID) {})
    } else {
        fileList = widget.NewList(config.VideoFiles, func(i widget.ListItemID, o widget.ListItemID, v widget.ListItemID) {})
    }
    
    // Settings sections
    settingsContainer := buildSettingsContainer()
    
    // Action buttons
    startBtn := widget.NewButton("START CONVERSION", func() {
        startConversion()
    })
    startBtn.Importance = widget.HighImportance
    
    quitBtn := widget.NewButton("QUIT", func() {
        os.Exit(0)
    })
    quitBtn.Importance = widget.LowImportance
    
    // Layout
    header := container.NewHBox(
        container.NewCenter(title),
        container.NewSpacer(),
        quitBtn,
    )
    
    content := container.NewVBox(
        header,
        widget.NewSeparator(),
        container.NewHSplit(
            container.NewVBox(
                fileLabel,
                fileList,
            ),
            settingsContainer,
        ),
        widget.NewSeparator(),
        container.NewHBox(
            container.NewSpacer(),
            startBtn,
        ),
    )
    
    return container.NewPadded(content)
}

func buildSettingsContainer() fyne.CanvasObject {
    // Encoder selection
    encoderLabel := widget.NewLabel("ENCODER")
    encoderLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    encoderSelect := widget.NewSelect([]string{
        "Auto-detect",
        "NVIDIA HEVC NVENC",
        "NVIDIA AV1 NVENC", 
        "AMD HEVC AMF",
        "AMD AV1 AMF",
        "Intel HEVC QSV",
        "Intel AV1 QSV",
        "CPU SVT-AV1",
        "CPU x265 HEVC",
    }, func(selected string) {
        config.SelectedEncoder = selected
    })
    encoderSelect.SetSelected(config.SelectedEncoder)
    
    // Quality selection
    qualityLabel := widget.NewLabel("QUALITY")
    qualityLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    qualitySelect := widget.NewSelect([]string{
        "Source quality (bypass)",
        "High Quality (CRF 18)",
        "Good Quality (CRF 20)",
        "DVD-NTSC Professional",
        "DVD-PAL Professional", 
        "Custom bitrate",
    }, func(selected string) {
        config.SelectedQuality = selected
    })
    qualitySelect.SetSelected(config.SelectedQuality)
    
    // Resolution selection
    resolutionLabel := widget.NewLabel("RESOLUTION")
    resolutionLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    resolutionSelect := widget.NewSelect([]string{
        "Source file resolution",
        "720p (1280×720)",
        "1080p (1920×1080)",
        "1440p (2560×1440)",
        "4K (3840×2160)",
        "2X Upscale",
        "4X Upscale",
    }, func(selected string) {
        config.SelectedResolution = selected
    })
    resolutionSelect.SetSelected(config.SelectedResolution)
    
    // FPS selection
    fpsLabel := widget.NewLabel("FPS")
    fpsLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    fpsSelect := widget.NewSelect([]string{
        "Original FPS",
        "60 FPS",
    }, func(selected string) {
        config.SelectedFPS = selected
    })
    fpsSelect.SetSelected(config.SelectedFPS)
    
    // Color correction
    colorLabel := widget.NewLabel("COLOR")
    colorLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    colorSelect := widget.NewSelect([]string{
        "No color correction",
        "Fix pink skin tones (Topaz AI)",
        "Warm enhancement",
        "Cool enhancement",
        "2000s DVD Restore",
        "90s Quality Restore", 
        "VHS Quality Restore",
        "Anime Preservation",
    }, func(selected string) {
        config.SelectedColor = selected
    })
    colorSelect.SetSelected(config.SelectedColor)
    
    // Container selection
    containerLabel := widget.NewLabel("CONTAINER")
    containerLabel.TextStyle = fyne.TextStyle{Bold: true}
    
    containerSelect := widget.NewSelect([]string{
        "MKV (flexible)",
        "MP4 (compatible)",
    }, func(selected string) {
        config.OutputContainer = strings.ToLower(strings.Fields(selected)[0])
    })
    containerSelect.SetSelected("MKV (flexible)")
    
    // Layout settings in grid
    settingsGrid := container.NewGridWithColumns(2,
        encoderLabel, encoderSelect,
        qualityLabel, qualitySelect,
        resolutionLabel, resolutionSelect,
        fpsLabel, fpsSelect,
        colorLabel, colorSelect,
        containerLabel, containerSelect,
    )
    
    return container.NewVBox(
        widget.NewCard(settingsGrid),
    )
}

func startConversion() {
    if len(config.VideoFiles) == 0 {
        dialog.ShowError("No video files found", "Please add video files to the directory and restart.")
        return
    }
    
    // Show confirmation dialog
    dialog.ShowConfirm("Start Conversion", 
        fmt.Sprintf("Convert %d file(s) with these settings?\n\nEncoder: %s\nQuality: %s\nResolution: %s\nFPS: %s\nColor: %s\nContainer: %s", 
            len(config.VideoFiles), 
            config.SelectedEncoder,
            config.SelectedQuality,
            config.SelectedResolution,
            config.SelectedFPS,
            config.SelectedColor,
            config.OutputContainer),
        func(confirmed bool) {
            if confirmed {
                runConversion()
            }
        }, nil)
}

func runConversion() {
    // This would call the bash conversion functions
    // For now, just show success message
    dialog.ShowInformation("Conversion Started", 
        fmt.Sprintf("Converting %d file(s)...\n\nSettings applied:\n- Encoder: %s\n- Quality: %s\n- Resolution: %s\n- FPS: %s\n- Color: %s\n- Container: %s", 
            len(config.VideoFiles),
            config.SelectedEncoder,
            config.SelectedQuality, 
            config.SelectedResolution,
            config.SelectedFPS,
            config.SelectedColor,
            config.OutputContainer))
}
EOF

# Check if Windows x64
detect_windows_x64() {
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && [[ "$(uname -m)" == "x86_64" ]]; then
        return 0
    fi
    return 1
}

# Pre-built Windows x64 GUI
build_windows_exe() {
    if detect_windows_x64; then
        echo "🎯 Detected Windows x64 - checking for pre-built GUI..."
        
        # Check for pre-built executable
        if [[ -f "$SCRIPT_DIR/lt-convert-gui.exe" ]]; then
            echo "✅ Found pre-built Windows GUI executable"
            return 0
        else
            echo "⚠️  Pre-built GUI not found, building from source..."
            return 1
        fi
    else
        return 1
    fi
}

# Build GUI from source (fallback)
build_gui_from_source() {
    echo "🔨 Building GUI from source..."
    
    cat > lt_convert_gui.go << 'ENDOFFILE'
package main

import (
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app" 
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/widget"
)

func main() {
    a := app.New()
    w := a.NewWindow("LT-Convert GUI")
    w.Resize(fyne.NewSize(800, 600))
    
    content := container.NewVBox(
        widget.NewLabelWithStyle("LT-Convert GUI", fyne.TextStyle{Bold: true}),
        widget.NewSeparator(),
        widget.NewLabel("VideoTools Style Interface v1.0"),
        widget.NewSeparator(),
        widget.NewButton("Launch Bash Mode", func() {
            w.Close()
        }),
        widget.NewButton("Quit", func() {
            a.Quit()
        }),
    )
    
    w.SetContent(content)
    w.ShowAndRun()
}
ENDOFFILE

    if go build -o lt-convert-gui lt_convert_gui.go 2>/dev/null; then
        echo "✅ GUI built successfully"
        return 0
    else
        echo "❌ Build failed"
        return 1
    fi
}

# Unified GUI builder
build_gui() {
    echo "🔨 Building GUI application..."
    
    # Try Windows x64 pre-built first
    if build_windows_exe; then
        echo "🚀 Launching pre-built Windows GUI..."
        cd "$SCRIPT_DIR"
        ./lt-convert-gui.exe
        return 0
    fi
    
    # Fallback to source build
    echo "📝 Building from source (fallback)..."
    build_gui_from_source
    
    if [[ $? -eq 0 ]]; then
        echo "🚀 Launching GUI..."
        if [[ -f "./lt-convert-gui" ]] || [[ -f "./lt-convert-gui.exe" ]]; then
            echo "✅ GUI executable ready"
            return 0
        else
            echo "❌ GUI executable not found after build"
            return 1
        fi
    else
        echo "❌ GUI build failed"
        return 1
    fi
}
ENDOFFILE

    if go build -o lt-convert-gui lt_convert_gui.go 2>/dev/null; then
        echo "✅ GUI built successfully"
        return 0
    else
        echo "❌ Build failed"
        return 1
    fi
}
EOF

    # Try to build
    echo "Building Go application..."
    if go build -o lt-convert-gui lt_convert_gui.go 2>/dev/null; then
        echo "✅ GUI built successfully"
        return 0
    else
        echo "❌ Build failed"
        return 1
    fi
}
GO_EOF

    echo "🔨 Building GUI..."
    
    # Build with error handling
    if go build -o lt-convert-gui lt_convert_gui.go 2>build.log; then
        echo "✅ GUI built successfully"
        return 0
    else
        echo "❌ GUI build failed. Check build.log:"
        cat build.log 2>/dev/null || echo "No build log created"
        return 1
    fi
}

func buildSimpleUI() fyne.CanvasObject {
    title := container.NewVBox(
        widget.NewLabelWithStyle("LT-CONVERT", fyne.TextStyle{Bold: true, Monospace: true}),
        widget.NewLabel("VideoTools Style GUI"),
    )
    
    // Encoder selection
    encoderSelect := widget.NewSelect([]string{
        "Auto-detect",
        "NVIDIA HEVC NVENC",
        "NVIDIA AV1 NVENC",
        "AMD HEVC AMF", 
        "AMD AV1 AMF",
        "Intel HEVC QSV",
        "Intel AV1 QSV",
        "CPU SVT-AV1",
        "CPU x265 HEVC",
    }, func(selected string) {
        fmt.Printf("Selected encoder: %s\n", selected)
    })
    
    // Quality selection
    qualitySelect := widget.NewSelect([]string{
        "Source quality (bypass)",
        "High Quality (CRF 18)",
        "Good Quality (CRF 20)",
        "DVD-NTSC Professional",
        "DVD-PAL Professional",
        "Custom bitrate",
    }, func(selected string) {
        fmt.Printf("Selected quality: %s\n", selected)
    })
    
    // Start button
    startBtn := widget.NewButton("START CONVERSION WITH BASH", func() {
        dialog.ShowConfirm("Start Conversion", "This will launch the bash version with your settings. Continue?", func(confirmed bool) {
            if confirmed {
                fmt.Println("Starting bash conversion...")
                // In real implementation, this would call lt-convert.sh with saved settings
                os.Exit(0)
            }
        }, nil)
    })
    startBtn.Importance = widget.HighImportance
    
    quitBtn := widget.NewButton("QUIT", func() {
        os.Exit(0)
    })
    
    return container.NewVBox(
        title,
        widget.NewSeparator(),
        container.NewGridWithColumns(2,
            widget.NewLabel("Encoder:"), encoderSelect,
            widget.NewLabel("Quality:"), qualitySelect,
        ),
        widget.NewSeparator(),
        container.NewHBox(
            widget.NewLabel("Click START to run bash version"),
            container.NewSpacer(),
            startBtn,
            quitBtn,
        ),
    )
}
GO_EOF

    # Try to build
    if go build -o lt-convert-gui lt_convert_gui.go 2>/dev/null; then
        echo "✅ GUI built successfully"
        return 0
    else
        echo "❌ GUI build failed"
        return 1
    fi
}
}

# Check dependencies
check_dependencies() {
    echo "🔍 Checking dependencies..."
    
    if ! command -v go &> /dev/null; then
        echo "❌ Go not found. Please install Go:"
        echo "   https://golang.org/dl/"
        return 1
    fi
    
    if ! command -v fyne &> /dev/null; then
        echo "❌ Fyne not found. Installing Fyne..."
        go install fyne.io/fyne/v2/cmd/fyne@latest
        if [[ $? -ne 0 ]]; then
            echo "❌ Failed to install Fyne"
            return 1
        fi
    fi
    
    echo "✅ Dependencies ready"
    return 0
}

# Main execution
main() {
    clear
    cat << "EOF"
╔════════════════════════════════════════════════════════╗
║                    LT-Convert GUI Mode                      ║
║                  (VideoTools Style)                     ║
╚══════════════════════════════════════════════════════════════════════════════╝

Launching modern Fyne-based GUI...
EOF
    
    # Check dependencies
    if ! check_dependencies; then
        echo "❌ Dependency check failed. Falling back to bash mode..."
        echo "Press Enter to continue to lt-convert.sh..."
        read
        bash "$SCRIPT_DIR/lt-convert.sh" "$@"
        return
    fi
    
    # Try to build and run GUI
    if ! build_gui; then
        echo "❌ GUI build failed. Falling back to bash mode..."
        echo "Press Enter to continue to lt-convert.sh..."
        read
        bash "$SCRIPT_DIR/lt-convert.sh" "$@"
        return
    fi
    
    # Run GUI with Windows x64 check
    run_gui() {
        cd "$SCRIPT_DIR"
        if detect_windows_x64; then
            # For Windows x64, check for exe
            if [[ -f "./lt-convert-gui.exe" ]]; then
                echo "🚀 Launching Windows GUI..."
                ./lt-convert-gui.exe
            elif [[ -f "./lt-convert-gui" ]]; then
                echo "🚀 Launching GUI..."
                ./lt-convert-gui
            else
                echo "❌ GUI executable not found"
                return 1
            fi
        else
            # Non-Windows or build failed, run bash version
            echo "📋 Running bash version..."
            bash "$SCRIPT_DIR/lt-convert.sh" "$@"
        fi
    }
}
}

# Run main function
main "$@"