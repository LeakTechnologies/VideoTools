# VideoTools Installation Guide

Dev97 player candidate: startup and pause/resume preserve buffered content and add no runtime dependencies. Windows software playback was witnessed and logged; residual faster-playback jitter and Linux runtime acceptance remain open. This candidate has not been merged or published.

Player lifecycle candidate (dev95, unlanded): source-owned cancellation and worker cleanup add no runtime dependencies. Automated Windows software-playback stress completed, but visual review is pending; Linux runtime review is also pending. No candidate installation/release is published.

Dev94 player note: native frame and timestamp waits now respond to cancellation/engine retirement. No new runtime dependencies. Basic software playback and close were checked on Windows; minor 2x-segment jitter remains under investigation. Linux runtime acceptance remains outstanding.

Dev93 restores Rip Content Browser title labels and selection controls. No new runtime dependencies. After updating, load a disc and check card visibility and selection while scrolling.


Select your operating system for detailed instructions.

---

## Supported Platforms

| Platform | Guide |
|----------|-------|
| Windows 10 / 11 | [INSTALL_WINDOWS.md](./INSTALL_WINDOWS.md) |
| Linux (Ubuntu, Fedora, Arch, etc.) | [INSTALL_LINUX.md](./INSTALL_LINUX.md) |

macOS is not a supported platform.

---

## System Requirements

| Requirement | Details |
|-------------|---------|
| **Go** | 1.21 or later (installers auto-install when missing) |
| **FFmpeg** | Required for all video and audio processing |
| **GPU** | OpenGL-capable display adapter required; software rendering is not supported |
| **Disk space** | 2 GB minimum for the application and dependencies |
| **Internet** | Required during initial dependency installation |

Optional dependencies (installed on demand via Settings > Dependencies):

- **Tesseract** — OCR for image-based subtitle extraction
- **realesrgan-ncnn-vulkan** — AI upscaling (requires Vulkan-capable GPU)
- **Whisper** — Speech-to-text for subtitle generation

---

## Windows

Run one of the following from PowerShell or CMD:

```powershell
# PowerShell
scripts\windows\install.ps1

# CMD
scripts\windows\install.bat
```

The installer provisions a repo-local MSYS2 UCRT64 toolchain, builds the binary, and
creates Start Menu shortcuts. On first run, if FFmpeg is not found, the app offers to
install a user-local FFmpeg bundle to `%LOCALAPPDATA%\VideoTools\bin`.

See [INSTALL_WINDOWS.md](./INSTALL_WINDOWS.md) for detailed options.

---

## Linux

```bash
bash scripts/linux/install.sh
```

Then reload your shell:

```bash
source ~/.bashrc   # or ~/.zshrc / ~/.config/fish/config.fish
```

See [INSTALL_LINUX.md](./INSTALL_LINUX.md) for detailed options and WSL notes.

---

## Downloads

Pre-built packages are available on the [Forgejo releases page](https://git.leaktechnologies.dev/leak_technologies/VideoTools/releases).

- **Linux:** `.zip` (binary) and `.AppImage`
- **Windows:** `.zip` containing `VideoTools.exe`

Unzip and run. Use Settings > Dependencies to install any missing runtime tools.

---

## Development Builds

For contributors and developers, see [BUILD_AND_RUN.md](./BUILD_AND_RUN.md).
