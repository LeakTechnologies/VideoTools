# VideoTools: Migrate from Scoop MinGW to MSYS2
# This script helps users switch from problematic Scoop MinGW to MSYS2

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Toolchain Migration" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "This script will:" -ForegroundColor Yellow
Write-Host "1. Remove Scoop MinGW (if problematic)" -ForegroundColor White
Write-Host "2. Install MSYS2 (recommended)" -ForegroundColor White
Write-Host "3. Update build script paths" -ForegroundColor White
Write-Host ""

$confirm = Read-Host "Proceed with migration? (y/N)"
if ($confirm -ne "y" -and $confirm -ne "Y") {
    Write-Host "Migration cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host "Removing Scoop MinGW..." -ForegroundColor Yellow
try {
    scoop uninstall mingw
    Write-Host "[OK]  Scoop MinGW removed" -ForegroundColor Green
} catch {
    Write-Host "[WARN]  Could not remove Scoop MinGW: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "Installing MSYS2..." -ForegroundColor Yellow
$installScript = Join-Path $PSScriptRoot "_internal/install-deps-windows.ps1"
$installArgs = @(
    "-SkipFFmpeg",
    "-SkipGStreamer", 
    "-InstallBuildTools",
    "-UseMSYS2",
    "-SkipPython",
    "-SkipDvdStyler",
    "-SkipWhisper"
)

& powershell -ExecutionPolicy Bypass -File $installScript @installArgs

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host " Migration complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Restart your terminal/PowerShell" -ForegroundColor White
Write-Host "2. Test build: .\build.ps1" -ForegroundColor White
Write-Host ""
Write-Host "Press any key to continue..." -ForegroundColor Cyan
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")