# VideoTools Build Script for Windows
# Builds the VideoTools application using Chocolatey-installed toolchain

param(
    [switch]$Clean,
    [switch]$SkipTests
)

# Set console encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

function Write-Header {
    param(
        [string]$Title
    )
    $line = "==============================================================="
    Write-Host $line -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host $line -ForegroundColor Cyan
    Write-Host ""
}

function Write-Section {
    param(
        [string]$Title
    )
    Write-Host "===============================================================" -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host "===============================================================" -ForegroundColor Cyan
    Write-Host ""
}

function Test-Command {
    param([string]$Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Refresh-Path {
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
}

function Wait-ForKey {
    param(
        [string]$Message = "Press any key to close..."
    )
    if ($env:CI) {
        return
    }
    Write-Host $Message
    try {
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    } catch {
        # Ignore key read errors in non-interactive environments
    }
}

function Exit-WithPause {
    param(
        [int]$ExitCode = 0,
        [string]$Message = ""
    )
    if ($Message) {
        Write-Host $Message -ForegroundColor $(if ($ExitCode -eq 0) { "Green" } else { "Red" })
    }
    Wait-ForKey
    exit $ExitCode
}

# Project configuration
$PROJECT_ROOT = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$BUILD_OUTPUT = Join-Path $PROJECT_ROOT "VideoTools.exe"
$BUILD_CHANNEL = if ($env:VT_BUILD_CHANNEL) { $env:VT_BUILD_CHANNEL } else { "dev" }

Write-Header "VideoTools Build Script (Windows)"
Write-Host "Build Channel: $BUILD_CHANNEL" -ForegroundColor Cyan
Write-Host "Project Root: $PROJECT_ROOT" -ForegroundColor Cyan
Write-Host ""

# Check for required tools
Write-Section "Environment Check"

if (-not (Test-Command go)) {
    Write-Host " ERROR: Go is not installed or not in PATH." -ForegroundColor Red
    Write-Host " Run: .\scripts\windows\install.ps1 -InstallBuildTools" -ForegroundColor Yellow
    Exit-WithPause 1
}

if (-not (Test-Command git)) {
    Write-Host " ERROR: Git is not installed or not in PATH." -ForegroundColor Red
    Write-Host " Run: .\scripts\windows\install.ps1 -InstallBuildTools" -ForegroundColor Yellow
    Exit-WithPause 1
}

# Display tool versions
Write-Host "Go Version:" -ForegroundColor Green
go version
Write-Host ""

Write-Host "Git Version:" -ForegroundColor Green
git --version
Write-Host ""

# Check for FFmpeg (optional but recommended)
if (Test-Command ffmpeg) {
    Write-Host "FFmpeg Version:" -ForegroundColor Green
    ffmpeg -version | Select-Object -First 1
    Write-Host ""
} else {
    Write-Host "WARNING: FFmpeg not found in PATH. Video processing may not work." -ForegroundColor Yellow
    Write-Host "Run: .\scripts\windows\install.ps1 to install FFmpeg." -ForegroundColor Yellow
    Write-Host ""
}

# Change to project directory
Set-Location $PROJECT_ROOT

if ($Clean) {
    Write-Host "Cleaning previous builds and cache..." -ForegroundColor Yellow
    go clean -cache -modcache -testcache 2>$null
    if (Test-Path $BUILD_OUTPUT) {
        Remove-Item $BUILD_OUTPUT -Force
    }
    Write-Host "Cache cleaned" -ForegroundColor Green
    Write-Host ""
}

Write-Section "Go Modules"

# Check if modules are already downloaded (skip if already built)
$moduleCache = Join-Path $env:USERPROFILE "go\pkg\mod"
if ((Test-Path $moduleCache) -and (Get-ChildItem $moduleCache -ErrorAction SilentlyContinue | Measure-Object).Count -gt 0) {
    Write-Host "Using cached Go modules" -ForegroundColor Green
} else {
    Write-Host "Downloading Go modules..." -ForegroundColor Yellow
    go mod download
    if ($LASTEXITCODE -ne 0) {
        Write-Host " Failed to download Go modules" -ForegroundColor Red
        Exit-WithPause 1
    }
}

go mod verify
if ($LASTEXITCODE -ne 0) {
    Write-Host " Failed to verify Go modules" -ForegroundColor Red
    Exit-WithPause 1
}
Write-Host " Go modules ready" -ForegroundColor Green
Write-Host ""

Write-Section "Build"
Write-Host "Building VideoTools..." -ForegroundColor Yellow
Write-Host ""

# Set build environment variables
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "1"

# Build flags
$ldflags = @(
    "-s", "-w",  # Strip symbols
    "-X main.BuildChannel=$BUILD_CHANNEL",
    "-X main.BuildDate=$(Get-Date -Format 'yyyy-MM-dd')",
    "-X main.BuildVersion=$(git describe --tags --always 2>$null)"
) -join " "

# Progress indicator
$progressChars = @("|", "/", "-", "\")
$progressIndex = 0

# Build with verbose output to show progress
Write-Host "Compiling..." -NoNewline
$buildJob = Start-Job -ScriptBlock {
    param($ldflags, $output, $projectRoot)
    Set-Location $projectRoot
    go build -v -ldflags $ldflags -o $output .
} -ArgumentList $ldflags, $BUILD_OUTPUT, $PROJECT_ROOT

# Show progress while building
while ($buildJob.State -eq "Running") {
    Write-Host "`rCompiling... $($progressChars[$progressIndex])" -NoNewline
    $progressIndex = ($progressIndex + 1) % 4
    Start-Sleep -Milliseconds 250
}

# Wait for job to complete and get result
$result = Receive-Job -Job $buildJob
Remove-Job -Job $buildJob -Force

# Show completed output
Write-Host "`rCompiling... Done" -ForegroundColor Green
if ($result) {
    $result | ForEach-Object { Write-Host "  $_" -ForegroundColor DarkGray }
}

if ($LASTEXITCODE -ne 0) {
    Write-Host " Build failed" -ForegroundColor Red
    Exit-WithPause 1
}

if (Test-Path $BUILD_OUTPUT) {
    $size = [math]::Round((Get-Item $BUILD_OUTPUT).Length / 1MB, 2)
    Write-Host " Build successful!" -ForegroundColor Green
    Write-Host " Output: $BUILD_OUTPUT" -ForegroundColor Green
    Write-Host " Size: ${size}MB" -ForegroundColor Green
    Write-Host ""
} else {
    Write-Host " Build failed: executable not found" -ForegroundColor Red
    Exit-WithPause 1
}

# Run tests if not skipped
if (-not $SkipTests) {
    Write-Section "Tests"
    Write-Host "Running tests..." -ForegroundColor Yellow
    go test -v ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host " Some tests failed" -ForegroundColor Yellow
        Write-Host ""
    } else {
        Write-Host " All tests passed" -ForegroundColor Green
        Write-Host ""
    }
}

# Create Start Menu shortcut
try {
    $startMenuPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\VideoTools"
    if (-not (Test-Path $startMenuPath)) {
        New-Item -ItemType Directory -Path $startMenuPath -Force | Out-Null
    }

    $appShortcut = Join-Path $startMenuPath "VideoTools.lnk"
    
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($appShortcut)
    $shortcut.TargetPath = $BUILD_OUTPUT
    $shortcut.WorkingDirectory = $PROJECT_ROOT
    $shortcut.Save()
    
    Write-Host "Start menu shortcut created" -ForegroundColor Green
} catch {
    Write-Host "Failed to create shortcut: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Section "Complete"
Write-Host "VideoTools build completed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "To run the application:" -ForegroundColor Cyan
Write-Host "  .\VideoTools.exe" -ForegroundColor White
Write-Host ""
Exit-WithPause 0