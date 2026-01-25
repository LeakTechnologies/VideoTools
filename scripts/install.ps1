param()

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[INFO]  Elevation required for Windows dependencies (GStreamer MSI)." -ForegroundColor Yellow
    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", "`"$PSScriptRoot\install.ps1`""
    )
    Start-Process -FilePath "powershell" -Verb RunAs -ArgumentList $args | Out-Null
    exit 0
}

& "$PSScriptRoot\install-deps-windows.ps1"
exit $LASTEXITCODE
