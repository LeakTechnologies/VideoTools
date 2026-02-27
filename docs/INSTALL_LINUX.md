# VideoTools Installation Guide for Linux & WSL

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
