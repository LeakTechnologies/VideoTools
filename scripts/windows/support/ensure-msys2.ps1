# VideoTools MSYS2 provisioning (repo-local by default)

param(
    [string]$Root = "",
    [string]$Flavor = "",
    [string[]]$Packages = @("base-devel", "mingw-w64-ucrt-x86_64-toolchain"),
    [switch]$SkipUpdate = $false
)

$ErrorActionPreference = "Stop"

function Resolve-ProjectRoot {
    return Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
}

function Resolve-Msys2Root {
    if ($env:VT_MSYS2_ROOT) {
        return $env:VT_MSYS2_ROOT
    }
    if (-not [string]::IsNullOrWhiteSpace($Root)) {
        return $Root
    }
    $projectRoot = Resolve-ProjectRoot
    return (Join-Path $projectRoot "Tools\\msys64")
}

function Resolve-Flavor {
    if ($env:VT_MSYS2_FLAVOR) {
        return $env:VT_MSYS2_FLAVOR
    }
    if (-not [string]::IsNullOrWhiteSpace($Flavor)) {
        return $Flavor
    }
    return "ucrt64"
}

function Write-Info {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Cyan
}

function Ensure-Msys2Root {
    param([string]$Msys2Root)
    if (Test-Path (Join-Path $Msys2Root "usr\\bin\\bash.exe")) {
        return
    }

    $targetRoot = Split-Path -Parent $Msys2Root
    if (-not (Test-Path $targetRoot)) {
        New-Item -ItemType Directory -Force -Path $targetRoot | Out-Null
    }

    $archiveUrl = "https://github.com/msys2/msys2-installer/releases/latest/download/msys2-base-x86_64.tar.xz"
    $archivePath = Join-Path $env:TEMP "msys2-base-x86_64.tar.xz"

    Write-Info "Downloading MSYS2 base..."
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath -UseBasicParsing

    Write-Info "Extracting MSYS2 base to $targetRoot..."
    & tar -xf $archivePath -C $targetRoot
    Remove-Item -Force $archivePath

    if (-not (Test-Path (Join-Path $Msys2Root "usr\\bin\\bash.exe"))) {
        throw "MSYS2 extraction failed: $Msys2Root"
    }
}

function Invoke-Msys2 {
    param(
        [string]$Msys2Root,
        [string]$Flavor,
        [string]$Command
    )

    $bashPath = Join-Path $Msys2Root "usr\\bin\\bash.exe"
    if (-not (Test-Path $bashPath)) {
        throw "MSYS2 bash not found: $bashPath"
    }

    $oldMsystem = $env:MSYSTEM
    $oldChere = $env:CHERE_INVOKING
    $oldPath = $env:Path
    $env:MSYSTEM = $Flavor.ToUpper()
    $env:CHERE_INVOKING = "1"
    $env:Path = (Join-Path $Msys2Root "usr\\bin") + ";" + $env:Path
    try {
        & $bashPath -lc $Command
        if ($LASTEXITCODE -ne 0) {
            throw "MSYS2 command failed: $Command"
        }
    } finally {
        $env:MSYSTEM = $oldMsystem
        $env:CHERE_INVOKING = $oldChere
        $env:Path = $oldPath
    }
}

function Write-Manifest {
    param(
        [string]$Msys2Root,
        [string]$Flavor,
        [string[]]$Packages
    )
    $projectRoot = Resolve-ProjectRoot
    $manifestPath = Join-Path $projectRoot "Tools\\msys2-packages.lock"
    $content = @(
        "root=$Msys2Root",
        "flavor=$Flavor",
        "packages=$($Packages -join ' ')"
    ) -join "`n"
    Set-Content -Path $manifestPath -Value $content -Encoding ASCII
}

$msys2Root = Resolve-Msys2Root
$flavorValue = Resolve-Flavor

Ensure-Msys2Root -Msys2Root $msys2Root

if (-not $SkipUpdate) {
    Write-Info "Updating MSYS2 package database..."
    Invoke-Msys2 -Msys2Root $msys2Root -Flavor $flavorValue -Command "pacman -Sy --noconfirm --noprogressbar"
}

if ($Packages -and $Packages.Count -gt 0) {
    $pkgList = $Packages -join " "
    Write-Info "Installing MSYS2 packages: $pkgList"
    Invoke-Msys2 -Msys2Root $msys2Root -Flavor $flavorValue -Command "pacman -S --needed --noconfirm $pkgList"
}

Write-Manifest -Msys2Root $msys2Root -Flavor $flavorValue -Packages $Packages

Write-Output $msys2Root
