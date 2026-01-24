# VideoTools Dependency Installer for Windows
# Installs build and runtime dependencies using Scoop

param(
    [switch]$SkipFFmpeg = $false,
    [switch]$SkipGStreamer = $false,
    [switch]$InstallPython = $false,
    [switch]$SkipPython = $false,
    [string]$DvdStylerUrl = "",
    [string]$DvdStylerZip = "",
    [switch]$SkipDvdStyler = $false,
    [string]$GStreamerRuntimeUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/gstreamer-1.0-msvc-x86_64-1.24.8.msi",
    [string]$GStreamerDevelUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/1.0/msvc/gstreamer-1.0-devel-msvc-x86_64-1.24.8.msi"
)

$ErrorActionPreference = "Stop"

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Windows Installation" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[WARN]   Running without Administrator privileges." -ForegroundColor Yellow
    Write-Host "         GStreamer install requires Administrator when missing." -ForegroundColor Yellow
    Write-Host ""
}

function Test-Command {
    param($Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Test-Pip {
    if (Test-Command pip) {
        return $true
    }
    if (Test-Command pip3) {
        return $true
    }
    return $false
}

function Ensure-Scoop {
    if (-not (Test-Command scoop)) {
        Write-Host "Installing Scoop..." -ForegroundColor Yellow
        Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
        Invoke-Expression (New-Object System.Net.WebClient).DownloadString("https://get.scoop.sh")

        if (-not (Test-Command scoop)) {
            Write-Host "[ERROR]  Failed to install Scoop" -ForegroundColor Red
            exit 1
        }
        Write-Host "[OK]  Scoop installed" -ForegroundColor Green
    } else {
        Write-Host "[OK]  Scoop already installed" -ForegroundColor Green
    }
}

function Install-ViaScoop {
    Write-Host "Using Scoop package manager..." -ForegroundColor Green
    Ensure-Scoop

    $packages = New-Object System.Collections.Generic.List[string]
    if (-not (Test-Command go)) {
        $packages.Add("go")
    }
    if (-not (Test-Command gcc)) {
        $packages.Add("mingw")
    }
    if (-not $SkipFFmpeg -and -not (Test-Command ffmpeg)) {
        $packages.Add("ffmpeg")
    }
    if ($InstallPython -and -not (Test-Pip)) {
        $packages.Add("python")
    }

    if ($packages.Count -eq 0) {
        Write-Host "[OK]  Dependencies already installed" -ForegroundColor Green
        return
    }

    Write-Host "Installing: $($packages -join ', ')" -ForegroundColor Yellow
    scoop install @packages
}

function Install-GStreamerMsi {
    param(
        [string]$RuntimeUrl,
        [string]$DevelUrl
    )

    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        Write-Host "[ERROR]  GStreamer requires Administrator privileges to install." -ForegroundColor Red
        Write-Host "Run PowerShell as Administrator and re-run this installer." -ForegroundColor Yellow
        exit 1
    }

    $tempDir = Join-Path $env:TEMP ("gstreamer-" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

    $runtimeMsi = Join-Path $tempDir "gstreamer-runtime.msi"
    $develMsi = Join-Path $tempDir "gstreamer-devel.msi"

    Write-Host "Downloading GStreamer runtime..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $RuntimeUrl -OutFile $runtimeMsi -UseBasicParsing

    Write-Host "Downloading GStreamer development files..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $DevelUrl -OutFile $develMsi -UseBasicParsing

    Write-Host "Installing GStreamer runtime..." -ForegroundColor Yellow
    $runtime = Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$runtimeMsi`" /qn /norestart" -Wait -PassThru
    if ($runtime.ExitCode -ne 0) {
        throw "GStreamer runtime install failed with exit code $($runtime.ExitCode)"
    }

    Write-Host "Installing GStreamer development files..." -ForegroundColor Yellow
    $devel = Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$develMsi`" /qn /norestart" -Wait -PassThru
    if ($devel.ExitCode -ne 0) {
        throw "GStreamer dev install failed with exit code $($devel.ExitCode)"
    }

    Remove-Item -Recurse -Force $tempDir
}

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
    $dvdstylerZip = Join-Path $env:TEMP "dvdstyler-win64.zip"
    $needsDVDTools = (-not (Test-Command dvdauthor)) -or (-not (Test-Command mkisofs))

    if (-not $needsDVDTools) {
        return
    }

    Write-Host ""
    Write-Host "Optional module: DVD authoring tools (DVDStyler portable)" -ForegroundColor Yellow
    $dvdChoice = Read-Host "Install DVD authoring tools? (y/N)"
    if ($dvdChoice -ne "y" -and $dvdChoice -ne "Y") {
        $SkipDvdStyler = $true
        Write-Host "[SKIP] DVD authoring tools skipped" -ForegroundColor Yellow
        return
    }

    Write-Host "Installing DVD authoring tools (DVDStyler portable)..." -ForegroundColor Yellow
    if (-not (Test-Path $toolsRoot)) {
        New-Item -ItemType Directory -Force -Path $toolsRoot | Out-Null
    }
    if (Test-Path $dvdstylerDir) {
        Remove-Item -Recurse -Force $dvdstylerDir
    }

    $dvdZipProvided = $PSBoundParameters.ContainsKey("DvdStylerZip") -and $DvdStylerZip
    $dvdUrlProvided = $PSBoundParameters.ContainsKey("DvdStylerUrl") -and $DvdStylerUrl
    if ($dvdUrlProvided) {
        $env:VT_DVDSTYLER_URL = $DvdStylerUrl
        $dvdstylerUrls = @($DvdStylerUrl) + $dvdstylerUrls
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
    $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    $downloaded = $false
    $lastUrl = ""
    if ($dvdZipProvided) {
        if (Test-Path $DvdStylerZip) {
            Copy-Item -Path $DvdStylerZip -Destination $dvdstylerZip -Force
            $downloaded = $true
            $lastUrl = $DvdStylerZip
        } else {
            Write-Host "[ERROR]  Provided DVDStyler ZIP not found: $DvdStylerZip" -ForegroundColor Red
            exit 1
        }
    } else {
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

Write-Host "Checking system..." -ForegroundColor Yellow
Write-Host ""

$osVersion = [System.Environment]::OSVersion.Version
Write-Host "Windows Version: $($osVersion.Major).$($osVersion.Minor) (Build $($osVersion.Build))" -ForegroundColor Cyan

if ($osVersion.Major -lt 10) {
    Write-Host "[WARN]   Windows 10 or later is recommended" -ForegroundColor Yellow
}
Write-Host ""

if (-not (Test-Pip)) {
    if (-not $SkipPython -and -not $InstallPython) {
        Write-Host "Optional module: Python + pip (AI tooling and optional modules)" -ForegroundColor Yellow
        $pyChoice = Read-Host "Install Python + pip? (y/N)"
        if ($pyChoice -eq "y" -or $pyChoice -eq "Y") {
            $InstallPython = $true
        } else {
            $SkipPython = $true
        }
        Write-Host ""
    }
}

Install-ViaScoop

if (-not $SkipGStreamer -and -not (Test-Command gst-launch-1.0)) {
    Write-Host "GStreamer is required for VideoTools playback." -ForegroundColor Yellow
    Install-GStreamerMsi -RuntimeUrl $GStreamerRuntimeUrl -DevelUrl $GStreamerDevelUrl
}

Ensure-DVDStylerTools

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "[OK]  DEPENDENCIES INSTALLED" -ForegroundColor Green
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

# Refresh environment variables
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

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
        Write-Host "[INFO]   ffmpeg skipped (use -SkipFFmpeg:`$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   ffmpeg not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Command gst-launch-1.0) {
    $gstVersion = gst-launch-1.0 --version | Select-Object -First 1
    Write-Host "[OK]  gstreamer: $gstVersion" -ForegroundColor Green
} else {
    if ($SkipGStreamer) {
        Write-Host "[INFO]   gstreamer skipped (use -SkipGStreamer:`$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   gstreamer not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Pip) {
    if (Test-Command pip) {
        $pipVersion = pip --version
        Write-Host "[OK]  pip: $pipVersion" -ForegroundColor Green
    } elseif (Test-Command pip3) {
        $pipVersion = pip3 --version
        Write-Host "[OK]  pip: $pipVersion" -ForegroundColor Green
    }
} else {
    if ($SkipPython) {
        Write-Host "[INFO]   Python + pip skipped" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   pip not found in PATH (restart terminal)" -ForegroundColor Yellow
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

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host " Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Restart your terminal/PowerShell" -ForegroundColor White
Write-Host "  2. Build: .\\scripts\\build.ps1" -ForegroundColor White
Write-Host ""
