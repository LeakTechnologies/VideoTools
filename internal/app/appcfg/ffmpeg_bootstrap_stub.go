//go:build !native_media

package appcfg

func AddFFmpegDllsToPath() error { return nil }

// ValidateFFmpegDLLs is a no-op without the native media engine.
func ValidateFFmpegDLLs() error { return nil }

// DiagnoseDLLSetup reports that DLL diagnostics need the native media build.
func DiagnoseDLLSetup() string {
	return "native media engine not compiled in (build without native_media tag); no FFmpeg diagnostics available"
}
