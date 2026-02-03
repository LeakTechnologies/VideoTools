param()

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Windows Installation" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

& "$PSScriptRoot\..\..\windows\support\install-deps-windows.ps1"
exit $LASTEXITCODE
