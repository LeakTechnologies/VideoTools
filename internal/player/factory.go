package player

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Factory creates VTPlayer instances based on backend preference
type Factory struct {
	config *Config
}

// NewFactory creates a new player factory with the given configuration
func NewFactory(config *Config) *Factory {
	return &Factory{
		config: config,
	}
}

// CreatePlayer creates a new VTPlayer instance based on the configured backend
func (f *Factory) CreatePlayer() (VTPlayer, error) {
	if f.config == nil {
		f.config = &Config{
			Backend: BackendAuto,
			Volume:  100.0,
		}
	}

	backend := f.config.Backend

	// Auto-select backend if needed
	if backend == BackendAuto {
		backend = f.selectBestBackend()
	}

	switch backend {
	case BackendMPV:
		return f.createMPVPlayer()
	case BackendVLC:
		return f.createVLCPlayer()
	case BackendFFplay:
		return f.createFFplayPlayer()
	default:
		return nil, fmt.Errorf("unsupported backend: %v", backend)
	}
}

// selectBestBackend automatically chooses the best available backend
func (f *Factory) selectBestBackend() BackendType {
	// Try MPV first (best for frame accuracy)
	if f.isMPVAvailable() {
		return BackendMPV
	}

	// Try VLC next (good cross-platform support)
	if f.isVLCAvailable() {
		return BackendVLC
	}

	// Fall back to FFplay (always available with ffmpeg)
	if f.isFFplayAvailable() {
		return BackendFFplay
	}

	// Default to MPV and let it fail with a helpful error
	return BackendMPV
}

// isMPVAvailable checks if MPV is available on the system
func (f *Factory) isMPVAvailable() bool {
	// Check for mpv executable
	_, err := exec.LookPath("mpv")
	if err != nil {
		return false
	}

	// Additional platform-specific checks could be added here
	// For example, checking for libmpv libraries on Linux/Windows

	return true
}

// isVLCAvailable checks if VLC is available on the system
func (f *Factory) isVLCAvailable() bool {
	_, err := exec.LookPath("vlc")
	if err != nil {
		return false
	}

	// Check for libvlc libraries
	// This would be platform-specific
	switch runtime.GOOS {
	case "linux":
		// Check for libvlc.so
		_, err := exec.LookPath("libvlc.so.5")
		if err != nil {
			// Try other common library names
			_, err := exec.LookPath("libvlc.so")
			return err == nil
		}
		return true
	case "windows":
		// Check for VLC installation directory
		_, err := exec.LookPath("libvlc.dll")
		return err == nil
	case "darwin":
		// Check for VLC app or framework
		_, err := exec.LookPath("/Applications/VLC.app/Contents/MacOS/VLC")
		return err == nil
	}

	return false
}

// isFFplayAvailable checks if FFplay is available on the system
func (f *Factory) isFFplayAvailable() bool {
	_, err := exec.LookPath("ffplay")
	return err == nil
}

// createMPVPlayer creates an MPV-based player
func (f *Factory) createMPVPlayer() (VTPlayer, error) {
	// Use the existing MPV controller
	return NewMPVController(f.config)
}

// createVLCPlayer creates a VLC-based player
func (f *Factory) createVLCPlayer() (VTPlayer, error) {
	// Use the existing VLC controller
	return NewVLCController(f.config)
}

// createFFplayPlayer creates an FFplay-based player
func (f *Factory) createFFplayPlayer() (VTPlayer, error) {
	// Wrap the existing FFplay controller to implement VTPlayer interface
	return NewFFplayWrapper(f.config)
}

// GetAvailableBackends returns a list of available backends
func (f *Factory) GetAvailableBackends() []BackendType {
	var backends []BackendType

	if f.isMPVAvailable() {
		backends = append(backends, BackendMPV)
	}
	if f.isVLCAvailable() {
		backends = append(backends, BackendVLC)
	}
	if f.isFFplayAvailable() {
		backends = append(backends, BackendFFplay)
	}

	return backends
}

// SetConfig updates the factory configuration
func (f *Factory) SetConfig(config *Config) {
	f.config = config
}

// GetConfig returns the current factory configuration
func (f *Factory) GetConfig() *Config {
	return f.config
}
