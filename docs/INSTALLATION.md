# VideoTools Installation Guide

Welcome to the VideoTools installation guide. Please select your operating system to view the detailed instructions.

---

## Supported Platforms

### Windows

For Windows 10 and 11, please follow our detailed, step-by-step guide. It covers both automated and manual setup.

- **[View Windows Installation Guide](./INSTALL_WINDOWS.md)**
  - Use `scripts\install.bat` or `scripts\install.ps1` on Windows.
  - The Windows installer defaults to MSI downloads for GStreamer runtime; the development package is only installed when build tools are selected.
  - Use `-GStreamerVersion` to override the default MSI version when needed.
  - Optional DVD authoring tools default to the portable ZIP and fall back to winget, then MSYS2, if downloads fail.
  - Windows build scripts will prompt for elevation when needed.
  - Set `VT_MIRROR_BASE` to prefer downloads from `mirror.leaktechnologies.dev` once available.
- **[Windows Packaging Roadmap](./WINDOWS_PACKAGING.md)**

### Linux & macOS

For Linux (Ubuntu, Fedora, Arch, etc.), macOS, and Windows Subsystem for Linux (WSL), the installation is handled by a single, powerful script.

- **[View Linux, macOS, & WSL Installation Guide](./INSTALL_LINUX.md)**

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
