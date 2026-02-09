param(
    [switch]$Force,
    [switch]$SkipBuildTools,
    [switch]$InstallBuildTools,
    [switch]$SkipFFmpeg,
    [switch]$SkipGStreamer,
    [switch]$InstallPython,
    [switch]$SkipPython,
    [switch]$SkipWhisper,
    [switch]$InstallWhisper,
    [string]$GStreamerVersion = "1.26.10",
    [string]$GStreamerRuntimeMsi,
    [string]$GStreamerDevelMsi,
    [switch]$PreferWinget
)

function Write-Header {
    param(
        [string]$Title
    )
    $line = "════════════════════════════════════════════════════════════════"
    Write-Host $line -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host $line -ForegroundColor Cyan
    Write-Host ""
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO]  $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR]  $Message" -ForegroundColor Red
}

function Write-Skip {
    param([string]$Message)
    Write-Host "[SKIP]  $Message" -ForegroundColor Yellow
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
    Write-Info "Installing Chocolatey package manager..."
    try {
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
        
        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
        
        if (Test-Command choco) {
            Write-Success "Chocolatey installed successfully"
            choco upgrade -y
        } else {
            throw "Chocolatey installation failed"
        }
    } catch {
        Write-Error "Failed to install Chocolatey: $($_.Exception.Message)"
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
                Write-Skip "$DisplayName already installed"
                return $true
            } else {
                Write-Info "Installing $DisplayName..."
                $result = choco install $PackageName -y --accept-license --ignore-package-exit-codes
                if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 3010) {
                    Write-Success "$DisplayName installed successfully"
                    return $true
                } else {
                    if ($Required) {
                        Write-Error "Failed to install required package $DisplayName"
                        return $false
                    } else {
                        Write-Skip "Failed to install $DisplayName (optional)"
                        return $true
                    }
                }
            }
        } catch {
            Write-Error "Error installing $DisplayName`: $($_.Exception.Message)"
            if ($Required) { return $false }
            return $true
        }
    }
    return $false
}

function Install-GStreamer {
    if ($SkipGStreamer) {
        Write-Skip "Skipping GStreamer installation"
        return $true
    }

    Write-Info "Installing GStreamer (required for video playback)..."
    
    if ($PreferWinget -and (Test-Command winget)) {
        try {
            Write-Info "Attempting to install GStreamer via winget..."
            winget install --id GStreamer.GStreamer -e --accept-package-agreements --accept-source-agreements
            if ($LASTEXITCODE -eq 0) {
                Write-Success "GStreamer installed via winget"
                return $true
            }
        } catch {
            Write-Info "Winget installation failed, trying MSI approach..."
        }
    }

    # MSI installation approach
    try {
        $runtimeUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/gstreamer-1.0-msvc-x86_64-$($GStreamerVersion)-msvc.msi"
        $develUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/gstreamer-1.0-devel-msvc-x86_64-$($GStreamerVersion)-msvc.msi"
        
        if ($GStreamerRuntimeMsi) {
            $runtimeUrl = $GStreamerRuntimeMsi
        }
        if ($GStreamerDevelMsi) {
            $develUrl = $GStreamerDevelMsi
        }

        Write-Info "Downloading GStreamer runtime..."
        $runtimeMsi = Join-Path $env:TEMP "gstreamer-runtime.msi"
        Invoke-WebRequest -Uri $runtimeUrl -OutFile $runtimeMsi -UseBasicParsing

        Write-Info "Downloading GStreamer development..."
        $develMsi = Join-Path $env:TEMP "gstreamer-devel.msi"
        Invoke-WebRequest -Uri $develUrl -OutFile $develMsi -UseBasicParsing

        Write-Info "Installing GStreamer packages..."
        Start-Process -FilePath "msiexec" -ArgumentList "/i", "`"$runtimeMsi`"", "/quiet", "ADDLOCAL=ALL" -Wait -NoNewWindow
        Start-Process -FilePath "msiexec" -ArgumentList "/i", "`"$develMsi`"", "/quiet", "ADDLOCAL=ALL" -Wait -NoNewWindow

        Remove-Item $runtimeMsi, $develMsi -ErrorAction SilentlyContinue
        Write-Success "GStreamer installed successfully"
        return $true
    } catch {
        Write-Error "Failed to install GStreamer: $($_.Exception.Message)"
        return $false
    }
}

function Install-WhisperModel {
    if ($SkipWhisper) {
        Write-Skip "Skipping Whisper model installation"
        return
    }

    Write-Info "Installing Whisper model for subtitles..."
    try {
        $modelDir = Join-Path $env:USERPROFILE "Videos\VideoTools\models"
        if (-not (Test-Path $modelDir)) {
            New-Item -ItemType Directory -Path $modelDir -Force | Out-Null
        }

        $modelPath = Join-Path $modelDir "whisper-small.bin"
        if (Test-Path $modelPath -and -not $Force) {
            Write-Skip "Whisper model already exists"
            return
        }

        $modelUrl = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin"
        Write-Info "Downloading Whisper model..."
        Invoke-WebRequest -Uri $modelUrl -OutFile $modelPath -UseBasicParsing
        Write-Success "Whisper model installed"
    } catch {
        Write-Skip "Failed to download Whisper model: $($_.Exception.Message)"
    }
}

# Main installation flow
Write-Header "VideoTools Windows Installation"

Write-Info "Build tools are provided via Chocolatey."
Write-Info "This replaces the previous MSYS2 UCRT64 toolchain approach."

# Install Chocolatey
if (-not (Test-Command choco)) {
    if (-not (Install-Chocolatey)) {
        Write-Error "Chocolatey installation failed. Cannot proceed."
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
    Write-Info "Installing build tools..."
    Install-Package -PackageName "golang" -DisplayName "Go programming language" -Required
    Install-Package -PackageName "git" -DisplayName "Git version control" -Required
} else {
    Write-Skip "Skipping build tools installation"
}

# Install FFmpeg
if (-not $SkipFFmpeg) {
    Install-Package -PackageName "ffmpeg" -DisplayName "FFmpeg video processing"
} else {
    Write-Skip "Skipping FFmpeg installation"
}

# Install Python
if ($InstallPython) {
    Install-Package -PackageName "python" -DisplayName "Python with pip"
} elseif (-not $SkipPython) {
    Write-Host "Install Python + pip? (y/N): " -ForegroundColor Yellow -NoNewline
    $response = Read-Host
    if ($response -match '^[Yy]') {
        Install-Package -PackageName "python" -DisplayName "Python with pip"
    } else {
        Write-Skip "Skipping Python installation"
    }
}

# Install GStreamer (required)
if (-not (Install-GStreamer)) {
    Write-Error "GStreamer installation failed. Video playback may not work."
}

# Install Whisper model
if ($InstallWhisper) {
    Install-WhisperModel
} elseif (-not $SkipWhisper) {
    Write-Host "Install Whisper model for subtitles? (y/N): " -ForegroundColor Yellow -NoNewline
    $response = Read-Host
    if ($response -match '^[Yy]') {
        Install-WhisperModel
    } else {
        Write-Skip "Skipping Whisper model installation"
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
    
    Write-Success "Start menu shortcuts created"
} catch {
    Write-Skip "Failed to create shortcuts: $($_.Exception.Message)"
}

Write-Success "VideoTools dependencies installation completed!"
Write-Info "Next steps:"
Write-Info "  1. Run: .\scripts\windows\build.bat"
Write-Info "  2. Run: .\VideoTools.exe"