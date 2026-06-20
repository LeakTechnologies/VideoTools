package main

import (
	"fmt"
	"log"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/player"
)

func main() {
	fmt.Println("VideoTools VT_Player Demo")
	fmt.Println("=========================")

	// Create player configuration
	config := &player.Config{
		Backend:       player.BackendAuto,
		Volume:        50.0,
		AutoPlay:      false,
		HardwareAccel: true,
	}

	// Create factory
	factory := player.NewFactory(config)

	// Show available backends
	backends := factory.GetAvailableBackends()
	fmt.Printf("Available backends: %v\n", backends)

	// Create player
	vtPlayer, err := factory.CreatePlayer()
	if err != nil {
		log.Fatalf("Failed to create player: %v", err)
	}

	fmt.Printf("Created player with backend: %T\n", vtPlayer)

	// Set up callbacks
	vtPlayer.SetTimeCallback(func(t time.Duration) {
		fmt.Printf("Time: %v\n", t)
	})

	vtPlayer.SetFrameCallback(func(frame int64) {
		fmt.Printf("Frame: %d\n", frame)
	})

	vtPlayer.SetStateCallback(func(state player.PlayerState) {
		fmt.Printf("State: %v\n", state)
	})

	// Demo usage
	fmt.Println("\nPlayer created successfully!")
	fmt.Println("Player features:")
	fmt.Println("- Frame-accurate seeking")
	fmt.Println("- Multiple backend support (MPV, VLC, FFplay)")
	fmt.Println("- Fyne UI integration")
	fmt.Println("- Preview mode for trim/upscale modules")
	fmt.Println("- Microsecond precision timing")

	// Test player methods
	fmt.Printf("Current volume: %.1f\n", vtPlayer.GetVolume())
	fmt.Printf("Current speed: %.1f\n", vtPlayer.GetSpeed())
	fmt.Printf("Preview mode: %v\n", vtPlayer.IsPreviewMode())

	// Test video info (empty until file loaded)
	info := vtPlayer.GetVideoInfo()
	fmt.Printf("Video info: %+v\n", info)

	fmt.Println("\nTo use with actual video files:")
	fmt.Println("1. Load a video: vtPlayer.Load(\"path/to/video.mp4\", 0)")
	fmt.Println("2. Play: vtPlayer.Play()")
	fmt.Println("3. Seek to time: vtPlayer.SeekToTime(10 * time.Second)")
	fmt.Println("4. Seek to frame: vtPlayer.SeekToFrame(300)")
	fmt.Println("5. Extract frame: vtPlayer.ExtractFrame(5 * time.Second)")

	// Clean up
	vtPlayer.Close()
	fmt.Println("\nPlayer closed successfully!")
}
