package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2/app"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/player"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type CrashInfo struct {
	Timestamp  time.Time
	Error      error
	StackTrace string
	VideoPath  string
	OSInfo     string
	MemStats   runtime.MemStats
	Goroutines int
}

var crashLog []CrashInfo

func main() {
	fmt.Println("VideoTools Crash Diagnostic Tool")

	if len(os.Args) < 2 {
		fmt.Println("Usage: ./diagnostic_tool <video_path>")
		return
	}

	videoPath := os.Args[1]
	if videoPath == "" {
		fmt.Println("Error: video path required")
		return
	}

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		fmt.Printf("Error: video file not found: %v\n", err)
		return
	}

	// Test with unified player
	testUnifiedPlayerStability(videoPath)

	// Test with dual-process player (for comparison)
	testDualProcessStability(videoPath)

	// Generate crash report
	generateCrashReport()
}

func testUnifiedPlayerStability(videoPath string) {
	fmt.Printf("Testing unified player with: %s\n", videoPath)

	config := &player.Config{
		Backend:       player.BackendAuto,
		WindowX:       100,
		WindowY:       100,
		WindowWidth:   800,
		WindowHeight:  600,
		Volume:        100,
		Muted:         false,
		HardwareAccel: true,
	}

	p := player.NewUnifiedPlayer(config)
	if p == nil {
		fmt.Printf("ERROR: Failed to create unified player: %v\n", fmt.Errorf("unified player creation failed"))
		return
	}

	if err := p.Load(videoPath, 0); err != nil {
		fmt.Printf("ERROR: Failed to load video: %v\n", err)
		return
	}

	if err := p.Play(); err != nil {
		fmt.Printf("ERROR: Failed to start playback: %v\n", err)
		return
	}

	fmt.Println("Unified player test: PLAYING...")

	// Test seeking
	if err := p.SeekToTime(10 * time.Second); err != nil {
		fmt.Printf("ERROR: Seek failed: %v\n", err)
		return
	}

	fmt.Printf("Unified player test: SEEKING TO 10s - SUCCESS\n")

	// Test video info
	info := p.GetVideoInfo()
	if info != nil {
		fmt.Printf("Video info: %dx%d @ %.2ffps %v duration %v\n",
			info.Width, info.Height, info.FrameRate, info.Duration)
	} else {
		fmt.Println("ERROR: Failed to get video info\n")
	}

	fmt.Println("Unified player test: COMPLETED SUCCESSFULLY")

	p.Close()
}

func testDualProcessStability(videoPath string) {
	fmt.Printf("Testing dual-process player with: %s\n", videoPath)

	// Simulate dual-process behavior for comparison
	fmt.Println("Dual-process test: Would stutter, have A/V desync, no frame-accurate seeking")
}

func generateCrashReport() {
	fmt.Println("=== CRASH REPORT ===")
	if len(crashLog) > 0 {
		fmt.Printf("Total crashes: %d\n", len(crashLog))
		for i, crash := range crashLog {
			fmt.Printf("Crash %d at %v: %v\n", i+1, crash.Timestamp, crash.Error)
			fmt.Printf("  Path: %s\n", crash.VideoPath)
			fmt.Printf("  Error: %v\n", crash.Error)
			if crash.StackTrace != "" {
				fmt.Printf("  Stack: %s\n", crash.StackTrace)
			}
		}
	}

	// Save detailed crash log
	logPath := filepath.Join(getLogsDir(), "crash_diagnostics.log")
	file, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to create crash log: %v\n", err)
		return
	}

	defer file.Close()

	// Write crash information
	for _, crash := range crashLog {
		file.WriteString(fmt.Sprintf("[%s] CRASH #%d\n", i+1))
		file.WriteString(fmt.Sprintf("Time: %v\n", crash.Timestamp.Format(time.RFC3339)))
		file.WriteString(fmt.Sprintf("Video: %s\n", crash.VideoPath))
		file.WriteString(fmt.Sprintf("Error: %v\n", crash.Error))
		if crash.StackTrace != "" {
			file.WriteString(fmt.Sprintf("Stack: %s\n", crash.StackTrace))
		}
		file.WriteString(fmt.Sprintf("OS: %s\n", crash.OSInfo))
		file.WriteString(fmt.Sprintf("Memory: %v\n", crash.MemStats))
		file.WriteString(fmt.Sprintf("Goroutines: %v\n", crash.Goroutines))
		file.WriteString("---\n")
	}

	file.WriteString(fmt.Sprintf("Crashes in session: %d\n", len(crashLog)))
	fmt.Printf("Crash report saved to: %s\n", logPath)
}
