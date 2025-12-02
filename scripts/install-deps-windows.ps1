# VideoTools Dependency Installer for Windows
# Installs all required build and runtime dependencies using Chocolatey or Scoop

param(
    [switch]$UseScoop = $false,
    [switch]$SkipFFmpeg = $false
)

Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  VideoTools Dependency Installer (Windows)" -ForegroundColor Cyan
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Check if running as administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "⚠️  This script should be run as Administrator for best results" -ForegroundColor Yellow
    Write-Host "   Right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    Write-Host ""
    $continue = Read-Host "Continue anyway? (y/N)"
    if ($continue -ne 'y' -and $continue -ne 'Y') {
        exit 1
    }
    Write-Host ""
}

# Function to check if a command exists
function Test-Command {
    param($Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

# Function to install via Chocolatey
function Install-ViaChocolatey {
    Write-Host "📦 Using Chocolatey package manager..." -ForegroundColor Green

    # Check if Chocolatey is installed
    if (-not (Test-Command choco)) {
        Write-Host "Installing Chocolatey..." -ForegroundColor Yellow
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

        if (-not (Test-Command choco)) {
            Write-Host "❌ Failed to install Chocolatey" -ForegroundColor Red
            exit 1
        }
        Write-Host "✓ Chocolatey installed" -ForegroundColor Green
    } else {
        Write-Host "✓ Chocolatey already installed" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Installing dependencies via Chocolatey..." -ForegroundColor Yellow

    # Install Go
    if (-not (Test-Command go)) {
        Write-Host "Installing Go..." -ForegroundColor Yellow
        choco install -y golang
    } else {
        Write-Host "✓ Go already installed" -ForegroundColor Green
    }

    # Install GCC (via TDM-GCC or mingw)
    if (-not (Test-Command gcc)) {
        Write-Host "Installing MinGW-w64 (GCC)..." -ForegroundColor Yellow
        choco install -y mingw
    } else {
        Write-Host "✓ GCC already installed" -ForegroundColor Green
    }

    # Install Git (useful for development)
    if (-not (Test-Command git)) {
        Write-Host "Installing Git..." -ForegroundColor Yellow
        choco install -y git
    } else {
        Write-Host "✓ Git already installed" -ForegroundColor Green
    }

    # Install ffmpeg
    if (-not $SkipFFmpeg) {
        if (-not (Test-Command ffmpeg)) {
            Write-Host "Installing ffmpeg..." -ForegroundColor Yellow
            choco install -y ffmpeg
        } else {
            Write-Host "✓ ffmpeg already installed" -ForegroundColor Green
        }
    }

    Write-Host "✓ Chocolatey installation complete" -ForegroundColor Green
}

# Function to install via Scoop
function Install-ViaScoop {
    Write-Host "📦 Using Scoop package manager..." -ForegroundColor Green

    # Check if Scoop is installed
    if (-not (Test-Command scoop)) {
        Write-Host "Installing Scoop..." -ForegroundColor Yellow
        Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
        Invoke-Expression (New-Object System.Net.WebClient).DownloadString('https://get.scoop.sh')

        if (-not (Test-Command scoop)) {
            Write-Host "❌ Failed to install Scoop" -ForegroundColor Red
            exit 1
        }
        Write-Host "✓ Scoop installed" -ForegroundColor Green
    } else {
        Write-Host "✓ Scoop already installed" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Installing dependencies via Scoop..." -ForegroundColor Yellow

    # Install Go
    if (-not (Test-Command go)) {
        Write-Host "Installing Go..." -ForegroundColor Yellow
        scoop install go
    } else {
        Write-Host "✓ Go already installed" -ForegroundColor Green
    }

    # Install GCC
    if (-not (Test-Command gcc)) {
        Write-Host "Installing MinGW-w64 (GCC)..." -ForegroundColor Yellow
        scoop install mingw
    } else {
        Write-Host "✓ GCC already installed" -ForegroundColor Green
    }

    # Install Git
    if (-not (Test-Command git)) {
        Write-Host "Installing Git..." -ForegroundColor Yellow
        scoop install git
    } else {
        Write-Host "✓ Git already installed" -ForegroundColor Green
    }

    # Install ffmpeg
    if (-not $SkipFFmpeg) {
        if (-not (Test-Command ffmpeg)) {
            Write-Host "Installing ffmpeg..." -ForegroundColor Yellow
            scoop install ffmpeg
        } else {
            Write-Host "✓ ffmpeg already installed" -ForegroundColor Green
        }
    }

    Write-Host "✓ Scoop installation complete" -ForegroundColor Green
}

# Main installation logic
Write-Host "Checking system..." -ForegroundColor Yellow
Write-Host ""

# Check Windows version
$osVersion = [System.Environment]::OSVersion.Version
Write-Host "Windows Version: $($osVersion.Major).$($osVersion.Minor) (Build $($osVersion.Build))" -ForegroundColor Cyan

if ($osVersion.Major -lt 10) {
    Write-Host "⚠️  Warning: Windows 10 or later is recommended" -ForegroundColor Yellow
}
Write-Host ""

# Choose package manager
if ($UseScoop) {
    Install-ViaScoop
} else {
    # Check if either package manager is already installed
    $hasChoco = Test-Command choco
    $hasScoop = Test-Command scoop

    if ($hasChoco) {
        Install-ViaChocolatey
    } elseif ($hasScoop) {
        Install-ViaScoop
    } else {
        Write-Host "No package manager detected. Choose one:" -ForegroundColor Yellow
        Write-Host "  1. Chocolatey (recommended, requires admin)" -ForegroundColor White
        Write-Host "  2. Scoop (user-level, no admin required)" -ForegroundColor White
        Write-Host ""
        $choice = Read-Host "Enter choice (1 or 2)"

        if ($choice -eq "2") {
            Install-ViaScoop
        } else {
            Install-ViaChocolatey
        }
    }
}

Write-Host ""
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "✅ DEPENDENCIES INSTALLED" -ForegroundColor Green
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Refresh environment variables
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# Verify installations
Write-Host "Verifying installations..." -ForegroundColor Yellow
Write-Host ""

if (Test-Command go) {
    $goVersion = go version
    Write-Host "✓ Go: $goVersion" -ForegroundColor Green
} else {
    Write-Host "⚠️  Go not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command gcc) {
    $gccVersion = gcc --version | Select-Object -First 1
    Write-Host "✓ GCC: $gccVersion" -ForegroundColor Green
} else {
    Write-Host "⚠️  GCC not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command ffmpeg) {
    $ffmpegVersion = ffmpeg -version | Select-Object -First 1
    Write-Host "✓ ffmpeg: $ffmpegVersion" -ForegroundColor Green
} else {
    if ($SkipFFmpeg) {
        Write-Host "ℹ️  ffmpeg skipped (use -SkipFFmpeg:$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "⚠️  ffmpeg not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Command git) {
    $gitVersion = git --version
    Write-Host "✓ Git: $gitVersion" -ForegroundColor Green
} else {
    Write-Host "ℹ️  Git not installed (optional)" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "🎉 Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Restart your terminal/PowerShell" -ForegroundColor White
Write-Host "  2. Clone VideoTools repository" -ForegroundColor White
Write-Host "  3. Run: .\scripts\build.ps1" -ForegroundColor White
Write-Host ""
Write-Host "For GPU encoding support (NVIDIA):" -ForegroundColor Yellow
Write-Host "  - Ensure latest NVIDIA drivers are installed" -ForegroundColor White
Write-Host "  - NVENC will be automatically detected and used" -ForegroundColor White
Write-Host ""
