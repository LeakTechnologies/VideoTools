# VideoTools Uninstaller for Windows
# Removes VideoTools and its dependencies while preserving shared tools

param(
    [switch]$RemoveAll = $false,
    [switch]$KeepBuildTools = $false,
    [switch]$KeepGStreamer = $false,
    [switch]$Force = $false,
    [switch]$WhatIf = $false
)

$ErrorActionPreference = "Stop"

$script:ProjectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

function Write-Header {
    param(
        [string]$Title
    )
    $line = "==============================================================="
    Write-Host $line -ForegroundColor Yellow
    Write-Host "  $Title" -ForegroundColor Yellow
    Write-Host $line -ForegroundColor Yellow
    Write-Host ""
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO]  $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK]    $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARN]  $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Test-Command {
    param($Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Remove-IfExists {
    param(
        [string]$Path,
        [string]$Description = "",
        [switch]$Recurse = $false
    )
    if (-not (Test-Path $Path)) {
        return $true
    }
    
    if ($WhatIf) {
        Write-Info "Would remove: $Path"
        return $true
    }
    
    try {
        if ($Recurse) {
            Remove-Item -Path $Path -Recurse -Force
        } else {
            Remove-Item -Path $Path -Force
        }
        if ($Description) {
            Write-Success "Removed $Description"
        }
        return $true
    } catch {
        Write-Error "Failed to remove $Path`: $($_.Exception.Message)"
        return $false
    }
}

function Remove-RegistryValue {
    param(
        [string]$Key,
        [string]$ValueName = "",
        [string]$Description = ""
    )
    if ($WhatIf) {
        Write-Info "Would remove registry: $Key\$ValueName"
        return $true
    }
    
    try {
        if (Test-Path $Key) {
            if ($ValueName) {
                Remove-ItemProperty -Path $Key -Name $ValueName -ErrorAction SilentlyContinue
            } else {
                Remove-Item -Path $Key -Recurse -Force -ErrorAction SilentlyContinue
            }
            if ($Description) {
                Write-Success "Removed $Description from registry"
            }
        }
        return $true
    } catch {
        Write-Error "Failed to remove registry $Key`: $($_.Exception.Message)"
        return $false
    }
}

function Remove-StartMenuItems {
    $startMenuPaths = @(
        "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\VideoTools",
        "$env:PROGRAMDATA\Microsoft\Windows\Start Menu\Programs\VideoTools"
    )
    
    foreach ($path in $startMenuPaths) {
        Remove-IfExists -Path $path -Description "Start Menu items" -Recurse
    }
}

function Remove-DesktopShortcuts {
    $desktopPaths = @(
        "$env:PUBLIC\Desktop\VideoTools.lnk",
        "$env:USERPROFILE\Desktop\VideoTools.lnk"
    )
    
    foreach ($path in $desktopPaths) {
        Remove-IfExists -Path $path -Description "Desktop shortcut"
    }
}

function Remove-VideoToolsFiles {
    # Remove build artifacts
    $distPaths = @(
        "$script:ProjectRoot\dist",
        "$script:ProjectRoot\VideoTools.exe",
        "$script:ProjectRoot\VideoTools",
        "$script:ProjectRoot\ffmpeg.exe",
        "$script:ProjectRoot\ffprobe.exe"
    )
    
    foreach ($path in $distPaths) {
        Remove-IfExists -Path $path -Description "VideoTools executable/file" -Recurse
    }
    
    # Remove cache and temporary files
    $cachePaths = @(
        "$env:LOCALAPPDATA\VideoTools",
        "$env:TEMP\VideoTools*",
        "$script:ProjectRoot\.git\objects\pack\*.pack", # Build caches if any
        "$script:ProjectRoot\vendor" # Go vendor cache
    )
    
    foreach ($path in $cachePaths) {
        if ($path -like "*\*") {
            Get-ChildItem -Path $path -ErrorAction SilentlyContinue | ForEach-Object {
                Remove-IfExists -Path $_.FullName -Description "Cache file" -Recurse
            }
        } else {
            Remove-IfExists -Path $path -Description "Cache directory" -Recurse
        }
    }
}

function Remove-BuildTools {
    if ($KeepBuildTools) {
        Write-Warning "Skipping build tools removal (KeepBuildTools specified)"
        return
    }
    
    # Only remove MSYS2 if it was installed by VideoTools (check for manifest)
    $manifestPath = "$script:ProjectRoot\Tools\msys2-packages.lock"
    $msys2Path = "$script:ProjectRoot\Tools\msys64"
    
    if ((Test-Path $manifestPath) -and (Test-Path $msys2Path)) {
        Write-Info "Removing VideoTools-installed MSYS2 toolchain..."
        Remove-IfExists -Path $msys2Path -Description "MSYS2 toolchain" -Recurse
        Remove-IfExists -Path $manifestPath -Description "MSYS2 package manifest"
    } else {
        Write-Info "MSYS2 not installed by VideoTools (no manifest found) - preserving"
    }
    
    # Remove tools directory if empty
    $toolsPath = "$script:ProjectRoot\Tools"
    if (Test-Path $toolsPath) {
        try {
            $items = Get-ChildItem -Path $toolsPath -ErrorAction SilentlyContinue
            if (-not $items -or $items.Count -eq 0) {
                Remove-IfExists -Path $toolsPath -Description "Empty Tools directory" -Recurse
            }
        } catch {
            # Ignore errors checking directory contents
        }
    }
    
    # Note: We don't automatically remove system-wide Go as it wasn't installed by VT
    Write-Info "System-wide Go installation preserved (not installed by VideoTools)"
}
    
    # Remove repo-local MSYS2 installation
    $msys2Path = "$script:ProjectRoot\Tools\msys64"
    Remove-IfExists -Path $msys2Path -Description "MSYS2 toolchain" -Recurse
    
    # Remove tools directory if empty
    $toolsPath = "$script:ProjectRoot\Tools"
    if (Test-Path $toolsPath) {
        try {
            $items = Get-ChildItem -Path $toolsPath -ErrorAction SilentlyContinue
            if (-not $items -or $items.Count -eq 0) {
                Remove-IfExists -Path $toolsPath -Description "Empty Tools directory" -Recurse
            }
        } catch {
            # Ignore errors checking directory contents
        }
    }
    
    # Remove MSYS2 package manifest
    $manifestPath = "$script:ProjectRoot\Tools\msys2-packages.lock"
    Remove-IfExists -Path $manifestPath -Description "MSYS2 package manifest"
    
    # Remove Go installation if it was installed by VideoTools and not shared
    # Note: We don't automatically remove Go as it might be used by other tools
    Write-Warning "Go installation preserved (may be used by other applications)"
}

function Remove-FFmpeg {
    Write-Info "FFmpeg not removed (not installed by VideoTools)"
    
    $ffmpegPaths = @(
        "$script:ProjectRoot\ffmpeg.exe",
        "$script:ProjectRoot\ffprobe.exe"
    )
    
    foreach ($path in $ffmpegPaths) {
        if (Test-Path $path) {
            Write-Info "FFmpeg binary preserved at project root"
        }
    }
}
    }
}
    }
}
    }
}
    }
}
    }
}

