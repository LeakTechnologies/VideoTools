# VideoTools Installation Guide

Welcome to the VideoTools installation guide. Please select your operating system to view the detailed instructions.

---

## Supported Platforms

### Windows

For Windows 10 and 11, please follow our detailed, step-by-step guide. It covers both automated and manual setup.

- **[View Windows Installation Guide](./INSTALL_WINDOWS.md)**
  - Use `scripts\windows\install.bat` or `scripts\windows\install.ps1` on Windows.
  - The Windows installer defaults to MSI downloads for GStreamer; use `-PreferWinget` to try winget first.
  - Use `-GStreamerVersion` to override the default MSI version when needed.
  - Optional DVD authoring tools default to the Leak Technologies mirror installer and can use winget with `-PreferWinget`.
  - SourceForge DVDStyler mirrors are opt-in on Windows via `VT_DVDSTYLER_ALLOW_SOURCEFORGE=1`.
  - Use `VT_MIRROR_TOKEN` or `VT_MIRROR_BASIC` if Leak Technologies mirrors are private.
  - `scripts\windows\build.bat` delegates to PowerShell for elevation and build output.
  - If GCC fails the build preflight, use MSYS2 UCRT64 with `mingw-w64-ucrt-x86_64-toolchain`.
  - The Windows installer provisions a repo-local MSYS2 toolchain at `Tools\msys64` (UCRT64). Override with `VT_MSYS2_ROOT` or `VT_MSYS2_FLAVOR`.
  - The Windows installer can reinstall the MSYS2 toolchain if GCC fails a test compile and will auto-install build tools when missing (Scoop GCC is ignored).
  - The Windows build script attempts to repair missing MSYS2 GCC packages automatically when possible.
  - On Windows first run, if FFmpeg is missing, the app can install a user-local FFmpeg bundle to `%LOCALAPPDATA%\VideoTools\bin` from inside the UI.
  - The Settings > Dependencies tab now prioritizes core dependencies and provides install actions for FFmpeg across platforms (Windows app-local install, Linux package manager commands).
  - Windows builds embed the VT icon when the MSYS2 windres tool is available.
  - Logs default to `~/Videos/VideoTools/logs` and can be changed in Settings.
  - Forgejo dev builds produce a Linux AppImage with the VT icon embedded.
  - Forgejo Windows packaging workflows emit `GITHUB_OUTPUT` as UTF-8 (no BOM) to avoid host-runner post-step failures.
  - Forgejo Windows packages are built as GUI-only binaries (no console window) and include only `VideoTools.exe` plus `README.md` in the zip at `dist/windows/` (artifacts upload only the zip).
  - Forgejo dev release workflow now builds Linux and Windows artifacts in the same run before publishing (replaces standalone Windows/Linux packaging workflows).
  - Forgejo dev releases upload only Linux AppImage/zip and Windows zip (no build metadata files).
  - Forgejo dev releases delete existing assets with the same name before uploading new ones.
  - Forgejo dev releases now purge existing release assets before uploading to prevent duplicates.
  - Forgejo dev releases fail if asset purge or upload calls are unauthorized.
  - Forgejo dev release workflow uses concurrency to prevent overlapping uploads.
  - Forgejo dev releases use a nightly build release note body for clarity.
  - Forgejo mirrors the repository to Codeberg using built-in push mirror settings.
  - Windows builds pause on success or failure so you can review output before the window closes.
  - On Windows VMs with basic/virtual display adapters, VideoTools will show a preflight warning and exit; enable 3D acceleration or install GPU drivers.
  - Whisper model downloads use the Leak Technologies mirror by default and are optional on Windows.
  - Windows build scripts will prompt for elevation when needed.
  - The Windows installer waits for a keypress before closing on success.
  - On failure, the Windows installer pauses so you can copy the error before closing.
  - The Windows installer creates Start Menu shortcuts under "VideoTools" (Build shortcut plus app shortcut after build).
- **[Windows Packaging Roadmap](./WINDOWS_PACKAGING.md)**

### Linux

For Linux (Ubuntu, Fedora, Arch, etc.) and Windows Subsystem for Linux (WSL), the installation is handled by a single, powerful script.

- **[View Linux & WSL Installation Guide](./INSTALL_LINUX.md)**
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
- **UI Layout:** The main menu adapts to window width; resize the window to change tile density.
- **Scrolling:** Settings and long panels use adaptive scroll speed for smoother navigation at different window sizes.
- **Settings Tabs:** Each Settings tab scrolls independently so the tab header stays visible.
- **Master Settings:** Preferences include global hardware acceleration detection and module visibility toggles.
- **Subtitles:** The Subtitles module can extract embedded tracks losslessly or OCR image-based tracks to SRT/ASS (requires Tesseract). OCR language codes: `eng`, `fra`, `iku`.

---

## Bundled vs Standard Downloads

- **Standard package:** App only (dependencies installed via Settings or scripts).
- **Bundled package:** App + FFmpeg + Tesseract (with `eng`/`fra` required, `iku` optional) + GStreamer + whisper.cpp small model (`ggml-small.bin`). Use the bundled launcher to run with bundled dependencies:
  - Linux: `run-bundled.sh`
  - Windows: `run-bundled.ps1` or `run-bundled.bat`

---

## Development

If you are a developer looking to contribute to the project, please see the [Build and Run Guide](./BUILD_AND_RUN.md) for instructions on setting up a development environment.
Build scripts write packaged artifacts to `dist/<os>/<channel>/` and emit a `build.json` file alongside each zip.
