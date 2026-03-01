# VideoTools Installation Guide for Windows

This guide provides step-by-step instructions for installing VideoTools on Windows 10 and 11.

---

## Method 1: Automated Installation (Recommended)

This method uses Chocolatey package manager to automatically download and configure all necessary dependencies.

> **Note:** This dev installer uses Chocolatey for maximum reliability. The production installer plan is MSIX + WinGet. See `docs/WINDOWS_PACKAGING.md`.

### Step 1: Download the Project

If you haven't already, download the project files as a ZIP and extract them to a folder on your computer (e.g., `C:\Users\YourUser\Documents\VideoTools`).

### Step 2: Run the Dependency Installer

1.  Open the project folder in File Explorer.
2.  Choose one of these entrypoints:
    - Run `.\scripts\windows\install.bat` (recommended).
    - Run `.\scripts\windows\install.ps1` from PowerShell.
3.  A terminal window will open and run the PowerShell installer. It installs core dependencies with minimal changes:
    *   Go (build toolchain)
    *   Git (version control)
    *   FFmpeg (video processing)
    *   GStreamer (player backend)
    *   Python (pip for optional tooling)
    *   Note: GStreamer installs via MSI and requires an Administrator PowerShell window.
    *   Note: Build tools are optional; the installer will auto-install Go/Git via Chocolatey when missing unless -SkipBuildTools is set.
    *   Note: The installer will prompt for elevation when required and keep the admin window open for logs.
    *   Note: On success, the installer waits for a keypress before closing the window.

Support scripts are stored in `scripts/windows/support/`. Wrappers remain in `scripts/` for convenience and documentation stability.

Optional flags:
- `-SkipFFmpeg` or `-SkipGStreamer` to skip those dependencies.
- `-InstallBuildTools` to force install Go + Git via Chocolatey without a prompt.
- `-SkipBuildTools` to skip Go + Git via Chocolatey.
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
Build tools are provided via Chocolatey package manager for system-wide consistency and reliability.
If you opt into build tools, Go and Git will be installed automatically via Chocolatey packages.
The build script uses Chocolatey-installed toolchain for all compilation and packaging operations.
Logs default to `~/Videos/VideoTools/logs` and can be changed in Settings.

The installer defaults to MSI downloads for GStreamer. If MSI downloads fail, grab the MSI files from https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/ and re-run the installer with `-GStreamerRuntimeMsi` and `-GStreamerDevelMsi`. Use `-PreferWinget` if you want the installer to try winget first.

DVDStyler defaults to the Leak Technologies mirror installer (`DVDStyler-3.2.1-win64.exe`). If it fails and `-PreferWinget` is set, the installer tries winget before skipping the optional module.
SourceForge mirrors are only used when `VT_DVDSTYLER_ALLOW_SOURCEFORGE=1` is set in the environment.
Whisper uses the Leak Technologies mirror by default and can be overridden with `-WhisperModelUrl`.

> **Note:** If Windows Defender SmartScreen appears, click "More info" and then "Run anyway". This is expected as the application is not yet digitally signed.

### Step 3: Build and Run VideoTools

After the installer runs, a Start Menu folder named "VideoTools" is created. It includes a "Build VideoTools" shortcut and, once you build, a "VideoTools" app shortcut. The build script refreshes the app shortcut after a successful build.

1.  Build the app:
    - `.\scripts\windows\build.bat` (delegates to `build.ps1` and handles elevation)
    - If the build reports a toolchain failure, ensure Go and Git are installed via Chocolatey.
2.  Run the executable:
    - `.\VideoTools.exe`
    - The Player module includes a fullscreen toggle in the playback controls.
    - The main menu adapts to window width; resize the window to change tile density.
    - Settings and long panels use adaptive scroll speed for smoother navigation.
    - Settings tabs scroll independently so the tab header stays visible.
    - Preferences include hardware acceleration detection and module visibility toggles.
    - The Subtitles module can extract embedded tracks losslessly or OCR image-based tracks to SRT/ASS (requires Tesseract).
    - Conversion workers now catch internal panics and surface a failure dialog instead of closing the UI.
    - If a conversion was active when the app closed, a recovery notice appears on next launch.
    - If FFmpeg is missing on first launch, VideoTools will offer an in-app install to `%LOCALAPPDATA%\VideoTools\bin`.
    - The same app-local FFmpeg install action is available in Settings > Dependencies.

> **Bundled package:** Includes FFmpeg, Tesseract (eng/fra required, iku optional), GStreamer, and the whisper.cpp small model (`ggml-small.bin`). Launch with `run-bundled.ps1` (or `run-bundled.bat`) so the bundled dependencies are used.

