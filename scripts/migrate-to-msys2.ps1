# VideoTools: Ensure MSYS2 toolchain
# This script installs the MSYS2 MinGW-w64 toolchain for Windows builds

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Toolchain Setup" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "This script will:" -ForegroundColor Yellow
Write-Host "1. Install MSYS2 (if missing)" -ForegroundColor White
Write-Host "2. Install MinGW-w64 GCC via MSYS2" -ForegroundColor White
Write-Host ""

$confirm = Read-Host "Proceed with MSYS2 setup? (y/N)"
if ($confirm -ne "y" -and $confirm -ne "Y") {
    Write-Host "Setup cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host "Installing MSYS2 toolchain..." -ForegroundColor Yellow
$installScript = Join-Path $PSScriptRoot "_internal/install-deps-windows.ps1"
$installArgs = @(
    "-SkipFFmpeg",
    "-SkipGStreamer",
    "-InstallBuildTools",
    "-SkipPython",
    "-SkipDvdStyler",
    "-SkipWhisper"
)

& powershell -ExecutionPolicy Bypass -File $installScript @installArgs

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host " Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Restart your terminal/PowerShell" -ForegroundColor White
Write-Host "2. Test build: .\build.ps1" -ForegroundColor White
Write-Host ""
Write-Host "Press any key to continue..." -ForegroundColor Cyan
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
