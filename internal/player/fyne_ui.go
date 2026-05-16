package player

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"
)

// FynePlayerUI provides a Fyne-based user interface for the VTPlayer
type FynePlayerUI struct {
	app       fyne.App
	window    fyne.Window
	player    VTPlayer
	container *fyne.Container

	// UI Components
	playPauseBtn  *widget.Button
	stopBtn       *widget.Button
	seekSlider    *vtheme.Slider
	timeLabel     *widget.Label
	durationLabel *widget.Label
	volumeSlider  *vtheme.Slider
	fullscreenBtn *widget.Button
	fileBtn       *widget.Button
	frameLabel    *widget.Label
	fpsLabel      *widget.Label

	// State tracking
	isPlaying   bool
	currentTime time.Duration
	duration    time.Duration
	manualSeek  bool
}

// NewFynePlayerUI creates a new Fyne UI for the VTPlayer
func NewFynePlayerUI(app fyne.App, player VTPlayer) *FynePlayerUI {
	ui := &FynePlayerUI{
		app:    app,
		player: player,
		window: app.NewWindow("VideoTools Player"),
	}

	ui.setupUI()
	ui.setupCallbacks()
	ui.setupWindow()

	return ui
}

// setupUI creates the user interface components
func (ui *FynePlayerUI) setupUI() {
	// Control buttons - using text instead of icons for compatibility
	ui.playPauseBtn = widget.NewButton("Play", ui.togglePlayPause)
	ui.stopBtn = widget.NewButton("Stop", ui.stop)
	ui.fullscreenBtn = widget.NewButton("Fullscreen", ui.toggleFullscreen)
	ui.fileBtn = widget.NewButton("Open File", ui.openFile)

	// Time controls
	ui.seekSlider = vtheme.MakeSlider(0, 100)
	ui.seekSlider.OnChanged = ui.onSeekChanged

	ui.timeLabel = widget.NewLabel("00:00:00")
	ui.durationLabel = widget.NewLabel("00:00:00")

	// Volume control
	ui.volumeSlider = vtheme.MakeSlider(0, 100)
	ui.volumeSlider.SetValue(ui.player.GetVolume())
	ui.volumeSlider.OnChanged = ui.onVolumeChanged

	// Info labels
	ui.frameLabel = widget.NewLabel("Frame: 0")
	ui.fpsLabel = widget.NewLabel("FPS: 0.0")

	// Volume percentage label
	volumeLabel := widget.NewLabel(fmt.Sprintf("%.0f%%", ui.player.GetVolume()))

	// Layout containers
	buttonContainer := container.NewHBox(
		ui.fileBtn,
		ui.playPauseBtn,
		ui.stopBtn,
		ui.fullscreenBtn,
	)

	timeContainer := container.NewHBox(
		ui.timeLabel,
		ui.seekSlider,
		ui.durationLabel,
	)

	volumeContainer := container.NewHBox(
		widget.NewLabel("Volume:"),
		ui.volumeSlider,
		volumeLabel,
	)

	infoContainer := container.NewHBox(
		ui.frameLabel,
		ui.fpsLabel,
	)

	// Update volume label when slider changes
	ui.volumeSlider.OnChanged = func(value float64) {
		volumeLabel.SetText(fmt.Sprintf("%.0f%%", value))
		ui.onVolumeChanged(value)
	}

	// Main container
	ui.container = container.NewVBox(
		buttonContainer,
		timeContainer,
		volumeContainer,
		infoContainer,
	)
}

// setupCallbacks registers player event callbacks
func (ui *FynePlayerUI) setupCallbacks() {
	ui.player.SetTimeCallback(ui.onTimeUpdate)
	ui.player.SetFrameCallback(ui.onFrameUpdate)
	ui.player.SetStateCallback(ui.onStateUpdate)
}

// setupWindow configures the main window
func (ui *FynePlayerUI) setupWindow() {
	ui.window.SetContent(ui.container)
	ui.window.Resize(fyne.NewSize(600, 200))
	ui.window.SetFixedSize(false)
	ui.window.CenterOnScreen()
}

// Show makes the player UI visible
func (ui *FynePlayerUI) Show() {
	ui.window.Show()
}

// Hide makes the player UI invisible
func (ui *FynePlayerUI) Hide() {
	ui.window.Hide()
}

// Close closes the player and UI
func (ui *FynePlayerUI) Close() {
	ui.player.Close()
	ui.window.Close()
}

// togglePlayPause toggles between play and pause states
func (ui *FynePlayerUI) togglePlayPause() {
	if ui.isPlaying {
		ui.pause()
	} else {
		ui.play()
	}
}

// play starts playback
func (ui *FynePlayerUI) play() {
	if err := ui.player.Play(); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to play: %w", err), ui.window)
		return
	}
	ui.isPlaying = true
	ui.playPauseBtn.SetText("Pause")
}

// pause pauses playback
func (ui *FynePlayerUI) pause() {
	if err := ui.player.Pause(); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to pause: %w", err), ui.window)
		return
	}
	ui.isPlaying = false
	ui.playPauseBtn.SetText("Play")
}

