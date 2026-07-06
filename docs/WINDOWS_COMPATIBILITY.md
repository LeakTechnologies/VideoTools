# Windows Compatibility Implementation Plan

## Current Status

VideoTools is built with Go + Fyne, which are inherently cross-platform. However, several areas need attention for full Windows support.

---

## ✅ Already Cross-Platform

The codebase already uses good practices:
- `filepath.Join()` for path construction
- `os.TempDir()` for temporary files
- `filepath.Separator` awareness
- Fyne GUI framework (cross-platform)

---

## 🔧 Required Changes

### 1. FFmpeg Detection and Bundling

**Current**: Assumes `ffmpeg` is in PATH
**Windows Issue**: FFmpeg not typically installed system-wide

**Solution**:
```go
func findFFmpeg() string {
    // Priority order:
    // 1. Bundled ffmpeg.exe in application directory
    // 2. FFMPEG_PATH environment variable
    // 3. System PATH
    // 4. Common install locations (C:\Program Files\ffmpeg\bin\)

    if runtime.GOOS == "windows" {
        // Check application directory first
        exePath, _ := os.Executable()
        bundledFFmpeg := filepath.Join(filepath.Dir(exePath), "ffmpeg.exe")
        if _, err := os.Stat(bundledFFmpeg); err == nil {
            return bundledFFmpeg
        }
    }

    // Check PATH
    path, err := exec.LookPath("ffmpeg")
    if err == nil {
        return path
    }

    return "ffmpeg" // fallback
}
```

### 2. Process Management

**Current**: Uses `context.WithCancel()` for process termination
**Windows Issue**: Windows doesn't support SIGTERM signals

**Solution**:
```go
func killFFmpegProcess(cmd *exec.Cmd) error {
    if runtime.GOOS == "windows" {
        // Windows: use Kill() directly
        return cmd.Process.Kill()
    } else {
        // Unix: try graceful shutdown first
        cmd.Process.Signal(os.Interrupt)
        time.Sleep(1 * time.Second)
        return cmd.Process.Kill()
    }
}
```

### 3. File Path Handling

**Current**: Good use of `filepath` package
**Potential Issues**: UNC paths, drive letters

**Enhancements**:
```go
// Validate Windows-specific paths
func validateWindowsPath(path string) error {
    if runtime.GOOS != "windows" {
        return nil
    }

    // Check for drive letter
    if len(path) >= 2 && path[1] == ':' {
        drive := strings.ToUpper(string(path[0]))
        if drive < "A" || drive > "Z" {
            return fmt.Errorf("invalid drive letter: %s", drive)
        }
    }

    // Check for UNC path
    if strings.HasPrefix(path, `\\`) {
        // Valid UNC path
        return nil
    }

    return nil
}
```

### 4. Hardware Acceleration Detection

**Current**: Linux-focused (VAAPI detection)
**Windows Needs**: NVENC, QSV, AMF detection

**Implementation**:
```go
func detectWindowsGPU() []string {
    var encoders []string

    // Test for NVENC (NVIDIA)
    if testFFmpegEncoder("h264_nvenc") {
        encoders = append(encoders, "nvenc")
    }

    // Test for QSV (Intel)
    if testFFmpegEncoder("h264_qsv") {
        encoders = append(encoders, "qsv")
    }

    // Test for AMF (AMD)
    if testFFmpegEncoder("h264_amf") {
        encoders = append(encoders, "amf")
    }

    return encoders
}

func testFFmpegEncoder(encoder string) bool {
    cmd := exec.Command(findFFmpeg(), "-encoders")
    output, err := cmd.Output()
    if err != nil {
        return false
    }
    return strings.Contains(string(output), encoder)
}
```

### 5. Temporary File Cleanup

**Current**: Uses `os.TempDir()`
**Windows Enhancement**: Better cleanup on Windows

```go
func createTempVideoDir() (string, error) {
    baseDir := os.TempDir()
    if runtime.GOOS == "windows" {
        // Use AppData\Local\Temp\VideoTools on Windows
        appData := os.Getenv("LOCALAPPDATA")
        if appData != "" {
            baseDir = filepath.Join(appData, "Temp")
        }
    }

    dir := filepath.Join(baseDir, fmt.Sprintf("videotools-%d", time.Now().Unix()))
    return dir, os.MkdirAll(dir, 0755)
}
```

### 6. File Associations and Context Menu

