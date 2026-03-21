# VideoTools EXE signing helper (optional)

param(
    [string]$ExePath = "",
    [string]$CertPath = "",
    [string]$CertPassword = "",
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

function Resolve-DefaultExe {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    return (Join-Path $projectRoot "VideoTools.exe")
}

$exe = $ExePath
if ([string]::IsNullOrWhiteSpace($exe)) {
    $exe = Resolve-DefaultExe
}
if (-not (Test-Path $exe)) {
    throw "EXE not found: $exe"
}
if ([string]::IsNullOrWhiteSpace($CertPath)) {
    throw "CertPath is required (PFX file)."
}
if (-not (Test-Path $CertPath)) {
    throw "Cert not found: $CertPath"
}
if ([string]::IsNullOrWhiteSpace($CertPassword)) {
    throw "CertPassword is required."
}

$signTool = Get-Command signtool.exe -ErrorAction SilentlyContinue
if (-not $signTool) {
    throw "signtool.exe not found. Install Windows SDK." 
}

& $signTool.Path sign /f $CertPath /p $CertPassword /fd SHA256 /tr $TimestampUrl /td SHA256 $exe
if ($LASTEXITCODE -ne 0) {
    throw "signtool failed with exit code $LASTEXITCODE"
}

Write-Host "[OK] Signed $exe" -ForegroundColor Green
