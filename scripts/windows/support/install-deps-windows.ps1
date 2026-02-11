# VideoTools Dependency Installer for Windows
# Installs build and runtime dependencies using Chocolatey

param(
    [switch]$SkipFFmpeg = $false,
    [switch]$SkipGStreamer = $false,
    [switch]$InstallBuildTools = $false,
    [switch]$SkipBuildTools = $false,
    [switch]$InstallPython = $false,
    [switch]$SkipPython = $false,
    [switch]$SkipWhisper = $false,
    [switch]$InstallWhisper = $false,
    [string]$GStreamerVersion = "1.28.0",
    [string]$GStreamerRuntimeMsi = "",
    [string]$GStreamerDevelMsi = "",
    [switch]$PreferWinget = $false
)

# Colors for output
$RED = [ConsoleColor]::Red
$GREEN = [ConsoleColor]::Green
$YELLOW = [ConsoleColor]::Yellow
$BLUE = [ConsoleColor]::Blue
$CYAN = [ConsoleColor]::Cyan
$NC = [ConsoleColor]::White

# Configuration
$PROJECT_ROOT = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))

function Write-Color {
    param(
        [string]$Message,
        [ConsoleColor]$Color = $NC
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Header {
    param([string]$Title)
    $line = "==============================================================="
    Write-Color $line $CYAN
    Write-Color "  $Title" $CYAN
    Write-Color $line $CYAN
    Write-Host ""
}

function Test-Command {
    param([string]$Command)
    try {
        Get-Command $Command -ErrorAction Stop | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Install-Chocolatey {
    Write-Color "[1/4] Installing Chocolatey package manager..." $CYAN
    try {
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
        
        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
        
        if (Test-Command choco) {
            Write-Color "[OK] Chocolatey installed successfully" $GREEN
            choco upgrade -y
        } else {
            throw "Chocolatey installation failed"
        }
    } catch {
        Write-Color "[ERROR] Failed to install Chocolatey: $($_.Exception.Message)" $RED
        return $false
    }
    return $true
}

function Install-Package {
    param(
        [string]$PackageName,
        [string]$DisplayName = $PackageName,
        [switch]$Required = $false
    )
    
    if (Test-Command choco) {
        try {
            $installed = choco list --exact $PackageName --local-only --exact
            if ($installed -match $PackageName -and -not $Force) {
                Write-Color "[OK] $DisplayName already installed" $GREEN
                return $true
            } else {
                Write-Color "Installing $DisplayName..." $YELLOW
                $result = choco install $PackageName -y --accept-license --ignore-package-exit-codes
                if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 3010) {
                    Write-Color "[OK] $DisplayName installed successfully" $GREEN
                    return $true
                } else {
                    if ($Required) {
                        Write-Color "[ERROR] Failed to install required package $DisplayName" $RED
                        return $false
                    } else {
                        Write-Color "[SKIP] Failed to install $DisplayName (optional)" $YELLOW
                        return $true
                    }
                }
            }
        } catch {
            Write-Color "[ERROR] Error installing $DisplayName`: $($_.Exception.Message)" $RED
            if ($Required) { return $false }
            return $true
        }
    }
    return $false
}

function Install-GStreamer {
    if ($SkipGStreamer) {
        Write-Color "[SKIP] Skipping GStreamer installation" $YELLOW
        return $true
    }

    Write-Color "[3/4] Installing GStreamer (required for video playback)..." $CYAN
    
    if ($PreferWinget -and (Test-Command winget)) {
        try {
            Write-Color "Attempting to install GStreamer via winget..." $YELLOW
            winget install --id GStreamer.GStreamer -e --accept-package-agreements --accept-source-agreements
            if ($LASTEXITCODE -eq 0) {
                Write-Color "[OK] GStreamer installed via winget" $GREEN
                return $true
            }
        } catch {
            Write-Color "Winget installation failed, trying MSI approach..." $YELLOW
        }
    }

    # MSI installation approach
    try {
        # Use mirror when available, fallback to official
        $installerUrl = "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/-/raw/main/gstreamer-1.0-msvc-x86_64-$($GStreamerVersion).exe"
        $fallbackInstallerUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/gstreamer-1.0-msvc-x86_64-$($GStreamerVersion).exe"
        
        if ($GStreamerRuntimeMsi) {
            $runtimeUrl = $GStreamerRuntimeMsi
            $fallbackRuntimeUrl = $GStreamerRuntimeMsi
        }
        if ($GStreamerDevelMsi) {
            $develUrl = $GStreamerDevelMsi
            $fallbackDevelUrl = $GStreamerDevelMsi
        }

        Write-Color "Downloading GStreamer installer..." $YELLOW
        $installerExe = Join-Path $env:TEMP "gstreamer-installer.exe"
        try {
            Invoke-WebRequest -Uri $installerUrl -OutFile $installerExe -UseBasicParsing
        } catch {
            Write-Color "Mirror failed, trying official source..." $YELLOW
            try {
                Invoke-WebRequest -Uri $fallbackInstallerUrl -OutFile $installerExe -UseBasicParsing
            } catch {
                Write-Color "[ERROR] Failed to download GStreamer installer from both mirror and official source: $($_.Exception.Message)" $RED
                return $false
            }
        }

        Write-Color "Installing GStreamer..." $YELLOW
        Start-Process -FilePath $installerExe -ArgumentList "/S" -Wait -NoNewWindow

        Remove-Item $installerExe -ErrorAction SilentlyContinue
        Write-Color "[OK] GStreamer installed successfully" $GREEN
        return $true
    } catch {
        Write-Color "[ERROR] Failed to install GStreamer: $($_.Exception.Message)" $RED
        return $false
    }
}

function Install-WhisperModel {
    if ($SkipWhisper) {
        Write-Color "[SKIP] Skipping Whisper model installation" $YELLOW
        return
    }

    Write-Color "Installing Whisper model for subtitles..." $CYAN
    try {
        $modelDir = Join-Path $env:USERPROFILE "Videos\VideoTools\models"
        if (-not (Test-Path $modelDir)) {
            New-Item -ItemType Directory -Path $modelDir -Force | Out-Null
        }

        $modelPath = Join-Path $modelDir "whisper-small.bin"
        if (Test-Path $modelPath -and -not $Force) {
            Write-Color "[OK] Whisper model already exists" $GREEN
            return
        }

        $modelUrl = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"
        Write-Color "Downloading Whisper model..." $YELLOW
        try {
            Invoke-WebRequest -Uri $modelUrl -OutFile $modelPath -UseBasicParsing
        } catch {
            Write-Color "[SKIP] Failed to download Whisper model: $($_.Exception.Message)" $YELLOW
            return
        }
        Write-Color "[OK] Whisper model installed" $GREEN
    } catch {
        Write-Color "[SKIP] Failed to download Whisper model: $($_.Exception.Message)" $YELLOW
    }
}

# Main installation flow
Write-Header "VideoTools Windows Installation"

# Check admin privileges for GStreamer MSI
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Color "[WARN] Running without Administrator privileges." $YELLOW
    Write-Color "       GStreamer install requires Administrator when missing." $YELLOW
    Write-Host ""
}

# Install Chocolatey
if (-not (Test-Command choco)) {
    if (-not (Install-Chocolatey)) {
        Write-Color "[ERROR] Chocolatey installation failed. Cannot proceed." $RED
        Write-Host "Press any key to close..." $CYAN
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }
}

# Determine build tools installation
$installBuild = $false
if ($InstallBuildTools) { $installBuild = $true }
elseif ($SkipBuildTools) { $installBuild = $false }
else {
    $installBuild = $true  # Default to installing build tools
}

# Install core packages
if ($installBuild) {
    Write-Color "[2/4] Installing build tools..." $CYAN
    Install-Package -PackageName "golang" -DisplayName "Go programming language" -Required
    Install-Package -PackageName "git" -DisplayName "Git version control" -Required
} else {
    Write-Color "[SKIP] Skipping build tools installation" $YELLOW
}

# Install FFmpeg
if (-not $SkipFFmpeg) {
    Install-Package -PackageName "ffmpeg" -DisplayName "FFmpeg video processing"
} else {
    Write-Color "[SKIP] Skipping FFmpeg installation" $YELLOW
}

# Install Python
if ($InstallPython) {
    Install-Package -PackageName "python" -DisplayName "Python with pip"
} elseif (-not $SkipPython) {
    Write-Host "Install Python + pip? [y/N]: " -ForegroundColor Yellow -NoNewline
    $response = Read-Host
    if ($response -match '^[Yy]') {
        Install-Package -PackageName "python" -DisplayName "Python with pip"
    } else {
        Write-Color "[SKIP] Skipping Python installation" $YELLOW
    }
}

# Install GStreamer (optional - mirror not available)
if (-not (Install-GStreamer)) {
    Write-Color "[WARN] GStreamer installation failed. Video playback may not work." $YELLOW
    Write-Color "       You can install GStreamer manually from: https://gstreamer.freedesktop.org/download/" $YELLOW
}

# Install Whisper model
if ($InstallWhisper) {
    Install-WhisperModel
} elseif (-not $SkipWhisper) {
    Write-Host "Install Whisper model for subtitles? [y/N]: " -ForegroundColor Yellow -NoNewline
    $response = Read-Host
    if ($response -match '^[Yy]') {
        Install-WhisperModel
    } else {
        Write-Color "[SKIP] Skipping Whisper model installation" $YELLOW
    }
}

# Create shortcuts
try {
    $startMenuPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\VideoTools"
    if (-not (Test-Path $startMenuPath)) {
        New-Item -ItemType Directory -Path $startMenuPath -Force | Out-Null
    }

    $buildShortcut = Join-Path $startMenuPath "Build VideoTools.lnk"
    $buildScript = Join-Path $PSScriptRoot "build.ps1"
    
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($buildShortcut)
    $shortcut.TargetPath = "powershell.exe"
    $shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$buildScript`""
    $shortcut.WorkingDirectory = $PSScriptRoot
    $shortcut.Save()
    
    Write-Color "[OK] Start menu shortcuts created" $GREEN
} catch {
    Write-Color "[SKIP] Failed to create shortcuts: $($_.Exception.Message)" $YELLOW
}

Write-Color "[SUCCESS] VideoTools dependencies installation completed!" $GREEN
Write-Host ""
Write-Color "Next steps:" $CYAN
Write-Color "  1. Run: .\scripts\windows\build.bat" $NC
Write-Color "  2. Run: .\VideoTools.exe" $NC
Write-Host ""
Write-Host "Press any key to close..." $CYAN
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")