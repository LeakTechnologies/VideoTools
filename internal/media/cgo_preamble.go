//go:build native_media

package media

/*
// CGo build directives for FFmpeg.
//
// Linux/macOS: resolved via pkg-config (uses .pc files from the build prefix).
//
// Windows (CI): overridden by CGO_CFLAGS / CGO_LDFLAGS environment variables
// set in ci-build.ps1 via pkg-config --libs --static, so the absolute path
// below is a local-dev fallback only. See scripts/windows/ci-build.ps1.
//
// Windows (local dev): expects FFmpeg headers/libs at C:/ffmpeg/ (the default
// install prefix used by build.ps1). Override via CGO_CFLAGS / CGO_LDFLAGS.
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil libswresample libavfilter
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304
*/
import "C"