// stop stops playback
func (ui *FynePlayerUI) stop() {
	if err := ui.player.Stop(); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to stop: %w", err), ui.window)
		return
	}
	ui.isPlaying = false
	ui.playPauseBtn.SetText("Play")
	ui.seekSlider.SetValue(0)
	ui.timeLabel.SetText("00:00:00")
}

// toggleFullscreen toggles fullscreen mode
func (ui *FynePlayerUI) toggleFullscreen() {
	// Note: This would need to be implemented per-backend
	// For now, just toggle the window fullscreen state
	ui.window.SetFullScreen(!ui.window.FullScreen())
}

// openFile shows a file picker and loads the selected video
func (ui *FynePlayerUI) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()

		filePath := reader.URI().Path()
		if err := ui.player.Load(filePath, 0); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to load file: %w", err), ui.window)
			return
		}

		// Update duration when file loads
		ui.duration = ui.player.GetDuration()
		ui.durationLabel.SetText(formatDuration(ui.duration))
		ui.seekSlider.Max = float64(ui.duration.Milliseconds())

		// Update video info
		info := ui.player.GetVideoInfo()
		ui.fpsLabel.SetText(fmt.Sprintf("FPS: %.2f", info.FrameRate))

	}, ui.window)
}

// onSeekChanged handles seek slider changes
func (ui *FynePlayerUI) onSeekChanged(value float64) {
	if ui.manualSeek {
		// Convert slider value to time duration
		seekTime := time.Duration(value) * time.Millisecond
		if err := ui.player.SeekToTime(seekTime); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to seek: %w", err), ui.window)
		}
	}
}

// onVolumeChanged handles volume slider changes
func (ui *FynePlayerUI) onVolumeChanged(value float64) {
	if err := ui.player.SetVolume(value); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to set volume: %w", err), ui.window)
		return
	}
}

// onTimeUpdate handles time position updates from the player
func (ui *FynePlayerUI) onTimeUpdate(currentTime time.Duration) {
	ui.currentTime = currentTime
	ui.timeLabel.SetText(formatDuration(currentTime))

	// Update seek slider without triggering manual seek
	ui.manualSeek = false
	ui.seekSlider.SetValue(float64(currentTime.Milliseconds()))
	ui.manualSeek = true
}

// onFrameUpdate handles frame position updates from the player
func (ui *FynePlayerUI) onFrameUpdate(frame int64) {
	ui.frameLabel.SetText(fmt.Sprintf("Frame: %d", frame))
}

// onStateUpdate handles player state changes
func (ui *FynePlayerUI) onStateUpdate(state PlayerState) {
	switch state {
	case StatePlaying:
		ui.isPlaying = true
		ui.playPauseBtn.SetText("Pause")
	case StatePaused:
		ui.isPlaying = false
		ui.playPauseBtn.SetText("Play")
	case StateStopped:
		ui.isPlaying = false
		ui.playPauseBtn.SetText("Play")
		ui.seekSlider.SetValue(0)
		ui.timeLabel.SetText("00:00:00")
	case StateError:
		ui.isPlaying = false
		ui.playPauseBtn.SetText("Play")
		dialog.ShowError(fmt.Errorf("Player error occurred"), ui.window)
	}
}

// formatDuration formats a time.Duration as HH:MM:SS
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// LoadVideoFile loads a specific video file
func (ui *FynePlayerUI) LoadVideoFile(filePath string, offset time.Duration) error {
	if err := ui.player.Load(filePath, offset); err != nil {
		return fmt.Errorf("failed to load video file: %w", err)
	}

	// Update duration when file loads
	ui.duration = ui.player.GetDuration()
	ui.durationLabel.SetText(formatDuration(ui.duration))
	ui.seekSlider.Max = float64(ui.duration.Milliseconds())

	// Update video info
	info := ui.player.GetVideoInfo()
	ui.fpsLabel.SetText(fmt.Sprintf("FPS: %.2f", info.FrameRate))

	return nil
}

// SeekToTime seeks to a specific time
func (ui *FynePlayerUI) SeekToTime(offset time.Duration) error {
	if err := ui.player.SeekToTime(offset); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	return nil
}

// SeekToFrame seeks to a specific frame number
func (ui *FynePlayerUI) SeekToFrame(frame int64) error {
	if err := ui.player.SeekToFrame(frame); err != nil {
		return fmt.Errorf("failed to seek to frame: %w", err)
	}
	return nil
}

// GetCurrentTime returns the current playback time
func (ui *FynePlayerUI) GetCurrentTime() time.Duration {
	return ui.player.GetCurrentTime()
}

// GetCurrentFrame returns the current frame number
func (ui *FynePlayerUI) GetCurrentFrame() int64 {
	return ui.player.GetCurrentFrame()
}

// ExtractFrame extracts a frame at the specified time
func (ui *FynePlayerUI) ExtractFrame(offset time.Duration) (interface{}, error) {
	return ui.player.ExtractFrame(offset)
}

// EnablePreviewMode enables or disables preview mode
func (ui *FynePlayerUI) EnablePreviewMode(enabled bool) {
	ui.player.EnablePreviewMode(enabled)
}

// IsPreviewMode returns whether preview mode is enabled
func (ui *FynePlayerUI) IsPreviewMode() bool {
	return ui.player.IsPreviewMode()
}
