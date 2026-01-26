# VideoTools Windows Setup Script
# Downloads FFmpeg and configures a portable or system-wide install

param(
    [switch]$Portable,
    [switch]$System
)

$ErrorActionPreference = "Stop"

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Windows Setup" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

if (-not $Portable -and -not $System) {
    Write-Host "Choose installation type:" -ForegroundColor Yellow
    Write-Host "  1) Portable (bundle FFmpeg with VideoTools)" -ForegroundColor White
    Write-Host "  2) System-wide (install FFmpeg to PATH)" -ForegroundColor White
    Write-Host ""
    $choice = Read-Host "Enter choice (1 or 2)"

    if ($choice -eq "2") {
        $System = $true
    } else {
        $Portable = $true
    }
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
$distDir = Join-Path $projectRoot "dist\windows"

$ffmpegUrl = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
$ffmpegZip = Join-Path $env:TEMP "ffmpeg-windows.zip"
$ffmpegExtract = Join-Path $env:TEMP "ffmpeg-extract"

Write-Host "Downloading FFmpeg for Windows..." -ForegroundColor Green
Write-Host "Source: $ffmpegUrl" -ForegroundColor Gray
Write-Host ""

try {
    Invoke-WebRequest -Uri $ffmpegUrl -OutFile $ffmpegZip -UseBasicParsing
    Write-Host "[OK] Download complete" -ForegroundColor Green
    Write-Host ""

    Write-Host "Extracting FFmpeg..." -ForegroundColor Green
    if (Test-Path $ffmpegExtract) {
        Remove-Item $ffmpegExtract -Recurse -Force
    }
    Expand-Archive -Path $ffmpegZip -DestinationPath $ffmpegExtract -Force

    $binDir = Get-ChildItem -Path $ffmpegExtract -Recurse -Directory -Filter "bin" | Select-Object -First 1
    if (-not $binDir) {
        throw "FFmpeg bin directory not found in downloaded archive"
    }
    $ffmpegExe = Join-Path $binDir.FullName "ffmpeg.exe"
    $ffprobeExe = Join-Path $binDir.FullName "ffprobe.exe"

    if (-not (Test-Path $ffmpegExe)) {
        throw "FFmpeg executable not found in downloaded archive"
    }

    Write-Host "[OK] Extraction complete" -ForegroundColor Green
    Write-Host ""

    if ($Portable) {
        Write-Host "Setting up portable installation..." -ForegroundColor Green

        if (-not (Test-Path $distDir)) {
            New-Item -ItemType Directory -Path $distDir -Force | Out-Null
        }

        Copy-Item $ffmpegExe -Destination $distDir -Force
        Copy-Item $ffprobeExe -Destination $distDir -Force

        Write-Host "[OK] FFmpeg installed to: $distDir" -ForegroundColor Green
        Write-Host ""

        $videoToolsExe = Join-Path $distDir "VideoTools.exe"
        if (Test-Path $videoToolsExe) {
            Write-Host "Setup complete (portable)" -ForegroundColor Green
            Write-Host ""
            Write-Host "Installation directory: $distDir" -ForegroundColor White
            Write-Host ""
            Write-Host "Contents:" -ForegroundColor White
            Get-ChildItem $distDir | Format-Table Name, Length -AutoSize
            Write-Host ""
            Write-Host "To run VideoTools:" -ForegroundColor Yellow
            Write-Host "  $videoToolsExe" -ForegroundColor White
            Write-Host ""
            Write-Host "Or double-click VideoTools.exe in:" -ForegroundColor Yellow
            Write-Host "  $distDir" -ForegroundColor White
        } else {
            Write-Host "FFmpeg setup complete, VideoTools.exe not found" -ForegroundColor Yellow
            Write-Host ""
            Write-Host "FFmpeg has been downloaded to: $distDir" -ForegroundColor White
            Write-Host ""
            Write-Host "Next steps:" -ForegroundColor Yellow
            Write-Host "  1. Build VideoTools.exe (see README.md)" -ForegroundColor White
            Write-Host "  2. Copy VideoTools.exe to: $distDir" -ForegroundColor White
            Write-Host "  3. Run VideoTools.exe" -ForegroundColor White
        }
    } elseif ($System) {
        Write-Host "Installing FFmpeg system-wide..." -ForegroundColor Green
        Write-Host "(This requires administrator privileges)" -ForegroundColor Yellow
        Write-Host ""

        $installDir = "C:\Program Files\ffmpeg\bin"

        $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

        if (-not $isAdmin) {
            Write-Host "This script needs to run as Administrator for system-wide installation." -ForegroundColor Red
            Write-Host ""
            Write-Host "Please:" -ForegroundColor Yellow
            Write-Host "  1. Right-click PowerShell" -ForegroundColor White
            Write-Host "  2. Select 'Run as Administrator'" -ForegroundColor White
            Write-Host "  3. Run this script again with -System flag" -ForegroundColor White
            Write-Host ""
            Write-Host "Or use portable installation instead:" -ForegroundColor Yellow
            Write-Host "  .\\scripts\\_internal\\setup-windows.ps1 -Portable" -ForegroundColor White
            Write-Host ""
            exit 1
        }

        if (-not (Test-Path $installDir)) {
            New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        }

        Copy-Item $ffmpegExe -Destination $installDir -Force
        Copy-Item $ffprobeExe -Destination $installDir -Force

        Write-Host "[OK] FFmpeg installed to: $installDir" -ForegroundColor Green
        Write-Host ""

        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$installDir*") {
            Write-Host "Adding FFmpeg to system PATH..." -ForegroundColor Green
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "Machine")
            $env:Path = "$env:Path;$installDir"
            Write-Host "[OK] PATH updated" -ForegroundColor Green
        } else {
            Write-Host "[OK] FFmpeg already in PATH" -ForegroundColor Green
        }
        Write-Host ""

        Write-Host "Verifying installation..." -ForegroundColor Green
        $ffmpegVersion = & "$installDir\ffmpeg.exe" -version 2>&1 | Select-Object -First 1
        Write-Host "[OK] $ffmpegVersion" -ForegroundColor Green
        Write-Host ""

        Write-Host "Setup complete (system-wide)" -ForegroundColor Green
        Write-Host ""
        Write-Host "FFmpeg installed to: $installDir" -ForegroundColor White
        Write-Host "PATH updated: Yes" -ForegroundColor White
        Write-Host ""
        Write-Host "You can now run VideoTools from anywhere." -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Note: Restart any open Command Prompt or PowerShell windows" -ForegroundColor Yellow
        Write-Host "for the PATH changes to take effect." -ForegroundColor Yellow
    }
} catch {
    Write-Host ""
    Write-Host "Setup failed: $_" -ForegroundColor Red
    Write-Host ""
    exit 1
} finally {
    if (Test-Path $ffmpegZip) {
        Remove-Item $ffmpegZip -Force
    }
    if (Test-Path $ffmpegExtract) {
        Remove-Item $ffmpegExtract -Recurse -Force
    }
}

Write-Host ""
