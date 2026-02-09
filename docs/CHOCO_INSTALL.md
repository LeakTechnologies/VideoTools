# VideoTools Chocolatey Installation Guide

## Overview
Chocolatey provides the most reliable "no-touch" Windows setup with system-wide package management and automatic dependency resolution.

## Quick Install (One Command)

```powershell
# Install Chocolatey + VideoTools dependencies
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1')); choco install golang ffmpeg git python -y
```

## Automated Script

Save as `scripts\windows\install-choco.ps1`:

```powershell
param([switch]$Force)

function Write-Header {
    param([string]$Title)
    $line = "════════════════════════════════════════════════════════════════"
    Write-Host $line -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host $line -ForegroundColor Cyan
    Write-Host ""
}

Write-Header "VideoTools Chocolatey Installation"

# Check admin
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[ERROR] This script requires Administrator privileges." -ForegroundColor Red
    Write-Host "Please run PowerShell as Administrator and retry." -ForegroundColor Yellow
    exit 1
}

# Install Chocolatey if missing
if (-not (Get-Command choco -ErrorAction SilentlyContinue)) {
    Write-Host "[INFO] Installing Chocolatey..." -ForegroundColor Cyan
    Set-ExecutionPolicy Bypass -Scope Process -Force
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
    iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
    refreshenv
}

# Install packages
$packages = @(
    "golang",
    "ffmpeg", 
    "git",
    "python"
)

Write-Host "[INFO] Installing VideoTools dependencies..." -ForegroundColor Cyan
foreach ($pkg in $packages) {
    $installed = choco list --exact $pkg --local-only
    if ($installed -match $pkg -and -not $Force) {
        Write-Host "[SKIP] $pkg already installed" -ForegroundColor Yellow
    } else {
        Write-Host "[INSTALL] $pkg" -ForegroundColor Green
        choco install $pkg -y --accept-license
    }
}

Write-Host "[SUCCESS] VideoTools dependencies installed!" -ForegroundColor Green
Write-Host "[INFO] Run: .\scripts\windows\build.bat to build VideoTools" -ForegroundColor Cyan
```

## Benefits vs MSYS2

| Feature | Chocolatey | MSYS2 |
|---------|------------|-------|
| Network reliability | ✅ Excellent | ❌ Connection issues |
| System integration | ✅ Native PATH | ❌ Isolated toolchain |
| Maintenance | ✅ Auto updates | ❌ Manual pacman |
| Windows support | ✅ Built-in | ❌ Unix emulation |
| Setup complexity | ✅ 1 command | ❌ Multi-step |

## Migration Path

1. Keep existing install script for backward compatibility
2. Add Chocolatey as primary method
3. Gradually deprecate MSYS2 approach
4. Update docs to recommend Chocolatey

This gives you the cleanest, most reliable Windows setup with minimal user interaction required.