function Remove-GStreamer {
    if ($KeepGStreamer) {
        Write-Warning "Skipping GStreamer removal (KeepGStreamer specified)"
        return
    }
    
    # VideoTools only installs GStreamer via MSI installer
    # We should be very careful here - only remove if explicitly requested via -RemoveAll
    if ($RemoveAll) {
        Write-Warning "WARNING: -RemoveAll specified - this will remove GStreamer system-wide"
        Write-Warning "Only proceed if you're sure no other applications use GStreamer"
        
        if (-not $Force) {
            $response = Read-Host "Are you sure you want to remove GStreamer system-wide? (y/N)"
            if ($response -notmatch '^[Yy]') {
                Write-Info "Skipping GStreamer removal"
                return
            }
        }
        
        # Remove GStreamer registry entries (only those added by VideoTools installer)
        $gstreamerKeys = @(
            "HKLM:\SOFTWARE\GStreamer",
            "HKLM:\SOFTWARE\Wow6432Node\GStreamer"
        )
        
        foreach ($key in $gstreamerKeys) {
            Remove-RegistryValue -Key $key -Description "GStreamer registry entries"
        }
        
        Write-Warning "GStreamer removed - you may need to manually clean PATH entries"
        Write-Warning "Check System Environment Variables for remaining GStreamer paths"
    } else {
        Write-Info "GStreamer preserved (system-wide installation - use -RemoveAll to remove)"
        Write-Info "VideoTools does not manage system-wide GStreamer unless explicitly installed via VT installer"
    }
}

