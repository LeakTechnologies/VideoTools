# VideoTools Uninstaller for Windows
# Removes VideoTools and its dependencies while preserving shared tools

param(
    [switch]$RemoveAll = $false,
    [switch]$KeepBuildTools = $false,
    [switch]$RemoveFFmpeg = $false,
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
    $line = "════════════════════════════════════════════════════════════════"
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
    if (-not $RemoveFFmpeg) {
        Write-Warning "Skipping FFmpeg removal (preserve by default - use -RemoveFFmpeg to remove)"
        return
    }
    
    # Remove portable FFmpeg if bundled with VideoTools
    $ffmpegPaths = @(
        "$script:ProjectRoot\ffmpeg.exe",
        "$script:ProjectRoot\ffprobe.exe"
    )
    
    foreach ($path in $ffmpegPaths) {
        Remove-IfExists -Path $path -Description "FFmpeg binary"
    }
    
    # Note: We don't remove system-wide FFmpeg as it might be used by other apps
    Write-Warning "System-wide FFmpeg preserved (may be used by other applications)"
}

function Remove-GStreamer {
    if ($KeepGStreamer) {
        Write-Warning "Skipping GStreamer removal (KeepGStreamer specified)"
        return
    }
    
    # Remove GStreamer if it was installed via MSI and no other apps are using it
    # This is conservative - we only remove if explicitly told to do so
    if ($RemoveAll -or $Force) {
        $gstreamerKeys = @(
            "HKLM:\SOFTWARE\GStreamer",
            "HKLM:\SOFTWARE\Wow6432Node\GStreamer"
        )
        
        foreach ($key in $gstreamerKeys) {
            Remove-RegistryValue -Key $key -Description "GStreamer registry entries"
        }
        
        # Remove GStreamer from PATH if added by VideoTools
        $envPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
        if ($envPath -like "*GStreamer*") {
            Write-Warning "GStreamer found in system PATH - manual cleanup may be required"
            Write-Warning "Check System Environment Variables: PATH"
        }
    } else {
        Write-Warning "GStreamer preserved (may be used by other applications)"
    }
}

function Remove-PythonPackages {
    # Remove Python packages installed by VideoTools
    if (Test-Command pip) {
        try {
            $packages = @("openai-whisper", "torch", "torchaudio", "numpy")
            foreach ($package in $packages) {
                pip uninstall -y $package 2>$null
                if ($LASTEXITCODE -eq 0) {
                    Write-Success "Removed Python package: $package"
                }
            }
        } catch {
            Write-Warning "Failed to remove some Python packages"
        }
    }
}

function Remove-WhisperModels {
    # Remove Whisper model files
    $modelPaths = @(
        "$script:ProjectRoot\models",
        "$env:USERPROFILE\.cache\whisper",
        "$env:LOCALAPPDATA\whisper"
    )
    
    foreach ($path in $modelPaths) {
        Remove-IfExists -Path $path -Description "Whisper models" -Recurse
    }
}

function Remove-DVDTools {
    # Remove DVD authoring tools if installed by VideoTools
    $dvdPaths = @(
        "$script:ProjectRoot\DVDStyler",
        "$script:ProjectRoot\DVDStyler.exe"
    )
    
    foreach ($path in $dvdPaths) {
        Remove-IfExists -Path $path -Description "DVD authoring tools" -Recurse
    }
}

function Show-Summary {
    Write-Header "Uninstall Summary"
    
    Write-Info "VideoTools files and cache removed"
    if (-not $KeepBuildTools) {
        Write-Info "Build tools (MSYS2) removed from project directory"
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
    if (-not $RemoveFFmpeg) {
        Write-Warning "- System-wide FFmpeg installation (preserved by default)"
    }
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