**Windows Registry Integration** (optional for later):
```
HKEY_CLASSES_ROOT\*\shell\VideoTools
    @="Open with VideoTools"
    Icon="C:\Program Files\VideoTools\VideoTools.exe,0"

HKEY_CLASSES_ROOT\*\shell\VideoTools\command
    @="C:\Program Files\VideoTools\VideoTools.exe \"%1\""
```

---

## 🏗️ Build System Changes

### Cross-Compilation from Linux

```bash
# Install MinGW-w64
sudo apt-get install gcc-mingw-w64

# Set environment for Windows build
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc

# Build for Windows
go build -o VideoTools.exe -ldflags="-H windowsgui"
```

### Build Script (`build-windows.sh`)

```bash
#!/bin/bash
set -e

echo "Building VideoTools for Windows..."

# Set Windows build environment
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc

# Build flags
LDFLAGS="-H windowsgui -s -w"

# Build
go build -o VideoTools.exe -ldflags="$LDFLAGS"

# Bundle ffmpeg (download if not present)
if [ ! -f "ffmpeg.exe" ]; then
    echo "ffmpeg.exe missing — copy the static ffmpeg.exe/ffprobe.exe from a VideoTools release zip"
    exit 1
fi

# Create distribution package
mkdir -p dist/windows
cp VideoTools.exe dist/windows/
cp ffmpeg.exe dist/windows/
cp README.md dist/windows/
cp LICENSE dist/windows/

echo "Windows build complete: dist/windows/"
```

### Create Windows Installer (NSIS Script)

```nsis
; VideoTools Installer Script

!define APP_NAME "VideoTools"
!define VERSION "0.1.0"
!define COMPANY "Leak Technologies"

Name "${APP_NAME}"
OutFile "VideoTools-Setup.exe"
InstallDir "$PROGRAMFILES64\${APP_NAME}"

Section "Install"
    SetOutPath $INSTDIR
    File "VideoTools.exe"
    File "ffmpeg.exe"
    File "README.md"
    File "LICENSE"

    ; Create shortcuts
    CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\VideoTools.exe"
    CreateDirectory "$SMPROGRAMS\${APP_NAME}"
    CreateShortcut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\VideoTools.exe"
    CreateShortcut "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk" "$INSTDIR\Uninstall.exe"

    ; Write uninstaller
    WriteUninstaller "$INSTDIR\Uninstall.exe"

    ; Add to Programs and Features
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "DisplayName" "${APP_NAME}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" "UninstallString" "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Uninstall"
    Delete "$INSTDIR\VideoTools.exe"
    Delete "$INSTDIR\ffmpeg.exe"
    Delete "$INSTDIR\README.md"
    Delete "$INSTDIR\LICENSE"
    Delete "$INSTDIR\Uninstall.exe"
    Delete "$DESKTOP\${APP_NAME}.lnk"
    RMDir /r "$SMPROGRAMS\${APP_NAME}"
    RMDir "$INSTDIR"

    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
SectionEnd
```

---

## 📝 Code Changes Needed

### New File: `platform.go`

```go
package main

import (
    "os/exec"
    "path/filepath"
    "runtime"
)

// PlatformConfig holds platform-specific configuration
type PlatformConfig struct {
    FFmpegPath string
    TempDir    string
    Encoders   []string
}

// DetectPlatform detects the current platform and returns configuration
func DetectPlatform() *PlatformConfig {
    cfg := &PlatformConfig{}

    cfg.FFmpegPath = findFFmpeg()
    cfg.TempDir = getTempDir()
    cfg.Encoders = detectEncoders()

    return cfg
}

// findFFmpeg locates the ffmpeg executable
func findFFmpeg() string {
    exeName := "ffmpeg"
    if runtime.GOOS == "windows" {
        exeName = "ffmpeg.exe"

        // Check bundled location first
        exePath, _ := os.Executable()
        bundled := filepath.Join(filepath.Dir(exePath), exeName)
        if _, err := os.Stat(bundled); err == nil {
            return bundled
        }
    }

    // Check PATH
    if path, err := exec.LookPath(exeName); err == nil {
        return path
    }

    return exeName
}

// getTempDir returns platform-appropriate temp directory
func getTempDir() string {
    base := os.TempDir()

    if runtime.GOOS == "windows" {
        appData := os.Getenv("LOCALAPPDATA")
        if appData != "" {
            return filepath.Join(appData, "Temp", "VideoTools")
        }
    }

    return filepath.Join(base, "videotools")
}

// detectEncoders detects available hardware encoders
func detectEncoders() []string {
    var encoders []string

    // Test common encoders
    testEncoders := []string{"h264_nvenc", "hevc_nvenc", "h264_qsv", "h264_amf"}

    for _, enc := range testEncoders {
        if testEncoder(enc) {
            encoders = append(encoders, enc)
        }
    }

    return encoders
}

func testEncoder(name string) bool {
    cmd := exec.Command(findFFmpeg(), "-hide_banner", "-encoders")
    output, err := cmd.Output()
    if err != nil {
        return false
    }
    return strings.Contains(string(output), name)
}
```

