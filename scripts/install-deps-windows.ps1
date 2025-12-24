# VideoTools Dependency Installer for Windows
# Installs all required build and runtime dependencies using Chocolatey or Scoop

param(
    [switch]$UseScoop = $false,
    [switch]$SkipFFmpeg = $false,
    [string]$DvdStylerUrl = "",
    [switch]$SkipDvdStyler = $false
)

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Windows Installation" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

# Check if running as administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "[WARN]   This script should be run as Administrator for best results" -ForegroundColor Yellow
    Write-Host "   Right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    Write-Host ""
    $continue = Read-Host "Continue anyway? (y/N)"
    if ($continue -ne 'y' -and $continue -ne 'Y') {
        exit 1
    }
    Write-Host ""
}

if ($DvdStylerUrl) {
    $env:VT_DVDSTYLER_URL = $DvdStylerUrl
}

# Function to check if a command exists
function Test-Command {
    param($Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

# Ensure DVD authoring tools exist on Windows by downloading DVDStyler portable
function Ensure-DVDStylerTools {
    if ($SkipDvdStyler) {
        Write-Host "[SKIP] DVD authoring tools skipped (DVDStyler)" -ForegroundColor Yellow
        return
    }
    $toolsRoot = Join-Path $PSScriptRoot "tools"
    $dvdstylerDir = Join-Path $toolsRoot "dvdstyler"
    $dvdstylerBin = Join-Path $dvdstylerDir "bin"
    $dvdstylerReferer = "https://sourceforge.net/projects/dvdstyler/"
    $dvdstylerUrls = @(
        "https://downloads.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://netcologne.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://cfhcable.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://pilotfiber.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://versaweb.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://liquidtelecom.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://master.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://ufpr.dl.sourceforge.net/project/dvdstyler/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip",
        "https://sourceforge.net/projects/dvdstyler/files/DVDStyler/3.2.1/DVDStyler-3.2.1-win64.zip/download"
    )
    if ($env:VT_DVDSTYLER_URL) {
        $dvdstylerUrls = @($env:VT_DVDSTYLER_URL) + $dvdstylerUrls
    }
    $dvdstylerZip = Join-Path $env:TEMP "dvdstyler-win64.zip"
    $needsDVDTools = (-not (Test-Command dvdauthor)) -or (-not (Test-Command mkisofs))

    if (-not $needsDVDTools) {
        return
    }

    Write-Host "Installing DVD authoring tools (DVDStyler portable)..." -ForegroundColor Yellow
    if (-not (Test-Path $toolsRoot)) {
        New-Item -ItemType Directory -Force -Path $toolsRoot | Out-Null
    }
    if (Test-Path $dvdstylerDir) {
        Remove-Item -Recurse -Force $dvdstylerDir
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
    $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    $downloaded = $false
    $lastUrl = ""
    foreach ($url in $dvdstylerUrls) {
        $lastUrl = $url
        $downloadOk = $false
        if (Test-Path $dvdstylerZip) {
            Remove-Item -Force $dvdstylerZip
        }
        try {
            Invoke-WebRequest -Uri $url -OutFile $dvdstylerZip -UseBasicParsing -MaximumRedirection 10 -UserAgent $userAgent -Headers @{
                "Referer" = $dvdstylerReferer
                "Accept"  = "application/zip"
            }
            $downloadOk = $true
        } catch {
            $downloadOk = $false
        }

        if (-not $downloadOk) {
            try {
                Start-BitsTransfer -Source $url -Destination $dvdstylerZip -ErrorAction Stop
                $downloadOk = $true
            } catch {
                $downloadOk = $false
            }
        }

        if (-not $downloadOk -and (Test-Command curl.exe)) {
            try {
                & curl.exe -L --retry 3 --user-agent $userAgent -o $dvdstylerZip $url | Out-Null
                if ($LASTEXITCODE -eq 0) {
                    $downloadOk = $true
                }
            } catch {
                $downloadOk = $false
            }
        }

        if (-not $downloadOk -or -not (Test-Path $dvdstylerZip)) {
            continue
        }

        try {
            $fs = [System.IO.File]::OpenRead($dvdstylerZip)
            try {
                $fileSize = (Get-Item $dvdstylerZip).Length
                if ($fileSize -lt 102400) {
                    continue
                }
                $sig = New-Object byte[] 2
                $null = $fs.Read($sig, 0, 2)
                if ($sig[0] -eq 0x50 -and $sig[1] -eq 0x4B) {
                    $downloaded = $true
                    break
                }
            } finally {
                $fs.Close()
            }
        } catch {
            # Try next URL
        }
    }
    if (-not $downloaded) {
        Write-Host "[ERROR]  Failed to download DVDStyler ZIP (invalid archive)" -ForegroundColor Red
        Write-Host "Last URL tried: $lastUrl" -ForegroundColor Yellow
        Write-Host "Tip: Set VT_DVDSTYLER_URL to a direct ZIP link and retry." -ForegroundColor Yellow
        Write-Host "Manual download page: https://sourceforge.net/projects/dvdstyler/files/DVDStyler/3.2.1/" -ForegroundColor Yellow
        Write-Host "After download, extract and ensure bin\\dvdauthor.exe and bin\\mkisofs.exe are on PATH." -ForegroundColor Yellow
        exit 1
    }

    $extractRoot = Join-Path $env:TEMP ("dvdstyler-extract-" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force -Path $extractRoot | Out-Null
    Expand-Archive -Path $dvdstylerZip -DestinationPath $extractRoot -Force

    $entries = Get-ChildItem -Path $extractRoot
    if ($entries.Count -eq 1 -and $entries[0].PSIsContainer) {
        Copy-Item -Path (Join-Path $entries[0].FullName "*") -Destination $dvdstylerDir -Recurse -Force
    } else {
        Copy-Item -Path (Join-Path $extractRoot "*") -Destination $dvdstylerDir -Recurse -Force
    }

    Remove-Item -Force $dvdstylerZip
    Remove-Item -Recurse -Force $extractRoot

    if (Test-Path $dvdstylerBin) {
        $env:Path = "$dvdstylerBin;$env:Path"
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notmatch [Regex]::Escape($dvdstylerBin)) {
            [Environment]::SetEnvironmentVariable("Path", "$dvdstylerBin;$userPath", "User")
        }
        Write-Host "[OK]  DVD authoring tools installed to $dvdstylerDir" -ForegroundColor Green
    } else {
        Write-Host "[ERROR]  DVDStyler tools missing after install" -ForegroundColor Red
        exit 1
    }
}

# Function to install via Chocolatey
function Install-ViaChocolatey {
    Write-Host " Using Chocolatey package manager..." -ForegroundColor Green

    # Check if Chocolatey is installed
    if (-not (Test-Command choco)) {
        Write-Host "Installing Chocolatey..." -ForegroundColor Yellow
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

        if (-not (Test-Command choco)) {
            Write-Host "[ERROR]  Failed to install Chocolatey" -ForegroundColor Red
            exit 1
        }
        Write-Host "[OK]  Chocolatey installed" -ForegroundColor Green
    } else {
        Write-Host "[OK]  Chocolatey already installed" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Installing dependencies via Chocolatey..." -ForegroundColor Yellow

    # Install Go
    if (-not (Test-Command go)) {
        Write-Host "Installing Go..." -ForegroundColor Yellow
        choco install -y golang
    } else {
        Write-Host "[OK]  Go already installed" -ForegroundColor Green
    }

    # Install GCC (via TDM-GCC or mingw)
    if (-not (Test-Command gcc)) {
        Write-Host "Installing MinGW-w64 (GCC)..." -ForegroundColor Yellow
        choco install -y mingw
    } else {
        Write-Host "[OK]  GCC already installed" -ForegroundColor Green
    }

    # Install Git (useful for development)
    if (-not (Test-Command git)) {
        Write-Host "Installing Git..." -ForegroundColor Yellow
        choco install -y git
    } else {
        Write-Host "[OK]  Git already installed" -ForegroundColor Green
    }

    # Install ffmpeg
    if (-not $SkipFFmpeg) {
        if (-not (Test-Command ffmpeg)) {
            Write-Host "Installing ffmpeg..." -ForegroundColor Yellow
            choco install -y ffmpeg
        } else {
            Write-Host "[OK]  ffmpeg already installed" -ForegroundColor Green
        }
    }

    Write-Host "[OK]  Chocolatey installation complete" -ForegroundColor Green
}

# Function to install via Scoop
function Install-ViaScoop {
    Write-Host " Using Scoop package manager..." -ForegroundColor Green

    # Check if Scoop is installed
    if (-not (Test-Command scoop)) {
        Write-Host "Installing Scoop..." -ForegroundColor Yellow
        Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
        Invoke-Expression (New-Object System.Net.WebClient).DownloadString('https://get.scoop.sh')

        if (-not (Test-Command scoop)) {
            Write-Host "[ERROR]  Failed to install Scoop" -ForegroundColor Red
            exit 1
        }
        Write-Host "[OK]  Scoop installed" -ForegroundColor Green
    } else {
        Write-Host "[OK]  Scoop already installed" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Installing dependencies via Scoop..." -ForegroundColor Yellow

    # Install Go
    if (-not (Test-Command go)) {
        Write-Host "Installing Go..." -ForegroundColor Yellow
        scoop install go
    } else {
        Write-Host "[OK]  Go already installed" -ForegroundColor Green
    }

    # Install GCC
    if (-not (Test-Command gcc)) {
        Write-Host "Installing MinGW-w64 (GCC)..." -ForegroundColor Yellow
        scoop install mingw
    } else {
        Write-Host "[OK]  GCC already installed" -ForegroundColor Green
    }

    # Install Git
    if (-not (Test-Command git)) {
        Write-Host "Installing Git..." -ForegroundColor Yellow
        scoop install git
    } else {
        Write-Host "[OK]  Git already installed" -ForegroundColor Green
    }

    # Install ffmpeg
    if (-not $SkipFFmpeg) {
        if (-not (Test-Command ffmpeg)) {
            Write-Host "Installing ffmpeg..." -ForegroundColor Yellow
            scoop install ffmpeg
        } else {
            Write-Host "[OK]  ffmpeg already installed" -ForegroundColor Green
        }
    }

    Write-Host "[OK]  Scoop installation complete" -ForegroundColor Green
}

# Main installation logic
Write-Host "Checking system..." -ForegroundColor Yellow
Write-Host ""

# Check Windows version
$osVersion = [System.Environment]::OSVersion.Version
Write-Host "Windows Version: $($osVersion.Major).$($osVersion.Minor) (Build $($osVersion.Build))" -ForegroundColor Cyan

if ($osVersion.Major -lt 10) {
    Write-Host "[WARN]   Warning: Windows 10 or later is recommended" -ForegroundColor Yellow
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

Ensure-DVDStylerTools

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "[OK]  DEPENDENCIES INSTALLED" -ForegroundColor Green
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

# Refresh environment variables
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# Verify installations
Write-Host "Verifying installations..." -ForegroundColor Yellow
Write-Host ""

if (Test-Command go) {
    $goVersion = go version
    Write-Host "[OK]  Go: $goVersion" -ForegroundColor Green
} else {
    Write-Host "[WARN]   Go not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command gcc) {
    $gccVersion = gcc --version | Select-Object -First 1
    Write-Host "[OK]  GCC: $gccVersion" -ForegroundColor Green
} else {
    Write-Host "[WARN]   GCC not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command ffmpeg) {
    $ffmpegVersion = ffmpeg -version | Select-Object -First 1
    Write-Host "[OK]  ffmpeg: $ffmpegVersion" -ForegroundColor Green
} else {
    if ($SkipFFmpeg) {
        Write-Host "[INFO]   ffmpeg skipped (use -SkipFFmpeg:$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   ffmpeg not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Command dvdauthor) {
    Write-Host "[OK]  dvdauthor: found" -ForegroundColor Green
} else {
    Write-Host "[WARN]   dvdauthor not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command mkisofs) {
    Write-Host "[OK]  mkisofs: found" -ForegroundColor Green
} else {
    Write-Host "[WARN]   mkisofs not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command git) {
    $gitVersion = git --version
    Write-Host "[OK]  Git: $gitVersion" -ForegroundColor Green
} else {
    Write-Host "[INFO]   Git not installed (optional)" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host " Setup complete!" -ForegroundColor Green
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