> **Note:** `scripts\windows\build.bat` and `scripts\windows\build.ps1` will prompt for elevation to ensure build tools are available.
The build script pauses for a keypress on success or failure so you can review the output before the window closes.
If you are running inside a VM and see an OpenGL preflight warning, enable 3D acceleration or install GPU drivers before retrying.

Optional: Chocolatey packages are automatically added to system PATH during installation.

If you want a portable FFmpeg bundle placed next to the Windows executable, run:
- `.\scripts\windows\support\setup-windows.ps1 -Portable`

For a system-wide FFmpeg install (PATH), use:
- `.\scripts\windows\support\setup-windows.ps1 -System`

> **Note:** On Windows, use `scripts\windows\install.ps1` or `scripts\windows\install.bat`. Running `scripts\linux\install.sh` from PowerShell will open Git Bash.

---

## Uninstalling VideoTools

To completely remove VideoTools from your system:

### Method 1: Automated Uninstall (Recommended)

1.  Run the uninstaller from PowerShell as Administrator:
    - `.\scripts\windows\uninstall.bat` (recommended)
    - `.\scripts\windows\uninstall.ps1` from PowerShell

2.  The uninstaller preserves shared dependencies by default:
    *   **FFmpeg** - Kept (used by many video applications)
    *   **GStreamer** - Kept (media framework for other apps)
    *   **Go** - Kept (programming language used by other tools)
    *   **Python packages** - Removed if installed specifically for VideoTools

### Uninstall Options

- `-Force` - Skip confirmation prompts
- `-RemoveAll` - Remove ALL components including shared dependencies
- `-RemoveFFmpeg` - Remove bundled FFmpeg binaries (not system-wide FFmpeg)
- `-KeepBuildTools` - Preserve Chocolatey-installed build tools
- `-WhatIf` - Show what would be removed without actually removing

### Examples

```powershell
# Standard uninstall (preserves shared dependencies)
.\scripts\windows\uninstall.bat

# Force uninstall without prompts
.\scripts\windows\uninstall.bat -Force

# Remove everything including bundled FFmpeg
.\scripts\windows\windows\uninstall.bat -RemoveAll -RemoveFFmpeg

# Preview what would be removed
.\scripts\windows\uninstall.ps1 -WhatIf
```

### Manual Cleanup (if needed)

After uninstall, you may want to manually check:

- **System PATH**: Remove any VideoTools entries from Environment Variables
- **Start Menu**: Verify the VideoTools folder is removed
- **Desktop**: Check for any remaining shortcuts
- **App Data**: `C:\Users\YourUser\AppData\Local\VideoTools` (should be removed automatically)

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

### Step 2: Install Chocolatey

1.  **Install:** Open PowerShell as Administrator and run:
    ```
    Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
    ```
2.  **Verify:** Close and reopen PowerShell, then run `choco --version` to verify installation.
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
2.  Run the build script: `scripts\windows\build.bat`
3.  Run the application: `run.bat`

---

## Troubleshooting

-   **"FFmpeg not found" Error:** This means VideoTools can't locate `ffmpeg.exe`. Ensure it's either in the same folder as `VideoTools.exe` or that the system-wide installation path is correct.
-   **Installer Parse Errors:** If the setup script reports PowerShell parse errors, update the repository to the latest version and re-run `scripts\windows\support\setup-windows.bat`.
-   **Forgejo runner output errors:** If a Windows Forgejo runner logs `invalid format '', expected a line with '=' or '<<'`, ensure the workflow writes `GITHUB_OUTPUT` using UTF-8 without BOM.
-   **Console window closes app:** Windows Forgejo packages are built as GUI-only binaries; a console window should not appear when launching the packaged `.exe`.
-   **Extra files in package:** Forgejo Windows packages ship only `VideoTools.exe` and `README.md` inside the zip at `dist/windows/`. Artifact uploads include the zip only.
-   **Dev release pipeline:** Forgejo dev release workflow builds Windows artifacts in the same run before publishing (standalone Windows workflow removed).
-   **Release assets:** Dev/stable releases upload only the Windows zip for Windows (no build metadata files).
-   **Duplicate assets:** Dev/stable releases delete existing assets with the same name before uploading new ones.
-   **Release cleanup:** Dev/stable releases purge existing assets before uploading to prevent duplicates.
-   **Release auth:** Dev/stable releases fail if asset deletion/upload is unauthorized.
-   **Application Doesn't Start:** Make sure you have a 64-bit version of Windows 10 or 11 and that your graphics drivers are up to date.
-   **Antivirus Warnings:** Some antivirus programs may flag the unsigned executable. This is a false positive.
