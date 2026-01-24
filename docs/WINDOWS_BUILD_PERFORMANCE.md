# Windows Build Performance Guide

## Issue: Slow Builds (5+ Minutes)

If you're experiencing very slow build times on Windows, follow these steps to dramatically improve performance.

## Quick Fixes

### 1. Use the Optimized Build Scripts

We've updated the build scripts with performance optimizations:

```bash
# Git Bash (Most Windows users)
./scripts/build.sh

# PowerShell
.\scripts\build.ps1

# Command Prompt
.\scripts\build.bat
```

**New Optimizations:**
- `-p N`: Parallel compilation using all CPU cores
- `-trimpath`: Faster builds and smaller binaries
- `-ldflags="-s -w"`: Strip debug symbols (faster linking)

### 2. Add Windows Defender Exclusions (CRITICAL!)

**This is the #1 cause of slow builds on Windows.**

Windows Defender scans every intermediate `.o` file during compilation, adding 2-5 minutes to build time.

#### Automated Script (Easiest - For Git Bash Users):

**From Git Bash (Run as Administrator):**
```bash
# Run the automated exclusion script
powershell.exe -ExecutionPolicy Bypass -File ./scripts/add-defender-exclusions.ps1
```

**To run Git Bash as Administrator:**
1. Search for "Git Bash" in Start Menu
2. Right-click → "Run as administrator"
3. Navigate to your VideoTools directory
4. Run the command above

#### Manual Method (GUI):

1. **Open Windows Security**
   - Press `Win + I` → Update & Security → Windows Security → Virus & threat protection

2. **Add Exclusions** (Manage settings → Add or remove exclusions):
   - `C:\Users\YourName\go` - Go package cache
   - `C:\Users\YourName\AppData\Local\go-build` - Go build cache
   - `C:\Users\YourName\Projects\VideoTools` - Your project directory
   - `C:\msys64` - MinGW toolchain (if using MSYS2)

#### PowerShell Method (If Not Using Git Bash):

Run PowerShell as Administrator:
```powershell
# Run the automated script
.\scripts\add-defender-exclusions.ps1

# Or add manually:
Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\go-build"
Add-MpPreference -ExclusionPath "$env:USERPROFILE\go"
Add-MpPreference -ExclusionPath "C:\Users\$env:USERNAME\Projects\VideoTools"
Add-MpPreference -ExclusionPath "C:\msys64"
```

**Expected improvement:** 5 minutes → 30-90 seconds

### 3. Use Go Build Cache

Make sure Go's build cache is enabled (it should be by default):

```powershell
# Check cache location
go env GOCACHE

# Should output something like: C:\Users\YourName\AppData\Local\go-build
```

**Don't use `-Clean` flag** unless you're troubleshooting. Clean builds are much slower.

### 4. Optimize MinGW/GCC

If using MSYS2/MinGW, ensure it's in your PATH before other compilers:

```powershell
# Check GCC version
gcc --version

# Should show: gcc (GCC) 13.x or newer
```

## Advanced Optimizations

### 1. Use Faster SSD for Build Cache

Move your Go cache to an SSD if it's on an HDD:

```powershell
# Set custom cache location on fast SSD
$env:GOCACHE = "D:\FastSSD\go-build"
go env -w GOCACHE="D:\FastSSD\go-build"
```

### 2. Increase Go Build Parallelism

For high-core-count CPUs:

```powershell
# Use all CPU threads
$env:GOMAXPROCS = [Environment]::ProcessorCount

# Or set specific count
$env:GOMAXPROCS = 16
```

### 3. Disable Real-Time Scanning Temporarily

**Only during builds** (not recommended for normal use):

```powershell
# Disable (run as Administrator)
Set-MpPreference -DisableRealtimeMonitoring $true

# Build your project
.\scripts\build.ps1

# Re-enable immediately after
Set-MpPreference -DisableRealtimeMonitoring $false
```

## Benchmarking Your Build

Time your build to measure improvements:

```powershell
# PowerShell
Measure-Command { .\scripts\build.ps1 }

# Command Prompt
echo %time% && .\scripts\build.bat && echo %time%
```

## Expected Build Times

With optimizations:

| Machine Type | Clean Build | Incremental Build |
|--------------|-------------|-------------------|
| Modern Desktop (8+ cores, SSD) | 30-60 seconds | 5-15 seconds |
| Laptop (4-6 cores, SSD) | 60-90 seconds | 10-20 seconds |
| Older Machine (2-4 cores, HDD) | 2-3 minutes | 30-60 seconds |

**Without Defender exclusions:** Add 2-5 minutes to above times.

## Still Slow?

### Check for Common Issues:

1. **Antivirus Software**
   - Third-party antivirus can be even worse than Defender
   - Add same exclusions in your antivirus settings

2. **Disk Space**
   - Go cache can grow large
   - Ensure 5+ GB free space on cache drive

3. **Background Processes**
   - Close resource-heavy applications during builds
   - Check Task Manager for CPU/disk usage

4. **Network Drives**
   - **Never** build on network drives or cloud-synced folders
   - Move project to local SSD

5. **WSL2 vs Native Windows**
   - Building in WSL2 can be faster
   - But adds complexity with GUI apps

## Troubleshooting Commands

```powershell
# Check Go environment
go env

# Check build cache size
Get-ChildItem -Path (go env GOCACHE) -Recurse | Measure-Object -Property Length -Sum

# Clean cache if too large (>10 GB)
go clean -cache

# Verify GCC is working
gcc --version
```

## Getting Help

If you're still experiencing slow builds after following this guide:

1. **Capture build timing:**
   ```powershell
   Measure-Command { go build -v -x . } > build-log.txt 2>&1
   ```

2. **Check system specs:**
   ```powershell
   systeminfo | findstr /C:"Processor" /C:"Physical Memory"
   ```

3. **Report issue** with:
   - Build timing output
   - System specifications
   - Windows version
   - Antivirus software in use

## Summary: Quick Start for Git Bash Users

**If you're using Git Bash on Windows (most users), do this:**

1. **Open Git Bash as Administrator**
   - Right-click Git Bash → "Run as administrator"

2. **Navigate to VideoTools:**
   ```bash
   cd ~/Projects/VideoTools  # or wherever your project is
   ```

3. **Add Defender exclusions (ONE TIME ONLY):**
   ```bash
   powershell.exe -ExecutionPolicy Bypass -File ./scripts/add-defender-exclusions.ps1
   ```

4. **Close and reopen Git Bash (normal, not admin)**

5. **Build with optimized script:**
   ```bash
   ./scripts/build.sh
   ```

**Expected result:** 5+ minutes → 30-90 seconds

### What Each Step Does:
1. ✅ **Add Windows Defender exclusions** (saves 2-5 minutes) - Most important!
2. ✅ **Use optimized build scripts** (saves 30-60 seconds) - Parallel compilation
3. ✅ **Avoid clean builds** (saves 1-2 minutes) - Uses Go's build cache
