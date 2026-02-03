# VideoTools Build Scripts

This directory contains scripts for building and managing VideoTools on different platforms.

## Recommended Workflow

For development on any platform:

```bash
./scripts/install.sh
./scripts/build.sh
./scripts/run.sh
```

Use `./scripts/install.sh` whenever you add new dependencies or need to reinstall.

## Layout

- Entry points live in `scripts/`.
- Support scripts live in `scripts/_internal/`.
- Optional tools and legacy helpers live in `scripts/tools/` and `scripts/legacy/`.

## Linux

### Install Dependencies

Automatically installs all required dependencies for your Linux distribution:

```bash
./scripts/_internal/install-deps-linux.sh
```

**Supported distributions:**
- Fedora / RHEL / CentOS
- Ubuntu / Debian / Pop!_OS / Linux Mint
- Arch Linux / Manjaro / EndeavourOS
- openSUSE / SLES

**Installs:**
- Go 1.21+
- GCC compiler
- OpenGL development libraries
- X11 development libraries
- ALSA audio libraries
- ffmpeg

### Build VideoTools

```bash
./scripts/build.sh
```

**Features:**
- Automatic dependency verification
- Clean build option
- Progress indicators
- Error handling

### Run VideoTools

```bash
./scripts/run.sh
```

Runs VideoTools with proper library paths configured.

### Shell Alias

```bash
source ./scripts/alias.sh
```

Adds a `VideoTools` command to your current shell session.

## Windows

### Install Dependencies

Run in PowerShell as Administrator:

```powershell
.\scripts\_internal\install-deps-windows.ps1
```

**Options:**
- `-SkipFFmpeg` - Skip ffmpeg installation (if you already have it)
- `-SkipGStreamer` - Skip GStreamer installation (not recommended)
- `-InstallBuildTools` - Install Go + MSYS2 UCRT64 toolchain
- `-SkipBuildTools` - Skip Go + MSYS2 UCRT64 toolchain
- `-InstallPython` - Install Python + pip
- `-SkipPython` - Skip Python + pip
- `-GStreamerRuntimeMsi` - Use local GStreamer runtime MSI
- `-GStreamerDevelMsi` - Use local GStreamer development MSI

**Installs:**
- FFmpeg (portable, user-level)
- GStreamer (MSI, required for playback)
- Optional: Go + MSYS2 UCRT64 toolchain (repo-local `Tools\msys64`, auto-installed when missing; Scoop GCC is ignored)
- Optional: Python + pip
- Optional: DVD authoring tools (DVDStyler portable)

### Build VideoTools

Run in PowerShell:

```powershell
.\scripts\build.ps1
```

**Options:**
- `-Clean` - Clean build cache before building
- `-SkipTests` - Skip running tests

**Features:**
- Automatic GPU detection (NVIDIA/Intel/AMD)
- Dependency verification
- File size reporting
- Build status indicators

Optional: open a shell with MSYS2 on PATH via `scripts\windows\vt-dev-shell.cmd`.

## Cross-Platform Notes

### CGO Requirements

VideoTools uses [Fyne](https://fyne.io/) for its GUI, which requires CGO (C bindings) for OpenGL support. This means:

1. **C compiler required** (GCC on Linux, MSYS2 UCRT64 toolchain on Windows)
2. **OpenGL libraries required** (system-dependent)
3. **Build time is longer** than pure Go applications

### ffmpeg Requirements

VideoTools requires `ffmpeg` to be available in the system PATH:

- **Linux**: Installed via package manager
- **Windows**: Installed via winget/Chocolatey or manually

The application will auto-detect available hardware encoders:
- NVIDIA: NVENC (h264_nvenc, hevc_nvenc)
- Intel: Quick Sync Video (h264_qsv, hevc_qsv)
- AMD: AMF (h264_amf, hevc_amf)
- VA-API (Linux only)

### GPU Encoding

For best performance with hardware encoding:

**NVIDIA (Recommended in user report):**
- Install latest NVIDIA drivers
- GTX 1060 and newer support NVENC
- Reduces 2-hour encode from 6-9 hours to <1 hour

**Intel:**
- Install Intel Graphics drivers
- 7th gen (Kaby Lake) and newer support Quick Sync
- Built into CPU, no dedicated GPU needed

**AMD:**
- Install latest AMD drivers
- Most modern Radeon GPUs support AMF
- Performance similar to NVENC

## Troubleshooting

### Linux: Missing OpenGL libraries

```bash
# Fedora/RHEL
sudo dnf install mesa-libGL-devel

# Ubuntu/Debian
sudo apt install libgl1-mesa-dev

# Arch
sudo pacman -S mesa
```

### Windows: MSYS2 MinGW-w64 not in PATH

After installing build tools, restart PowerShell or add to PATH manually:

```powershell
$env:Path += ";C:\\msys64\\mingw64\\bin"
```

### Build fails with "cgo: C compiler not found"

**Linux:** Install gcc
**Windows:** Install MSYS2 MinGW-w64 via `scripts/_internal/install-deps-windows.ps1`

### ffmpeg not found

**Linux:**
```bash
sudo dnf install ffmpeg-free  # Fedora
sudo apt install ffmpeg       # Ubuntu
```

**Windows:**
```powershell
.\scripts\_internal\install-deps-windows.ps1
```

### GPU encoding not working

1. Verify GPU drivers are up to date
2. Check ffmpeg encoders:
   ```bash
   ffmpeg -encoders | grep nvenc  # NVIDIA
   ffmpeg -encoders | grep qsv    # Intel
   ffmpeg -encoders | grep amf    # AMD
   ```
3. If encoders not listed, reinstall GPU drivers

## Development

### Quick Build Cycle

Linux:
```bash
./scripts/build.sh && ./scripts/run.sh
```

Windows:
```powershell
.\scripts\build.ps1 && .\VideoTools.exe
```

### Clean Build

Linux:
```bash
./scripts/build.sh  # Includes automatic cleaning
```

Windows:
```powershell
.\scripts\build.ps1 -Clean
```

### Build for Distribution

**Linux:**
```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -o VideoTools .
strip VideoTools  # Further reduce size
```

**Windows:**
```powershell
$env:CGO_ENABLED = "1"
go build -ldflags="-s -w -H windowsgui" -o VideoTools.exe .
```

The `-H windowsgui` flag prevents a console window from appearing on Windows.

## Platform-Specific Notes

### Linux: Wayland vs X11

VideoTools works on both Wayland and X11. The build scripts automatically detect your display server.

### Windows: Antivirus False Positives

Some antivirus software may flag the built executable. This is common with Go applications. You may need to:

1. Add an exception for the build directory
2. Submit the binary to your antivirus vendor for whitelisting


- Handle codesigning requirements

## License

VideoTools build scripts are part of the VideoTools project.
See the main project LICENSE file for details.