function Remove-PythonPackages {
    # Only remove Python packages that were installed by VideoTools
    # Check if VideoTools has a record of installing these packages
    $vtPythonMarker = "$script:ProjectRoot\.vt-python-installed"
    
    if (-not (Test-Path $vtPythonMarker)) {
        Write-Info "No record of VideoTools-installed Python packages found - preserving"
        return
    }
    
    if (Test-Command pip) {
        try {
            Write-Info "Removing Python packages installed by VideoTools..."
            $packages = @("openai-whisper", "torch", "torchaudio", "numpy")
            $removedCount = 0
            
            foreach ($package in $packages) {
                # Check if package is actually installed before trying to remove
                $packageInfo = pip show $package 2>$null
                if ($packageInfo -and $LASTEXITCODE -eq 0) {
                    pip uninstall -y $package 2>$null
                    if ($LASTEXITCODE -eq 0) {
                        Write-Success "Removed Python package: $package"
                        $removedCount++
                    } else {
                        Write-Warning "Failed to remove Python package: $package"
                    }
                }
            }
            
            if ($removedCount -gt 0) {
                Write-Info "Removed $removedCount Python packages installed by VideoTools"
            }
            
            # Remove the marker file
            Remove-IfExists -Path $vtPythonMarker -Description "VideoTools Python installation marker"
            
        } catch {
            Write-Warning "Failed to remove some Python packages (may require manual cleanup)"
        }
    } else {
        Write-Info "pip not found - cannot remove Python packages"
    }
}

function Remove-WhisperModels {
    # Only remove Whisper models that were downloaded by VideoTools
    $vtModelsPath = "$script:ProjectRoot\models"
    $vtWhisperMarker = "$script:ProjectRoot\.vt-whisper-downloaded"
    
    $removedCount = 0
    
    # Remove VideoTools-local models first
    if (Remove-IfExists -Path $vtModelsPath -Description "VideoTools local Whisper models" -Recurse) {
        $removedCount++
    }
    
    # Only remove user cache if VideoTools has a record of downloading there
    if (Test-Path $vtWhisperMarker) {
        $userCachePaths = @(
            "$env:USERPROFILE\.cache\whisper",
            "$env:LOCALAPPDATA\whisper"
        )
        
        foreach ($path in $userCachePaths) {
            if (Remove-IfExists -Path $path -Description "Whisper cache (downloaded by VideoTools)" -Recurse) {
                $removedCount++
            }
        }
        
        # Remove the marker file
        Remove-IfExists -Path $vtWhisperMarker -Description "VideoTools Whisper download marker"
    } else {
        Write-Info "No record of VideoTools-downloaded Whisper models in user cache"
    }
    
    if ($removedCount -gt 0) {
        Write-Info "Cleaned up Whisper models downloaded by VideoTools"
    }
}

function Remove-DVDTools {
    # Only remove DVD tools that were installed by VideoTools
    $vtDvdMarker = "$script:ProjectRoot\.vt-dvdstyler-installed"
    
    if (-not (Test-Path $vtDvdMarker)) {
        Write-Info "No record of VideoTools-installed DVD tools found"
        return
    }
    
    $dvdPaths = @(
        "$script:ProjectRoot\DVDStyler",
        "$script:ProjectRoot\DVDStyler.exe",
        "$script:ProjectRoot\Tools\DVDStyler"
    )
    
    $removedCount = 0
    foreach ($path in $dvdPaths) {
        if (Remove-IfExists -Path $path -Description "DVD tools installed by VideoTools" -Recurse) {
            $removedCount++
        }
    }
    
    if ($removedCount -gt 0) {
        Write-Info "Removed DVD tools installed by VideoTools"
    }
    
    # Remove the marker file
    Remove-IfExists -Path $vtDvdMarker -Description "VideoTools DVD tools installation marker"
}

