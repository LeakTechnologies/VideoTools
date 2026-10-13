# VideoTools Installation Guide for Linux & WSL

## Dev97 player integration

Player lifecycle, resume continuity, saved-position readiness and opt-in diagnostics are integrated. No new runtime dependency or packaging/workflow change. Windows playback acceptance is recorded in PLAYER_DEBUG.md; Linux runtime playback remains pending. Local diagnostic builds use the developer FFmpeg library path; shipped Windows static packaging is unchanged.

Dev97 player candidate: startup and pause/resume preserve buffered content and add no runtime dependencies. Windows software playback was witnessed and logged; residual faster-playback jitter and Linux runtime acceptance remain open. This candidate has not been merged or published.

Player lifecycle candidate (dev95, unlanded): source-owned cancellation and worker cleanup add no runtime dependencies. Automated Windows software-playback stress completed, but visual review is pending; Linux runtime review is also pending. No candidate installation/release is published.

Dev95: Convert keeps Load Video/source identity available when preview is closed, gives metadata the flexible space, and groups frame/cover and source-track actions under Player tools. No new runtime dependency. Verify layout with your selected font and display scaling.

Dev94 player note: native frame and timestamp waits now respond to cancellation/engine retirement. No new runtime dependencies. Basic software playback and close were checked on Windows; minor 2x-segment jitter remains under investigation. Linux runtime acceptance remains outstanding.


Dev93 restores Rip Content Browser title labels and selection controls. No new runtime dependencies. After updating, load a disc and check card visibility and selection while scrolling.


This guide provides detailed instructions for installing VideoTools on Linux and Windows Subsystem for Linux (WSL) using the automated script.

---

## One-Command Installation

The recommended method for all Unix-like systems is the `scripts/linux/install.sh` script.

```bash
bash scripts/linux/install.sh
```

This single command automates the entire setup process.

### What the Installer Does

1.  **Go Verification:** Checks if Go (version 1.21 or later) is installed and available in your `PATH`.
2.  **Build from Source:** Cleans any previous builds, downloads all necessary Go dependencies, and compiles the `VideoTools` binary.
3.  **Path Selection:** Prompts you to choose an installation location:
    *   **System-wide:** `/usr/local/bin` (Requires `sudo` privileges). Recommended for multi-user systems.
    *   **User-local:** `~/.local/bin` (Default). Recommended for most users as it does not require `sudo`.
4.  **Install Binary:** Copies the compiled binary to the selected location and makes it executable.
5.  **Configure Shell:** Detects your shell (`bash` or `zsh`) and updates the corresponding resource file (`~/.bashrc` or `~/.zshrc`) to:
    *   Add the installation directory to your `PATH`.
    *   Source the matching alias script (`alias.sh` for bash, `alias.zsh` for zsh).
6.  **Whisper Model Seed (Optional):** Downloads the whisper.cpp small model from the Leak Technologies mirror (fallback to upstream) when missing.
7.  **In-App Dependency Actions:** Settings > Dependencies now includes FFmpeg install/uninstall actions using your detected Linux package manager.

### After Installation

You must reload your shell for the changes to take effect:

```bash
# For bash users:
source ~/.bashrc

# For zsh users:
source ~/.zshrc

# For fish users (manual setup required):
source ~/.config/fish/config.fish
```

You can now run the application from anywhere by simply typing `VideoTools`.
The Player module includes a fullscreen toggle in the playback controls.
Settings and long panels use adaptive scroll speed for smoother navigation.
Settings tabs scroll independently so the tab header stays visible.
Preferences include hardware acceleration detection and module visibility toggles.
The Subtitles module can extract embedded tracks losslessly or OCR image-based tracks to SRT/ASS (requires Tesseract).
Conversion workers now catch internal panics and surface a failure dialog instead of closing the UI.
If a conversion was active when the app closed, a recovery notice appears on next launch.

> **Package format:** Linux releases ship as the main VT package. Install runtime dependencies from Settings > Dependencies or via `scripts/linux/install.sh`.

---

## Convenience Commands

The installation script sets up a few helpful aliases:

-   `VideoTools`: Runs the main application.
-   `VideoToolsRebuild`: Forces a full rebuild of the application from source.
-   `VideoToolsClean`: Cleans all build artifacts and clears the Go cache for the project.

---

## Build Artifacts

- Build packages are written to `dist/linux/<channel>/`.
- Each build writes a `build.json` file alongside the zip artifact.
- Set `VT_BUILD_CHANNEL=stable` for stable artifacts; default is `dev`.
- Forgejo dev builds also publish an AppImage (`<version>_linux.AppImage`) with the VT icon embedded.

---

## Manual Installation

If you prefer to perform the steps manually:

1.  **Build the Binary:**
    ```bash
    CGO_ENABLED=1 go build -o VideoTools .
    ```

2.  **Install the Binary:**
    *   **User-local:**
        ```bash
        mkdir -p ~/.local/bin
        cp VideoTools ~/.local/bin/
        ```
    *   **System-wide:**
        ```bash
        sudo cp VideoTools /usr/local/bin/
        ```

3.  **Update Shell Configuration:**
    Add the following lines to your `~/.bashrc` or `~/.zshrc` file, replacing `/path/to/VideoTools` with the actual absolute path to the project directory.

    ```bash
    # Add VideoTools to PATH
    export PATH="$HOME/.local/bin:$PATH"

    # Source VideoTools aliases
    source /path/to/VideoTools/scripts/alias.sh   # bash
    # source /path/to/VideoTools/scripts/alias.zsh # zsh
    # source /path/to/VideoTools/scripts/alias.fish # fish
    ```

4.  **Reload Your Shell:**
    ```bash
    source ~/.bashrc  # Or source ~/.zshrc
    ```

---

## Uninstallation

1.  **Remove the Binary:**
    *   If installed user-locally: `rm ~/.local/bin/VideoTools`
    *   If installed system-wide: `sudo rm /usr/local/bin/VideoTools`

2.  **Remove Shell Configuration:**
    Open your `~/.bashrc` or `~/.zshrc` file and remove the lines that were added for `VideoTools`.

---

## Platform-Specific Notes

- Logs default to `~/Videos/VideoTools/logs` and can be changed in Settings.

-   **WSL:** The Linux instructions work without modification inside a WSL environment.
