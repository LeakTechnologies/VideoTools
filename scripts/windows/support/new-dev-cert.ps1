# VideoTools self-signed dev cert generator (PFX + base64)

param(
    [string]$Subject = "CN=VideoTools Dev",
    [string]$OutPath = "",
    [string]$Password = "",
    [switch]$InstallCert
)

$ErrorActionPreference = "Stop"

function Resolve-DefaultOut {
    $projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
    return (Join-Path $projectRoot "packaging\windows\msix\VideoToolsDev.pfx")
}

if ([string]::IsNullOrWhiteSpace($OutPath)) {
    $OutPath = Resolve-DefaultOut
}
if ([string]::IsNullOrWhiteSpace($Password)) {
    $Password = [Guid]::NewGuid().ToString("N")
}

$cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject $Subject -CertStoreLocation Cert:\CurrentUser\My -KeyExportPolicy Exportable -KeySpec Signature -KeyAlgorithm RSA -KeyLength 2048 -HashAlgorithm SHA256
$pwd = ConvertTo-SecureString -String $Password -Force -AsPlainText
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutPath) | Out-Null
Export-PfxCertificate -Cert $cert -FilePath $OutPath -Password $pwd | Out-Null

if ($InstallCert) {
    $store = New-Object System.Security.Cryptography.X509Certificates.X509Store("TrustedPeople","CurrentUser")
    $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
    $store.Add($cert)
    $store.Close()
}

$bytes = [System.IO.File]::ReadAllBytes($OutPath)
$base64 = [System.Convert]::ToBase64String($bytes)

Write-Host "PFX: $OutPath"
Write-Host "Password: $Password"
Write-Host "Base64:"
Write-Output $base64
