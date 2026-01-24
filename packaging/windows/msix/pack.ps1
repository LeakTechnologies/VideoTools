# VideoTools MSIX packaging helper

param(
    [string]$InputExe = "dist/windows/VideoTools.exe",
    [string]$Version = "0.1.1.0",
    [string]$Publisher = "CN=Leak Technologies",
    [string]$OutDir = "dist/windows/msix"
)

$ErrorActionPreference = "Stop"

function Resolve-MakeAppx {
    $cmd = Get-Command makeappx.exe -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Path
    }
    $kitsRoot = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (-not (Test-Path $kitsRoot)) {
        throw "MakeAppx not found. Install Windows 10/11 SDK."
    }
    $candidates = Get-ChildItem -Path $kitsRoot -Directory | Sort-Object Name -Descending
    foreach ($dir in $candidates) {
        $path = Join-Path $dir.FullName "x64\makeappx.exe"
        if (Test-Path $path) {
            return $path
        }
    }
    throw "MakeAppx not found under Windows Kits."
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..\..")
$inputPath = Join-Path $repoRoot $InputExe
$outPath = Join-Path $repoRoot $OutDir

if (-not (Test-Path $inputPath)) {
    throw "Input exe not found: $inputPath"
}

$layoutDir = Join-Path $outPath "layout"
$assetsDir = Join-Path $layoutDir "Assets"
$manifestSrc = Join-Path $PSScriptRoot "AppxManifest.xml"
$manifestDest = Join-Path $layoutDir "AppxManifest.xml"
$iconSrc = Join-Path $PSScriptRoot "..\..\..\assets\logo\VT_Icon.png"

New-Item -ItemType Directory -Force -Path $layoutDir | Out-Null
New-Item -ItemType Directory -Force -Path $assetsDir | Out-Null

Copy-Item $inputPath -Destination (Join-Path $layoutDir "VideoTools.exe") -Force

if (-not (Test-Path $iconSrc)) {
    throw "Icon not found: $iconSrc"
}

Copy-Item $iconSrc -Destination (Join-Path $assetsDir "StoreLogo.png") -Force
Copy-Item $iconSrc -Destination (Join-Path $assetsDir "Square44x44Logo.png") -Force
Copy-Item $iconSrc -Destination (Join-Path $assetsDir "Square150x150Logo.png") -Force
Copy-Item $iconSrc -Destination (Join-Path $assetsDir "Wide310x150Logo.png") -Force

$manifest = Get-Content $manifestSrc -Raw
$manifest = $manifest -replace 'Version="[^"]+"', "Version=`"$Version`""
$manifest = $manifest -replace 'Publisher="[^"]+"', "Publisher=`"$Publisher`""
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($manifestDest, $manifest, $utf8NoBom)

$makeAppx = Resolve-MakeAppx
New-Item -ItemType Directory -Force -Path $outPath | Out-Null
$packagePath = Join-Path $outPath "VideoTools.msix"

& $makeAppx pack /d $layoutDir /p $packagePath /o
if ($LASTEXITCODE -ne 0) {
    throw "MakeAppx failed with exit code $LASTEXITCODE"
}

Write-Host "[OK] MSIX created at $packagePath" -ForegroundColor Green
