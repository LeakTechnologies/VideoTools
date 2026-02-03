# VideoTools Installation Guide for Windows

This guide provides step-by-step instructions for installing VideoTools on Windows 10 and 11.

---

## Method 1: Automated Installation (Recommended)

This method uses a script to automatically download and configure all necessary dependencies.

> **Note:** This dev installer is temporary. The production installer plan is MSIX + WinGet. See `docs/WINDOWS_PACKAGING.md`.

### Step 1: Download the Project

If you haven't already, download the project files as a ZIP and extract them to a folder on your computer (e.g., `C:\Users\YourUser\Documents\VideoTools`).

### Step 2: Run the Dependency Installer

1.  Open the project folder in File Explorer.
2.  Choose one of these entrypoints:
    - Run `.\scripts\install.bat` (recommended).
    - Run `.\scripts\install.ps1` from PowerShell.
3.  A terminal window will open and run the PowerShell installer. It installs core dependencies with minimal changes:
    *   Go (build toolchain)
    *   MSYS2 UCRT64 toolchain (gcc for CGO builds)
    *   FFmpeg (video processing)
    *   GStreamer (player backend)
    *   Python (pip for optional tooling)
    *   Note: GStreamer installs via MSI and requires an Administrator PowerShell window.
    *   Note: Build tools are optional; the installer will auto-install Go/MSYS2 when missing unless -SkipBuildTools is set.
    *   Note: The installer will prompt for elevation when required and keep the admin window open for logs.
    *   Note: On success, the installer waits for a keypress before closing the window.

Support scripts are stored in `scripts/_internal/` and should not be run directly unless noted.

Optional flags:
- `-SkipFFmpeg` or `-SkipGStreamer` to skip those dependencies.
- `-InstallBuildTools` to force install Go + MSYS2 UCRT64 toolchain without a prompt.
- `-SkipBuildTools` to skip Go + MSYS2 UCRT64 toolchain.
- `-InstallPython` to install Python + pip for AI tooling.
- `-SkipPython` to skip Python + pip.
- `-SkipDvdStyler` to skip DVD authoring tools.
- `-DvdStylerUrl`, `-DvdStylerExeUrl`, or `-DvdStylerZip` to force a custom DVDStyler download.
- `-DvdStylerExeArgs` to override the DVDStyler installer arguments (default: `/S`).
- `-InstallWhisper` to install the Whisper small model for subtitles.
- `-SkipWhisper` to skip the Whisper model.
- `-WhisperModelUrl` to override the Whisper model download URL.
- `-WhisperModelPath` to set a custom model path.
Environment overrides:
- `VT_MIRROR_TOKEN` to authenticate against private Leak Technologies mirrors.
- `VT_MIRROR_BASIC` to authenticate against private mirrors using `user:pass`.
The installer uses curl with a progress bar when available for large downloads (DVDStyler, Whisper).
- `-GStreamerRuntimeMsi` and `-GStreamerDevelMsi` to install from local MSI files.
- `-GStreamerVersion` to override the default MSI version (default: 1.26.10).
- `-PreferWinget` to prefer winget installs when available.

The installer will prompt before optional modules (Python + pip, build tools, DVD authoring tools, Whisper model) when they are missing. GStreamer is required and will be installed automatically (MSI) if not already present.
Build tools are provisioned via a repo-local MSYS2 UCRT64 toolchain at `Tools\msys64`. Override with `VT_MSYS2_ROOT` or `VT_MSYS2_FLAVOR`.
If you opt into build tools and GCC fails a test compile, the installer will offer to reinstall the MSYS2 toolchain. GCC from Scoop is ignored.
The build script attempts to repair missing MSYS2 GCC packages automatically when possible.

The installer defaults to MSI downloads for GStreamer. If MSI downloads fail, grab the MSI files from https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/ and re-run the installer with `-GStreamerRuntimeMsi` and `-GStreamerDevelMsi`. Use `-PreferWinget` if you want the installer to try winget first.

DVDStyler defaults to the Leak Technologies mirror installer (`DVDStyler-3.2.1-win64.exe`). If it fails and `-PreferWinget` is set, the installer tries winget before skipping the optional module.
SourceForge mirrors are only used when `VT_DVDSTYLER_ALLOW_SOURCEFORGE=1` is set in the environment.
Whisper uses the Leak Technologies mirror by default and can be overridden with `-WhisperModelUrl`.

> **Note:** If Windows Defender SmartScreen appears, click "More info" and then "Run anyway". This is expected as the application is not yet digitally signed.

### Step 3: Build and Run VideoTools

After the installer runs, a Start Menu folder named "VideoTools" is created. It includes a "Build VideoTools" shortcut and, once you build, a "VideoTools" app shortcut. The build script refreshes the app shortcut after a successful build.

