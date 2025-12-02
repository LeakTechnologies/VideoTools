# VideoTools Build Script for Windows
# Builds the VideoTools application with proper error handling

param(
    [switch]$Clean = $false,
    [switch]$SkipTests = $false
)

Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  VideoTools Build Script (Windows)" -ForegroundColor Cyan
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Get project root (parent of scripts directory)
$PROJECT_ROOT = Split-Path -Parent $PSScriptRoot
$BUILD_OUTPUT = Join-Path $PROJECT_ROOT "VideoTools.exe"

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "❌ ERROR: Go is not installed. Please run install-deps-windows.ps1 first." -ForegroundColor Red
    exit 1
}

Write-Host "📦 Go version:" -ForegroundColor Green
go version
Write-Host ""

# Change to project directory
Set-Location $PROJECT_ROOT

if ($Clean) {
    Write-Host "🧹 Cleaning previous builds and cache..." -ForegroundColor Yellow
    go clean -cache -modcache -testcache 2>$null
    if (Test-Path $BUILD_OUTPUT) {
        Remove-Item $BUILD_OUTPUT -Force
    }
    Write-Host "✓ Cache cleaned" -ForegroundColor Green
    Write-Host ""
}

Write-Host "⬇️  Downloading and verifying dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to download dependencies" -ForegroundColor Red
    exit 1
}

go mod verify
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to verify dependencies" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Dependencies verified" -ForegroundColor Green
Write-Host ""

Write-Host "🔨 Building VideoTools..." -ForegroundColor Yellow
Write-Host ""

# Fyne needs CGO for GLFW/OpenGL bindings
$env:CGO_ENABLED = "1"

# Build the application
go build -o $BUILD_OUTPUT .

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build successful!" -ForegroundColor Green
    Write-Host ""
    Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host "✅ BUILD COMPLETE" -ForegroundColor Green
    Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host ""

    # Get file size
    $fileSize = (Get-Item $BUILD_OUTPUT).Length
    $fileSizeMB = [math]::Round($fileSize / 1MB, 2)

    Write-Host "Output: $BUILD_OUTPUT" -ForegroundColor White
    Write-Host "Size: $fileSizeMB MB" -ForegroundColor White
    Write-Host ""
    Write-Host "To run:" -ForegroundColor Yellow
    Write-Host "  .\VideoTools.exe" -ForegroundColor White
    Write-Host ""

    # Check if ffmpeg is available
    if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
        Write-Host "⚠️  Warning: ffmpeg not found in PATH" -ForegroundColor Yellow
        Write-Host "   VideoTools requires ffmpeg to convert videos" -ForegroundColor Yellow
        Write-Host "   Run: .\scripts\install-deps-windows.ps1" -ForegroundColor Yellow
        Write-Host ""
    }

    # Check for NVIDIA GPU
    try {
        $nvidiaGpu = Get-WmiObject Win32_VideoController | Where-Object { $_.Name -like "*NVIDIA*" }
        if ($nvidiaGpu) {
            Write-Host "🎮 NVIDIA GPU detected: $($nvidiaGpu.Name)" -ForegroundColor Green
            Write-Host "   Hardware encoding (NVENC) will be available" -ForegroundColor Green
            Write-Host ""
        }
    } catch {
        # GPU detection failed, not critical
    }

} else {
    Write-Host "❌ Build failed!" -ForegroundColor Red
    exit 1
}
