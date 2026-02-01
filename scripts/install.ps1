param()
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[INFO]  Elevation required for Windows dependencies (GStreamer MSI)." -ForegroundColor Yellow
    Write-Host "        Approve the UAC prompt to continue." -ForegroundColor Yellow
    try {
        $passThrough = @()
        if ($args.Count -gt 0) {
            $passThrough += $args
        }
        $args = @(
            "-NoProfile",
            "-ExecutionPolicy", "Bypass",
            "-File", $PSCommandPath
        ) + $passThrough
        Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList $args -WorkingDirectory $PSScriptRoot | Out-Null
    } catch {
        Write-Host "[ERROR]  Failed to prompt for elevation." -ForegroundColor Red
        Write-Host "        Run this script from an Administrator PowerShell." -ForegroundColor Yellow
        exit 1
    }
    exit 0
}
Write-Host "[INFO]  Build tools are provided via MSYS2 (MinGW-w64)." -ForegroundColor Cyan
& "$PSScriptRoot\_internal\install-deps-windows.ps1" @args
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
    Write-Host "[ERROR]  Install failed with exit code $exitCode." -ForegroundColor Red
    Write-Host "Press any key to close..." -ForegroundColor Cyan
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}
exit $exitCode