### Modify `main.go`

Add platform initialization:
```go
var platformConfig *PlatformConfig

func main() {
    // Detect platform early
    platformConfig = DetectPlatform()
    logging.Debug(logging.CatSystem, "Platform: %s, FFmpeg: %s", runtime.GOOS, platformConfig.FFmpegPath)

    // ... rest of main
}
```

Update FFmpeg command construction:
```go
func (s *appState) startConvert(...) {
    // Use platform-specific ffmpeg path
    cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)

    // ... rest of function
}
```

---

## 🧪 Testing Plan

### Phase 1: Build Testing
- [ ] Cross-compile from Linux successfully
- [ ] Test executable runs on Windows 10
- [ ] Test executable runs on Windows 11
- [ ] Verify no missing DLL errors

### Phase 2: Functionality Testing
- [ ] File dialogs work correctly
- [ ] Drag-and-drop from Windows Explorer
- [ ] Video playback works
- [ ] Conversion completes successfully
- [ ] Queue management works
- [ ] Progress reporting accurate

### Phase 3: Hardware Testing
- [ ] Test with NVIDIA GPU (NVENC)
- [ ] Test with Intel integrated graphics (QSV)
- [ ] Test with AMD GPU (AMF)
- [ ] Test on system with no GPU

### Phase 4: Path Testing
- [ ] Paths with spaces
- [ ] Paths with special characters
- [ ] UNC network paths
- [ ] Different drive letters (C:, D:, etc.)
- [ ] Long paths (>260 characters)

### Phase 5: Edge Cases
- [ ] Multiple monitor setups
- [ ] High DPI displays
- [ ] Low memory systems
- [ ] Antivirus interference
- [ ] Windows Defender SmartScreen

---

## 📦 Distribution

### Portable Version
- Single folder with VideoTools.exe + ffmpeg.exe
- No installation required
- Can run from USB stick

### Installer Version
- NSIS or WiX installer
- System-wide installation
- Start menu shortcuts
- File associations (optional)
- Auto-update capability

### Windows Store (Future)
- MSIX package
- Automatic updates
- Sandboxed environment
- Microsoft Store visibility

---

## 🐛 Known Windows-Specific Issues to Address

1. **Console Window**: Use `-ldflags="-H windowsgui"` to hide console
2. **File Locking**: Windows locks files more aggressively - ensure proper file handle cleanup
3. **Path Length Limits**: Windows has 260 character path limit (use extended paths if needed)
4. **Antivirus False Positives**: May need code signing certificate
5. **DPI Scaling**: Fyne should handle this, but test on high-DPI displays

---

## 📋 Implementation Checklist

### Immediate (dev14)
- [ ] Create `platform.go` with FFmpeg detection
- [ ] Update all `exec.Command("ffmpeg")` to use platform config
- [ ] Add Windows encoder detection (NVENC, QSV, AMF)
- [ ] Create `build-windows.sh` script
- [ ] Test cross-compilation

### Short-term (dev15)
- [ ] Bundle ffmpeg.exe with Windows builds
- [ ] Create Windows installer (NSIS)
- [ ] Add file association registration
- [ ] Test on Windows 10/11

### Medium-term (dev16+)
- [ ] Code signing certificate
- [ ] Auto-update mechanism
- [ ] Windows Store submission
- [ ] Performance optimization for Windows

---

## 🔗 Resources

- **MinGW-w64**: https://www.mingw-w64.org/
- **Fyne Windows Guide**: https://developer.fyne.io/started/windows
- **Go Cross-Compilation**: https://go.dev/doc/install/source#environment
- **NSIS Documentation**: https://nsis.sourceforge.io/Docs/

---

**Last Updated**: 2025-12-04
**Target Version**: v0.1.0-dev14
