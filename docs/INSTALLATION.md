# VideoTools Installation Guide

This guide will help you install VideoTools with minimal setup.

## Quick Start (Recommended for Most Users)

### One-Command Installation

```bash
bash scripts/install.sh
```

That's it! The installer will:

1. ✅ Check your Go installation
2. ✅ Build VideoTools from source
3. ✅ Install the binary to your system
4. ✅ Set up shell aliases automatically
5. ✅ Configure your shell environment

### After Installation

Reload your shell:

```bash
# For bash users:
source ~/.bashrc

# For zsh users:
source ~/.zshrc
```

Then start using VideoTools:

```bash
VideoTools
```

---

## Installation Options

### Option 1: System-Wide Installation (Recommended for Shared Computers)

```bash
bash scripts/install.sh
# Select option 1 when prompted
# Enter your password if requested
```

**Advantages:**
- ✅ Available to all users on the system
- ✅ Binary in standard system path
- ✅ Professional setup

**Requirements:**
- Sudo access (for system-wide installation)

---

### Option 2: User-Local Installation (Recommended for Personal Use)

```bash
bash scripts/install.sh
# Select option 2 when prompted (default)
```

**Advantages:**
- ✅ No sudo required
- ✅ Works immediately
- ✅ Private to your user account
- ✅ No administrator needed

**Requirements:**
- None - works on any system!

---

## What the Installer Does

The `scripts/install.sh` script performs these steps:

### Step 1: Go Verification
- Checks if Go 1.21+ is installed
- Displays Go version
- Exits with helpful error message if not found

### Step 2: Build
- Cleans previous builds
- Downloads dependencies
- Compiles VideoTools binary
- Validates build success

### Step 3: Installation Path Selection
- Presents two options:
  - System-wide (`/usr/local/bin`)
  - User-local (`~/.local/bin`)
- Creates directories if needed

### Step 4: Binary Installation
- Copies binary to selected location
- Sets proper file permissions (755)
- Validates installation

### Step 5: Shell Environment Setup
- Detects your shell (bash/zsh)
- Adds VideoTools installation path to PATH
- Sources alias script for convenience commands
- Adds to appropriate rc file (`.bashrc` or `.zshrc`)

---

## Convenience Commands

After installation, you'll have access to:

```bash
VideoTools              # Run VideoTools directly
VideoToolsRebuild       # Force rebuild from source
VideoToolsClean         # Clean build artifacts and cache
```

---

## Development Workflow

For day-to-day development:

```bash
./scripts/build.sh
./scripts/run.sh
```

Use `./scripts/install.sh` when you add new system dependencies or want to reinstall.

---

## Requirements

### Essential
- **Go 1.21 or later** - https://go.dev/dl/
- **Bash or Zsh** shell

### Optional
- **FFmpeg** (for actual video encoding)
  ```bash
  ffmpeg -version
  ```

### System
- Linux, macOS, or Windows (native)
- At least 2 GB free disk space
- Stable internet connection (for dependencies)

---

## Troubleshooting

### "Go is not installed"

**Solution:** Install Go from https://go.dev/dl/

```bash
# After installing Go, verify:
go version
```

### Build Failed

**Solution:** Check build log for specific errors:

```bash
bash scripts/install.sh
# Look for error messages in the build log output
```

### Installation Path Not in PATH

If you see this warning:

```
Warning: ~/.local/bin is not in your PATH
```

**Solution:** Reload your shell:

```bash
source ~/.bashrc    # For bash
source ~/.zshrc     # For zsh
```

Or manually add to your shell configuration:

```bash
# Add this line to ~/.bashrc or ~/.zshrc:
export PATH="$HOME/.local/bin:$PATH"
```

### "Permission denied" on binary

**Solution:** Ensure file has correct permissions:

```bash
chmod +x ~/.local/bin/VideoTools
# or for system-wide:
ls -l /usr/local/bin/VideoTools
```

### Aliases Not Working

**Solution:** Ensure alias script is sourced:

```bash
# Check if this line is in your ~/.bashrc or ~/.zshrc:
source /path/to/VideoTools/scripts/alias.sh

# If not, add it manually:
echo 'source /path/to/VideoTools/scripts/alias.sh' >> ~/.bashrc
source ~/.bashrc
```

---

## Advanced: Manual Installation

If you prefer to install manually:

### Step 1: Build

```bash
cd /path/to/VideoTools
CGO_ENABLED=1 go build -o VideoTools .
```

### Step 2: Install Binary

```bash
# User-local installation:
mkdir -p ~/.local/bin
cp VideoTools ~/.local/bin/VideoTools
chmod +x ~/.local/bin/VideoTools

# System-wide installation:
sudo cp VideoTools /usr/local/bin/VideoTools
sudo chmod +x /usr/local/bin/VideoTools
```

### Step 3: Setup Aliases

```bash
# Add to ~/.bashrc or ~/.zshrc:
source /path/to/VideoTools/scripts/alias.sh

# Add to PATH if needed:
export PATH="$HOME/.local/bin:$PATH"
```

### Step 4: Reload Shell

```bash
source ~/.bashrc    # for bash
source ~/.zshrc     # for zsh
```

---

## Uninstallation

### If Installed System-Wide

```bash
sudo rm /usr/local/bin/VideoTools
```

### If Installed User-Local

```bash
rm ~/.local/bin/VideoTools
```

### Remove Shell Configuration

Remove these lines from `~/.bashrc` or `~/.zshrc`:

```bash
# VideoTools installation path
export PATH="$HOME/.local/bin:$PATH"

# VideoTools convenience aliases
source "/path/to/VideoTools/scripts/alias.sh"
```

---

## Verification

After installation, verify everything works:

```bash
# Check binary is accessible:
which VideoTools

# Check version/help:
VideoTools --help

# Check aliases are available:
type VideoToolsRebuild
type VideoToolsClean
```

---

## Getting Help

For issues or questions:

1. Check **BUILD_AND_RUN.md** for build-specific help
2. Check **DVD_USER_GUIDE.md** for usage help
3. Review installation logs in `/tmp/videotools-build.log`
4. Check shell configuration files for errors

---

## Next Steps

After successful installation:

1. **Read the Quick Start Guide:**
   ```bash
   cat DVD_USER_GUIDE.md
   ```

2. **Launch VideoTools:**
   ```bash
   VideoTools
   ```

3. **Convert your first video:**
   - Go to Convert module
   - Load a video
   - Select "DVD-NTSC (MPEG-2)" or "DVD-PAL (MPEG-2)"
   - Click "Add to Queue"
   - Click "View Queue" → "Start Queue"

---

## Platform-Specific Notes

### Linux (Ubuntu/Debian)

Installation is fully automatic. The script handles all steps.

### Linux (Arch/Manjaro)

Same as above. Installation works without modification.

### macOS

Installation works but requires Xcode Command Line Tools:

```bash
xcode-select --install
```

### Windows (WSL)

Installation works in WSL environment. Ensure you have WSL with Linux distro installed.

---

Enjoy using VideoTools! 🎬
