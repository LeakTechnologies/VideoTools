# Add Windows Defender Exclusions for VideoTools Build Performance
# This script adds build directories to Windows Defender exclusions
# Saves 2-5 minutes on build times!

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "❌ ERROR: This script must be run as Administrator!" -ForegroundColor Red
    Write-Host ""
    Write-Host "To run as Administrator:" -ForegroundColor Yellow
    Write-Host "  1. Right-click PowerShell" -ForegroundColor White
    Write-Host "  2. Select 'Run as Administrator'" -ForegroundColor White
    Write-Host "  3. Navigate to this directory" -ForegroundColor White
    Write-Host "  4. Run: .\scripts\add-defender-exclusions.ps1" -ForegroundColor White
    Write-Host ""
    Write-Host "Or from Git Bash (as Administrator):" -ForegroundColor Yellow
    Write-Host "  powershell.exe -ExecutionPolicy Bypass -File ./scripts/add-defender-exclusions.ps1" -ForegroundColor White
    exit 1
}

Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  Adding Windows Defender Exclusions for VideoTools" -ForegroundColor Cyan
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""

# Get paths
$goBuildCache = "$env:LOCALAPPDATA\go-build"
$goModCache = "$env:USERPROFILE\go"
$projectDir = Split-Path -Parent $PSScriptRoot
$msys64 = "C:\msys64"

Write-Host "Adding exclusions..." -ForegroundColor Yellow
Write-Host ""

# Add Go build cache
try {
    Add-MpPreference -ExclusionPath $goBuildCache -ErrorAction Stop
    Write-Host "✓ Added: $goBuildCache" -ForegroundColor Green
} catch {
    Write-Host "⚠ Already excluded or failed: $goBuildCache" -ForegroundColor Yellow
}

# Add Go module cache
try {
    Add-MpPreference -ExclusionPath $goModCache -ErrorAction Stop
    Write-Host "✓ Added: $goModCache" -ForegroundColor Green
} catch {
    Write-Host "⚠ Already excluded or failed: $goModCache" -ForegroundColor Yellow
}

# Add project directory
try {
    Add-MpPreference -ExclusionPath $projectDir -ErrorAction Stop
    Write-Host "✓ Added: $projectDir" -ForegroundColor Green
} catch {
    Write-Host "⚠ Already excluded or failed: $projectDir" -ForegroundColor Yellow
}

# Add MSYS2 if it exists
if (Test-Path $msys64) {
    try {
        Add-MpPreference -ExclusionPath $msys64 -ErrorAction Stop
        Write-Host "✓ Added: $msys64" -ForegroundColor Green
    } catch {
        Write-Host "⚠ Already excluded or failed: $msys64" -ForegroundColor Yellow
    }
} else {
    Write-Host "⊘ Skipped: $msys64 (not found)" -ForegroundColor Gray
}

Write-Host ""
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "✅ EXCLUSIONS ADDED" -ForegroundColor Green
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""
Write-Host "Expected build time improvement: 5+ minutes → 30-90 seconds" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Close and reopen your terminal" -ForegroundColor White
Write-Host "  2. Run: ./scripts/windows/build.ps1 (PowerShell) or ./scripts/windows/build.bat" -ForegroundColor White
Write-Host "  3. Or from Git Bash: ./scripts/linux/build-linux.sh" -ForegroundColor White
Write-Host ""
