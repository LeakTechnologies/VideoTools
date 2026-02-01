# VideoTools Installation Guide

Welcome to the VideoTools installation guide. Please select your operating system to view the detailed instructions.

---

## Supported Platforms

### Windows

For Windows 10 and 11, please follow our detailed, step-by-step guide. It covers both automated and manual setup.

- **[View Windows Installation Guide](./INSTALL_WINDOWS.md)**
  - Use `scripts\install.bat` or `scripts\install.ps1` on Windows.
  - The Windows installer defaults to MSI downloads for GStreamer; use `-PreferWinget` to try winget first.
  - Use `-GStreamerVersion` to override the default MSI version when needed.
  - Optional DVD authoring tools default to the Leak Technologies mirror installer and can use winget with `-PreferWinget`.
  - SourceForge DVDStyler mirrors are opt-in on Windows via `VT_DVDSTYLER_ALLOW_SOURCEFORGE=1`.
  - Use `VT_MIRROR_TOKEN` or `VT_MIRROR_BASIC` if Leak Technologies mirrors are private.
  - `scripts\build.bat` delegates to PowerShell for elevation and build output.
  - If GCC fails the build preflight, use MSYS2 with `mingw-w64-x86_64-gcc`.
  - The Windows installer can reinstall MSYS2 MinGW-w64 if GCC fails a test compile and will auto-install build tools when missing (Scoop GCC is ignored).
  - Whisper model downloads use the Leak Technologies mirror by default and are optional on Windows.
  - Windows build scripts will prompt for elevation when needed.
  - The Windows installer waits for a keypress before closing on success.
  - On failure, the Windows installer pauses so you can copy the error before closing.
- **[Windows Packaging Roadmap](./WINDOWS_PACKAGING.md)**

### Linux & macOS

For Linux (Ubuntu, Fedora, Arch, etc.), macOS, and Windows Subsystem for Linux (WSL), the installation is handled by a single, powerful script.

- **[View Linux, macOS, & WSL Installation Guide](./INSTALL_LINUX.md)**
  - Whisper model downloads use the Leak Technologies mirror with upstream fallback.

---

## General Requirements

Before you begin, ensure your system meets these basic requirements:

- **Go:** Version 1.21 or later is required to build the application; the installer will auto-install it when missing.
- **FFmpeg:** Required for all video and audio processing. Our platform-specific guides cover how to install this.
- **Python + pip:** Required for AI tooling and optional modules; the installers will set this up automatically where possible.
- **Disk Space:** At least 2 GB of free disk space for the application and its dependencies.
- **Internet Connection:** Required for downloading dependencies during the build process.
- **Player Controls:** Fullscreen toggle is available in the Player module after install.

---

## Development

If you are a developer looking to contribute to the project, please see the [Build and Run Guide](./BUILD_AND_RUN.md) for instructions on setting up a development environment.
Build scripts write packaged artifacts to `dist/<os>/<channel>/` and emit a `build.json` file alongside each zip.





