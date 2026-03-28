# VideoTools EXE signing helper
#
# Supports two modes (tried in order):
#
#   1. SignPath.io (free for open-source projects — https://signpath.io/product/open-source)
#      No certificate to manage; SignPath holds the cert and signs via their API.
#      Required env vars: SIGNPATH_API_TOKEN, SIGNPATH_ORGANIZATION_ID
#      Required SignPath setup:
#        - Project slug:             videotools
#        - Signing policy slug:      release-signing
#        - Artifact config slug:     exe
#        (configure these once at app.signpath.io after OSS approval)
#
#   2. PFX certificate (fallback — for local dev / self-signed testing)
#      Required params: -CertPath, -CertPassword

param(
    [string]$ExePath      = "",
    [string]$CertPath     = "",
    [string]$CertPassword = "",
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

function Resolve-DefaultExe {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    return (Join-Path $projectRoot "VideoTools.exe")
}

$exe = $ExePath
if ([string]::IsNullOrWhiteSpace($exe)) { $exe = Resolve-DefaultExe }
if (-not (Test-Path $exe)) { throw "EXE not found: $exe" }

# ---------------------------------------------------------------------------
# Mode 1: SignPath.io
# ---------------------------------------------------------------------------
$spToken  = $env:SIGNPATH_API_TOKEN
$spOrgId  = $env:SIGNPATH_ORGANIZATION_ID

if ($spToken -and $spOrgId) {
    Write-Host "[sign] Mode: SignPath.io" -ForegroundColor Cyan

    # Install the SignPath PowerShell module if not already present
    if (-not (Get-Module -ListAvailable -Name SignPath)) {
        Write-Host "[sign] Installing SignPath PowerShell module..." -ForegroundColor Cyan
        Install-Module -Name SignPath -Force -Scope CurrentUser -AllowClobber
    }
    Import-Module SignPath -Force

    Submit-SigningRequest `
        -ApiToken          $spToken `
        -OrganizationId    $spOrgId `
        -ProjectSlug       "videotools" `
        -SigningPolicySlug "release-signing" `
        -InputArtifactPath $exe `
        -OutputArtifactPath $exe `
        -WaitForCompletion

    Write-Host "[OK] Signed $exe (SignPath.io)" -ForegroundColor Green
    return
}

# ---------------------------------------------------------------------------
# Mode 2: PFX certificate
# ---------------------------------------------------------------------------
if ([string]::IsNullOrWhiteSpace($CertPath)) {
    throw "No signing credentials found. Set SIGNPATH_API_TOKEN + SIGNPATH_ORGANIZATION_ID, or pass -CertPath."
}
if (-not (Test-Path $CertPath)) { throw "Cert not found: $CertPath" }
if ([string]::IsNullOrWhiteSpace($CertPassword)) { throw "-CertPassword is required for PFX signing." }

$signToolPath = ""
$signToolCmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
if ($signToolCmd) {
    $signToolPath = $signToolCmd.Path
} else {
    # signtool.exe is typically not in PATH; search Windows Kits
    $kitsRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (Test-Path $kitsRoot) {
        $candidates = Get-ChildItem -Path $kitsRoot -Directory | Sort-Object Name -Descending
        foreach ($dir in $candidates) {
            $candidate = Join-Path $dir.FullName "x64\signtool.exe"
            if (Test-Path $candidate) { $signToolPath = $candidate; break }
        }
    }
}
if (-not $signToolPath) { throw "signtool.exe not found. Install the Windows SDK." }

Write-Host "[sign] Mode: PFX certificate" -ForegroundColor Cyan
& $signToolPath sign /f $CertPath /p $CertPassword /fd SHA256 /tr $TimestampUrl /td SHA256 $exe
if ($LASTEXITCODE -ne 0) { throw "signtool failed with exit code $LASTEXITCODE" }
Write-Host "[OK] Signed $exe (PFX)" -ForegroundColor Green
