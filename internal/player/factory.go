package player

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Deprecated: MPV backend is deprecated and will be removed in a future release.
// Use the native media engine (FFmpeg-based) instead.
const BackendMPVDeprecated = true

// Deprecated: VLC backend is deprecated and will be removed in a future release.
// Use the native media engine (FFmpeg-based) or FFplay instead.
const BackendVLCDeprecated = true

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
	case BackendFFplay:
		return f.createFFplayPlayer()
	case BackendNative:
		return f.createNativePlayer()
	case BackendMPV:
		return nil, fmt.Errorf("MPV backend is deprecated; use BackendNative or BackendFFplay")
	case BackendVLC:
		return nil, fmt.Errorf("VLC backend is deprecated; use BackendNative or BackendFFplay")
	default:
		return nil, fmt.Errorf("unsupported backend: %v", backend)
	}
}

// selectBestBackend automatically chooses the best available backend
func (f *Factory) selectBestBackend() BackendType {
	// Native media engine is preferred when available
	// For now, fall back to FFplay as the default
	if f.isFFplayAvailable() {
		return BackendFFplay
	}

	return BackendFFplay
}

// isFFplayAvailable checks if FFplay is available on the system
func (f *Factory) isFFplayAvailable() bool {
	_, err := exec.LookPath("ffplay")
	return err == nil
}

// createFFplayPlayer creates an FFplay-based player
func (f *Factory) createFFplayPlayer() (VTPlayer, error) {
	return NewFFplayWrapper(f.config)
}

// createNativePlayer creates the native media engine player
// This is a placeholder - the native engine is accessed directly via internal/media package
func (f *Factory) createNativePlayer() (VTPlayer, error) {
	return nil, fmt.Errorf("native player should be accessed via internal/media.Engine")
}

// GetAvailableBackends returns a list of available backends
func (f *Factory) GetAvailableBackends() []BackendType {
	var backends []BackendType

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

// IsNativeEngineAvailable returns true if the native media engine is available
func IsNativeEngineAvailable() bool {
	return true
}

// BackendType represents the player backend being used
// Deprecated: BackendMPV and BackendVLC are deprecated
// Use BackendNative (FFmpeg-based) or BackendFFplay instead

// GetPlatformFFplayPaths returns platform-specific FFplay executable names
func GetPlatformFFplayPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"ffplay.exe", "ffplay"}
	case "darwin":
		return []string{"ffplay"}
	default:
		return []string{"ffplay"}
	}
}
