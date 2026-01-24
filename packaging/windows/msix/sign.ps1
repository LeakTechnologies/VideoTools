# VideoTools MSIX signing helper

param(
    [string]$MsixPath = "dist/windows/msix/VideoTools.msix",
    [string]$PfxPath = "packaging/windows/msix/VideoToolsDev.pfx",
    [string]$PfxPassword = "devpass",
    [string]$SignToolPath = "",
    [switch]$CreateDevCert,
    [switch]$InstallCert
)

$ErrorActionPreference = "Stop"

function Resolve-SignTool {
    $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Path
    }
    $kitsRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (-not (Test-Path $kitsRoot)) {
        throw "SignTool not found. Install Windows 10/11 SDK."
    }
    $candidates = Get-ChildItem -Path $kitsRoot -Directory | Sort-Object Name -Descending
    foreach ($dir in $candidates) {
        $path = Join-Path $dir.FullName "x64\signtool.exe"
        if (Test-Path $path) {
            return $path
        }
    }
    throw "SignTool not found under Windows Kits."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..\..")
$msixFullPath = Join-Path $repoRoot $MsixPath
$pfxFullPath = Join-Path $repoRoot $PfxPath

if (-not (Test-Path $msixFullPath)) {
    throw "MSIX not found: $msixFullPath"
}

if ($CreateDevCert) {
    $cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject "CN=Leak Technologies Dev" -CertStoreLocation Cert:\CurrentUser\My
    $pwd = ConvertTo-SecureString -String $PfxPassword -Force -AsPlainText
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $pfxFullPath) | Out-Null
    Export-PfxCertificate -Cert $cert -FilePath $pfxFullPath -Password $pwd | Out-Null

    if ($InstallCert) {
        $store = New-Object System.Security.Cryptography.X509Certificates.X509Store("TrustedPeople","CurrentUser")
        $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
        $store.Add($cert)
        $store.Close()
    }
}

if (-not (Test-Path $pfxFullPath)) {
    throw "PFX not found: $pfxFullPath (use -CreateDevCert)"
}

$signTool = $SignToolPath
if (-not $signTool) {
    $signTool = Resolve-SignTool
}
& $signTool sign /fd SHA256 /td SHA256 /a /f $pfxFullPath /p $PfxPassword $msixFullPath
if ($LASTEXITCODE -ne 0) {
    throw "SignTool failed with exit code $LASTEXITCODE"
}

Write-Host "[OK] Signed MSIX: $msixFullPath" -ForegroundColor Green
