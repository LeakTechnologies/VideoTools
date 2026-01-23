# VideoTools Installation Guide for Windows

This guide provides step-by-step instructions for installing VideoTools on Windows 10 and 11.

---

## Method 1: Automated Installation (Recommended)

This method uses a script to automatically download and configure all necessary dependencies.

### Step 1: Download the Project

If you haven't already, download the project files as a ZIP and extract them to a folder on your computer (e.g., `C:\Users\YourUser\Documents\VideoTools`).

### Step 2: Run the Setup Script

1.  Open the project folder in File Explorer.
2.  Find and double-click on `setup-windows.bat`.
3.  A terminal window will open and run the PowerShell setup script. This will:
    *   **Download FFmpeg:** The script automatically fetches the latest stable version of FFmpeg, which is required for all video operations.
    *   **Install Dependencies:** It places the necessary files in the correct directories.
    *   **Configure for Portability:** By default, it sets up VideoTools as a "portable" application, meaning all its components (like `ffmpeg.exe`) are stored directly within the project's `scripts/` folder.

> **Note:** If Windows Defender SmartScreen appears, click "More info" and then "Run anyway". This is expected as the application is not yet digitally signed.

### Step 3: Run VideoTools

Once the script finishes, you can run the application by double-clicking `run.bat` in the main project folder.

> **Note:** On Windows, use `setup-windows.bat` instead of `scripts/install.sh` to avoid mixed-shell prompts.

---

## Method 2: Manual Installation

If you prefer to set up the dependencies yourself, follow these steps.

### Step 1: Download and Install Go

1.  **Download:** Go to the official Go website: [go.dev/dl/](https://go.dev/dl/)
2.  **Install:** Run the installer and follow the on-screen instructions.
3.  **Verify:** Open a Command Prompt and type `go version`. You should see the installed Go version.

### Step 2: Download FFmpeg

FFmpeg is the engine that powers VideoTools.

1.  **Download:** Go to the recommended FFmpeg builds page: [github.com/BtbN/FFmpeg-Builds/releases](https://github.com/BtbN/FFmpeg-Builds/releases)
2.  Download the file named `ffmpeg-master-latest-win64-gpl.zip`.

### Step 3: Place FFmpeg Files

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

### Step 4: Build and Run

1.  Open a Command Prompt in the VideoTools project directory.
2.  Run the build script: `scripts\build.bat`
3.  Run the application: `run.bat`

---

## Troubleshooting

-   **"FFmpeg not found" Error:** This means VideoTools can't locate `ffmpeg.exe`. Ensure it's either in the same folder as `VideoTools.exe` or that the system-wide installation path is correct.
-   **Installer Parse Errors:** If the setup script reports PowerShell parse errors, update the repository to the latest version and re-run `setup-windows.bat`.
-   **Application Doesn't Start:** Make sure you have a 64-bit version of Windows 10 or 11 and that your graphics drivers are up to date.
-   **Antivirus Warnings:** Some antivirus programs may flag the unsigned executable. This is a false positive.
