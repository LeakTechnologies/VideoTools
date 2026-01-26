param()

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[INFO]  Elevation required for Windows dependencies (GStreamer MSI)." -ForegroundColor Yellow
    Write-Host "        Approve the UAC prompt to continue." -ForegroundColor Yellow
    try {
        $args = @(
            "-NoProfile",
            "-NoExit",
            "-ExecutionPolicy", "Bypass",
            "-File", $PSCommandPath
        )
        Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList $args -WorkingDirectory $PSScriptRoot | Out-Null
    } catch {
        Write-Host "[ERROR]  Failed to prompt for elevation." -ForegroundColor Red
        Write-Host "        Run this script from an Administrator PowerShell." -ForegroundColor Yellow
        exit 1
    }
    exit 0
}

& "$PSScriptRoot\_internal\install-deps-windows.ps1"
exit $LASTEXITCODE