1.  Build the app:
    - `.\scripts\build.bat` (delegates to `build.ps1` and handles elevation)
    - If the build reports a GCC toolchain failure, install MSYS2 and `mingw-w64-ucrt-x86_64-toolchain`.
2.  Run the executable:
    - `.\VideoTools.exe`
    - The Player module includes a fullscreen toggle in the playback controls.

> **Note:** `scripts\build.bat` and `scripts\build.ps1` will prompt for elevation to ensure build tools are available.
The build script pauses for a keypress on success or failure so you can review the output before the window closes.
If you are running inside a VM and see an OpenGL preflight warning, enable 3D acceleration or install GPU drivers before retrying.

Optional: use `scripts\windows\vt-dev-shell.cmd` to open a shell with the MSYS2 toolchain on PATH.

If you want a portable FFmpeg bundle placed next to the Windows executable, run:
- `.\scripts\_internal\setup-windows.ps1 -Portable`

For a system-wide FFmpeg install (PATH), use:
- `.\scripts\_internal\setup-windows.ps1 -System`

> **Note:** On Windows, use `scripts\install.ps1` or `scripts\install.bat`. Running `scripts\install.sh` from PowerShell will open Git Bash.

---

## Build Artifacts

- Build packages are written to `dist/windows/<channel>/`.
- Each build writes a `build.json` file alongside the zip artifact.
- Set `VT_BUILD_CHANNEL=stable` for stable artifacts; default is `dev`.

---

## Method 2: Manual Installation

If you prefer to set up the dependencies yourself, follow these steps.

### Step 1: Download and Install Go

1.  **Download:** Go to the official Go website: [go.dev/dl/](https://go.dev/dl/)
2.  **Install:** Run the installer and follow the on-screen instructions.
3.  **Verify:** Open a Command Prompt and type `go version`. You should see the installed Go version.

### Step 2: Install MSYS2 (UCRT64)

1.  **Download:** Go to https://www.msys2.org/ and download the installer.
2.  **Install:** Use the default install path (`C:\msys64`).
3.  **Install GCC:** Open "MSYS2 UCRT64" and run:
    ```
    pacman -S --needed base-devel mingw-w64-ucrt-x86_64-toolchain
    ```
4.  **PATH:** Ensure `C:\msys64\ucrt64\bin` is on your PATH (restart terminals after install).
### Step 3: Download FFmpeg

FFmpeg is the engine that powers VideoTools.

1.  **Download:** Go to the recommended FFmpeg builds page: [github.com/BtbN/FFmpeg-Builds/releases](https://github.com/BtbN/FFmpeg-Builds/releases)
2.  Download the file named `ffmpeg-master-latest-win64-gpl.zip`.

### Step 4: Place FFmpeg Files

You have two options for where to place the FFmpeg files:

#### Option A: Bundle with VideoTools (Portable)

This is the easiest option.

1.  Open the downloaded `ffmpeg-...-win64-gpl.zip`.
2.  Navigate into the `bin` folder inside the zip file.
3.  Copy `ffmpeg.exe` and `ffprobe.exe`.
4.  Paste them into the root directory of the VideoTools project, right next to `VideoTools.exe` (or `main.go` if you are building from source).

Your folder should look like this:
```
\---VideoTools
    |   VideoTools.exe  (or the built executable)
    |   ffmpeg.exe      <-- Copied here
    |   ffprobe.exe     <-- Copied here
    |   main.go
    \---...
```

#### Option B: Install System-Wide

This makes FFmpeg available to all applications on your system.

1.  Extract the entire `ffmpeg-...-win64-gpl.zip` to a permanent location, like `C:\Program Files\ffmpeg`.
2.  Add the FFmpeg `bin` directory to your system's PATH environment variable.
    *   Press the Windows key and type "Edit the system environment variables".
    *   Click the "Environment Variables..." button.
    *   Under "System variables", find and select the `Path` variable, then click "Edit...".
    *   Click "New" and add the path to your FFmpeg `bin` folder (e.g., `C:\Program Files\ffmpeg\bin`).
3.  **Verify:** Open a Command Prompt and type `ffmpeg -version`. You should see the version information.

### Step 5: Build and Run

1.  Open a Command Prompt in the VideoTools project directory.
2.  Run the build script: `scripts\build.bat`
3.  Run the application: `run.bat`

---

## Troubleshooting

-   **"FFmpeg not found" Error:** This means VideoTools can't locate `ffmpeg.exe`. Ensure it's either in the same folder as `VideoTools.exe` or that the system-wide installation path is correct.
-   **Installer Parse Errors:** If the setup script reports PowerShell parse errors, update the repository to the latest version and re-run `scripts\_internal\setup-windows.bat`.
-   **Application Doesn't Start:** Make sure you have a 64-bit version of Windows 10 or 11 and that your graphics drivers are up to date.
-   **Antivirus Warnings:** Some antivirus programs may flag the unsigned executable. This is a false positive.