function Show-Summary {
    Write-Header "Uninstall Summary"
    
    Write-Info "VideoTools files and cache removed"
    Write-Info "Start Menu items removed"
    Write-Info "Desktop shortcuts removed"
    
    $msys2Manifest = "$script:ProjectRoot\Tools\msys2-packages.lock"
    if ((Test-Path $msys2Manifest) -and -not $KeepBuildTools) {
        Write-Info "VideoTools-installed MSYS2 toolchain removed"
    } elseif (-not $KeepBuildTools) {
        Write-Info "MSYS2 preserved (not installed by VideoTools)"
    }
    
    Write-Info "FFmpeg preserved (not installed by VideoTools)"
    
    if ($RemoveAll) {
        Write-Warning "System-wide GStreamer removed (explicitly requested)"
    } else {
        Write-Info "GStreamer preserved (system-wide installation)"
    }
    
    Write-Host ""
    Write-Info "Project directory at: $script:ProjectRoot"
}
    
    Write-Info "FFmpeg preserved (not installed by VideoTools)"
    
    if ($RemoveAll) {
        Write-Warning "GStreamer removed"
    } else {
        Write-Info "GStreamer preserved"
    }
    
    Write-Host ""
    Write-Info "Project directory at: $script:ProjectRoot"
}
    
    Write-Info "FFmpeg preserved (not installed by VideoTools)"
    
    if ($RemoveAll) {
        Write-Warning "System-wide GStreamer removed (explicitly requested)"
    } else {
        Write-Info "GStreamer preserved (system-wide installation)"
    }
    
    Write-Host ""
    Write-Info "VideoTools only removes components it specifically installed:"
    Write-Info "- System-wide installations (Go, FFmpeg, GStreamer) are preserved by default"
    Write-Info "- Only project-local tools and bundled binaries are removed automatically"
    Write-Info "- Installation markers are checked before removing any dependencies"
    
    Write-Host ""
    Write-Info "Project directory at: $script:ProjectRoot"
    Write-Info "Installation markers and cache cleaned"
}
    if ($RemoveFFmpeg) {
        Write-Info "Portable FFmpeg binaries removed"
    }
    if (-not $KeepGStreamer -and ($RemoveAll -or $Force)) {
        Write-Info "GStreamer registry entries cleaned"
    }
    
    Write-Host ""
    Write-Warning "The following items were preserved (may require manual cleanup):"
    if ($KeepBuildTools -or -not $RemoveAll) {
        Write-Warning "- System-wide Go installation (if exists)"
        Write-Warning "- System-wide MSYS2 installation (if exists)"
    }
    Write-Warning "- System-wide FFmpeg installation (VideoTools never removes FFmpeg)"
    if ($KeepGStreamer -or -not $RemoveAll) {
        Write-Warning "- System-wide GStreamer installation"
        Write-Warning "- GStreamer PATH entries (manual cleanup may be needed)"
    }
    
    Write-Host ""
    Write-Info "Project directory at: $script:ProjectRoot"
    Write-Info "Start Menu items removed"
    Write-Info "Desktop shortcuts removed"
    Write-Info "Cache and temporary files cleaned"
}

# Main execution
Write-Header "VideoTools Uninstaller for Windows"

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning "Running without Administrator privileges"
    Write-Warning "Some items may not be completely removed"
    Write-Host ""
}

if (-not $Force -and -not $WhatIf) {
    Write-Warning "This will remove VideoTools and its components."
    Write-Warning "Use -Force to skip confirmation prompts."
    Write-Host ""
    $response = Read-Host "Do you want to continue? (y/N)"
    if ($response -notmatch '^[Yy]') {
        Write-Info "Uninstall cancelled."
        exit 0
    }
}

try {
    Write-Info "Removing VideoTools..."
    
    Remove-StartMenuItems
    Remove-DesktopShortcuts
    Remove-VideoToolsFiles
    Remove-BuildTools
    Remove-FFmpeg
    Remove-GStreamer
    Remove-PythonPackages
    Remove-WhisperModels
    Remove-DVDTools
    
    Show-Summary
    
    Write-Success "VideoTools uninstall completed successfully!"
    
} catch {
    Write-Error "Uninstall failed: $($_.Exception.Message)"
    exit 1
}

Write-Host ""
Write-Info "Press any key to exit..."
if (-not $WhatIf) {
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}