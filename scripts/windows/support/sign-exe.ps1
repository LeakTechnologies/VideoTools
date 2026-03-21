# VideoTools EXE signing helper
#
# Supports two modes (tried in order):
#   1. Azure Trusted Signing (cloud, no hardware token, immediate SAC trust)
#      Required env vars: AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET,
#                         VT_AZURE_SIGNING_ENDPOINT, VT_AZURE_SIGNING_ACCOUNT,
#                         VT_AZURE_SIGNING_PROFILE
#
#   2. PFX certificate (traditional, requires a certificate file)
#      Required params:  -CertPath, -CertPassword

param(
    [string]$ExePath      = "",
    [string]$CertPath     = "",
    [string]$CertPassword = "",
    [string]$TimestampUrl = "http://timestamp.acs.microsoft.com"
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

function Resolve-DefaultExe {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    return (Join-Path $projectRoot "VideoTools.exe")
}

function Resolve-Signtool {
    $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Path }
    $kitsRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (-not (Test-Path $kitsRoot)) { return $null }
    $candidates = Get-ChildItem -Path $kitsRoot -Directory | Sort-Object Name -Descending
    foreach ($dir in $candidates) {
        $path = Join-Path $dir.FullName "x64\signtool.exe"
        if (Test-Path $path) { return $path }
    }
    return $null
}

function Install-TrustedSigningDlib {
    # Install the Microsoft Trusted Signing Client NuGet package into a temp dir
    # and return the path to Azure.CodeSigning.Dlib.dll (x64).
    $pkgDir = Join-Path $env:TEMP "vt-trusted-signing"
    New-Item -ItemType Directory -Force -Path $pkgDir | Out-Null

    $nugetUrl = "https://dist.nuget.org/win-x86-commandline/latest/nuget.exe"
    $nugetExe = Join-Path $pkgDir "nuget.exe"
    if (-not (Test-Path $nugetExe)) {
        Write-Host "[sign] Downloading nuget.exe..." -ForegroundColor Cyan
        Invoke-WebRequest -Uri $nugetUrl -OutFile $nugetExe -UseBasicParsing
    }

    $pkgName    = "Microsoft.Trusted.Signing.Client"
    $pkgVersion = "1.0.60"   # pin a known-good version; bump as needed
    $pkgOut     = Join-Path $pkgDir "packages"
    if (-not (Test-Path (Join-Path $pkgOut "$pkgName.$pkgVersion"))) {
        Write-Host "[sign] Installing $pkgName $pkgVersion..." -ForegroundColor Cyan
        & $nugetExe install $pkgName -Version $pkgVersion -OutputDirectory $pkgOut -NonInteractive | Out-Null
    }

    $dlib = Get-ChildItem -Path $pkgOut -Recurse -Filter "Azure.CodeSigning.Dlib.dll" |
        Where-Object { $_.DirectoryName -match "x64" } |
        Select-Object -First 1
    if (-not $dlib) {
        throw "Azure.CodeSigning.Dlib.dll not found after NuGet install."
    }
    return $dlib.FullName
}

# ---------------------------------------------------------------------------
# Resolve exe path
# ---------------------------------------------------------------------------

$exe = $ExePath
if ([string]::IsNullOrWhiteSpace($exe)) { $exe = Resolve-DefaultExe }
if (-not (Test-Path $exe)) { throw "EXE not found: $exe" }

$signtool = Resolve-Signtool
if (-not $signtool) { throw "signtool.exe not found. Install the Windows SDK." }

# ---------------------------------------------------------------------------
# Mode 1: Azure Trusted Signing
# ---------------------------------------------------------------------------

$azureEndpoint = $env:VT_AZURE_SIGNING_ENDPOINT
$azureAccount  = $env:VT_AZURE_SIGNING_ACCOUNT
$azureProfile  = $env:VT_AZURE_SIGNING_PROFILE
$azureTenant   = $env:AZURE_TENANT_ID
$azureClient   = $env:AZURE_CLIENT_ID
$azureSecret   = $env:AZURE_CLIENT_SECRET

if ($azureEndpoint -and $azureAccount -and $azureProfile -and $azureTenant -and $azureClient -and $azureSecret) {
    Write-Host "[sign] Mode: Azure Trusted Signing" -ForegroundColor Cyan

    $dlibPath = Install-TrustedSigningDlib

    # Write metadata JSON expected by the DLIB
    $metaFile = Join-Path $env:TEMP "vt-trusted-signing-metadata.json"
    @{
        Endpoint               = $azureEndpoint
        CodeSigningAccountName = $azureAccount
        CertificateProfileName = $azureProfile
    } | ConvertTo-Json | Out-File -FilePath $metaFile -Encoding UTF8

    # The DLIB reads these standard Azure SDK env vars for auth
    $env:AZURE_TENANT_ID     = $azureTenant
    $env:AZURE_CLIENT_ID     = $azureClient
    $env:AZURE_CLIENT_SECRET = $azureSecret

    & $signtool sign /fd SHA256 /tr $TimestampUrl /td SHA256 /dlib $dlibPath /dmdf $metaFile $exe

    if ($LASTEXITCODE -ne 0) { throw "signtool (Azure Trusted Signing) failed with exit code $LASTEXITCODE" }
    Write-Host "[OK] Signed $exe (Azure Trusted Signing)" -ForegroundColor Green
    return
}

# ---------------------------------------------------------------------------
# Mode 2: PFX certificate
# ---------------------------------------------------------------------------

if ([string]::IsNullOrWhiteSpace($CertPath)) {
    throw "No signing credentials found. Set Azure Trusted Signing env vars or pass -CertPath."
}
if (-not (Test-Path $CertPath)) { throw "Cert not found: $CertPath" }
if ([string]::IsNullOrWhiteSpace($CertPassword)) { throw "CertPassword is required for PFX signing." }

Write-Host "[sign] Mode: PFX certificate" -ForegroundColor Cyan
& $signtool sign /f $CertPath /p $CertPassword /fd SHA256 /tr $TimestampUrl /td SHA256 $exe
if ($LASTEXITCODE -ne 0) { throw "signtool (PFX) failed with exit code $LASTEXITCODE" }
Write-Host "[OK] Signed $exe (PFX)" -ForegroundColor Green
