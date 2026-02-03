# VideoTools Build Script for Windows
# Builds the VideoTools application with proper error handling

param(
    [switch]$Clean = $false,
    [switch]$SkipTests = $false
)

function Write-Header {
    param(
        [string]$Title
    )
    $line = "════════════════════════════════════════════════════════════════"
    Write-Host $line -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host $line -ForegroundColor Cyan
    Write-Host ""
}

function Write-Section {
    param(
        [string]$Title
    )
    Write-Host "===============================================================" -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host "===============================================================" -ForegroundColor Cyan
    Write-Host ""
}

function Test-Command {
    param([string]$Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Refresh-Path {
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
}

function Wait-ForKey {
    param(
        [string]$Message = "Press any key to close..."
    )
    if ($env:CI) {
        return
    }
    try {
        if ($Host -and $Host.Name -eq "ConsoleHost" -and $Host.UI -and $Host.UI.RawUI) {
            Write-Host $Message -ForegroundColor Cyan
            $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        }
    } catch {
        # Ignore pause failures in non-interactive shells.
    }
}

function Exit-WithPause {
    param(
        [int]$Code = 0
    )
    Wait-ForKey
    exit $Code
}

function Find-Msys2Root {
    if ($env:VT_MSYS2_ROOT) {
        return $env:VT_MSYS2_ROOT
    }
    $repoLocal = Join-Path (Split-Path -Parent $PSScriptRoot) "Tools\\msys64"
    if (Test-Path (Join-Path $repoLocal "usr\\bin\\bash.exe")) {
        return $repoLocal
    }
    $candidates = @(
        "C:\\msys64",
        "C:\\msys2",
        (Join-Path $env:LOCALAPPDATA "Programs\\MSYS2"),
        (Join-Path $env:ProgramFiles "MSYS2")
    )
    foreach ($root in $candidates) {
        if (-not $root) { continue }
        $gccPath = Join-Path $root "ucrt64\\bin\\gcc.exe"
        if (Test-Path $gccPath) {
            return $root
        }
    }
    return $null
}

function Resolve-Msys2Root {
    $root = Find-Msys2Root
    if ($root) {
        return $root
    }
    return (Join-Path (Split-Path -Parent $PSScriptRoot) "Tools\\msys64")
}

function Resolve-Msys2Flavor {
    if ($env:VT_MSYS2_FLAVOR) {
        return $env:VT_MSYS2_FLAVOR
    }
    return "ucrt64"
}

function Ensure-Msys2Toolchain {
    param(
        [string]$Msys2Root
    )
    $flavor = Resolve-Msys2Flavor
    $ensureScript = Join-Path $PSScriptRoot "support\\ensure-msys2.ps1"
    if (-not (Test-Path $ensureScript)) {
        Write-Host " ensure-msys2.ps1 not found; skipping auto-provision." -ForegroundColor Yellow
        return $false
    }
    try {
        & $ensureScript -Root $Msys2Root -Flavor $flavor -Packages @("base-devel", "mingw-w64-ucrt-x86_64-toolchain")
        return $true
    } catch {
        Write-Host " MSYS2 toolchain provisioning failed: $($_.Exception.Message)" -ForegroundColor Yellow
        return $false
    }
}
function Create-StartMenuShortcut {
    param(
        [string]$ProjectRoot,
        [string]$ExePath
    )

    if (-not $ProjectRoot -or -not $ExePath) {
        return
    }
    if (-not (Test-Path $ExePath)) {
        return
    }

    $startMenuRoot = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    $vtFolder = Join-Path $startMenuRoot "VideoTools"
    if (-not (Test-Path $vtFolder)) {
        New-Item -ItemType Directory -Path $vtFolder -Force | Out-Null
    }

    try {
        $shell = New-Object -ComObject WScript.Shell
    } catch {
        Write-Host "[WARN]  Unable to create Start Menu shortcut." -ForegroundColor Yellow
        return
    }

    $exeShortcut = Join-Path $vtFolder "VideoTools.lnk"
    $shortcut = $shell.CreateShortcut($exeShortcut)
    $shortcut.TargetPath = $ExePath
    $shortcut.WorkingDirectory = $ProjectRoot
    $shortcut.Save()
    Write-Host "[OK]  Start Menu shortcut created: VideoTools" -ForegroundColor Green
}
function Use-Toolchain {
    $returnedPath = $null
    $msys2Root = Resolve-Msys2Root
    if ($msys2Root) {
        $flavor = Resolve-Msys2Flavor
        $path = Join-Path $msys2Root "$flavor\\bin"
        if (Test-Path $path) {
            if ($env:Path -notmatch [Regex]::Escape($path)) {
                $env:Path = "$path;$env:Path"
            }
            $gccPath = Join-Path $path "gcc.exe"
            $gxxPath = Join-Path $path "g++.exe"
            if (Test-Path $gccPath) {
                $env:CC = $gccPath
            }
            if (Test-Path $gxxPath) {
                $env:CXX = $gxxPath
            }

            $tempDir = Join-Path $env:TEMP "vt-gcc-test"
            try {
                New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
                $cfile = Join-Path $tempDir "test.c"
                $ofile = Join-Path $tempDir "test.o"
                Set-Content -Path $cfile -Value "int main(){return 0;}" -Encoding ASCII
                & gcc -c $cfile -o $ofile 2>$null | Out-Null
                $ok = Test-Path $ofile
                if (Test-Path $cfile) { Remove-Item $cfile -Force }
                if (Test-Path $ofile) { Remove-Item $ofile -Force }
                if (Test-Path $tempDir) { Remove-Item $tempDir -Recurse -Force }

                if ($ok) {
                    $returnedPath = $path
                }
            } catch {
                if (Test-Path $tempDir) { Remove-Item $tempDir -Recurse -Force }
            }
        }
    }

    if ($returnedPath) {
        return $returnedPath
    }

    return $null
}

function Test-Gcc {
    $tempDir = Join-Path $env:TEMP "vt-gcc-test"
    New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
    $cfile = Join-Path $tempDir "test.c"
    $ofile = Join-Path $tempDir "test.o"
    Set-Content -Path $cfile -Value "int main(){return 0;}" -Encoding ASCII
    & gcc -c $cfile -o $ofile 2>$null | Out-Null
    $ok = Test-Path $ofile
    if (Test-Path $cfile) { Remove-Item $cfile -Force }
    if (Test-Path $ofile) { Remove-Item $ofile -Force }
    return $ok
}

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[INFO]  Elevation required for Windows build tools." -ForegroundColor Yellow
    Write-Host "        Approve the UAC prompt to continue." -ForegroundColor Yellow
    try {
        $argsList = @(
            "-NoProfile",
            "-NoExit",
            "-ExecutionPolicy", "Bypass",
            "-File", $PSCommandPath
        )
        foreach ($key in $PSBoundParameters.Keys) {
            if ($PSBoundParameters[$key] -is [switch] -or $PSBoundParameters[$key] -eq $true) {
                $argsList += "-$key"
            } else {
                $argsList += "-$key"
                $argsList += "$($PSBoundParameters[$key])"
            }
        }
        if ($args.Count -gt 0) {
            $argsList += $args
        }
        Start-Process -FilePath "powershell.exe" -Verb RunAs -ArgumentList $argsList -WorkingDirectory $PSScriptRoot | Out-Null
    } catch {
        Write-Host "[ERROR]  Failed to prompt for elevation." -ForegroundColor Red
        Write-Host "        Run this script from an Administrator PowerShell." -ForegroundColor Yellow
        Exit-WithPause 1
    }
    exit 0
}

Write-Header "VideoTools Windows Build"

# Get project root (parent of scripts directory)
$PROJECT_ROOT = Split-Path -Parent $PSScriptRoot
$BUILD_OUTPUT = Join-Path $PROJECT_ROOT "VideoTools.exe"
$appVersionLine = (Get-Content (Join-Path $PROJECT_ROOT "main.go") | Select-String -Pattern 'appVersion' | Select-Object -First 1).ToString()
$appVersion = ""
if ($appVersionLine) {
    $parts = $appVersionLine -split ([char]34)
    if ($parts.Length -ge 2) {
        $appVersion = $parts[1]
    }
}
if ([string]::IsNullOrWhiteSpace($appVersion)) {
    $appVersion = "(version unknown)"
}
$gitCommit = ""
if (Get-Command git -ErrorAction SilentlyContinue) {
    $gitCommit = (git -C $PROJECT_ROOT rev-parse --short HEAD 2>$null).Trim()
}
if ([string]::IsNullOrWhiteSpace($gitCommit)) {
    $gitCommit = "nogit"
}
$channel = $env:VT_BUILD_CHANNEL
if ([string]::IsNullOrWhiteSpace($channel)) {
    $channel = "dev"
}
switch ($channel.ToLower()) {
    "stable" { $channel = "stable" }
    "public" { $channel = "stable" }
    "release" { $channel = "stable" }
    default { $channel = "dev" }
}
$version = $appVersion
if ($channel -eq "stable") {
    $version = $version -replace "-dev\\d+$", ""
}
$osTag = "win"
$distDir = Join-Path $PROJECT_ROOT "dist\\windows\\$channel"
$artifactName = "$version-$gitCommit`_$osTag.zip"

Write-Host (" Build: {0} ({1})" -f $version, $channel) -ForegroundColor Cyan
Write-Host (" Commit: {0}" -f $gitCommit) -ForegroundColor Cyan
Write-Host (" Output: {0}" -f $BUILD_OUTPUT) -ForegroundColor Cyan
Write-Host ""

Refresh-Path

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host " ERROR: Go is not installed. Please run scripts\\windows\\support\\install-deps-windows.ps1 first." -ForegroundColor Red
    Exit-WithPause 1
}

Write-Host " Go version:" -ForegroundColor Green
go version
Write-Host ""

# Ensure toolchain PATH and compiler env vars are set
$toolchainPath = Use-Toolchain
if ($toolchainPath) {
    Write-Host " Toolchain: $toolchainPath" -ForegroundColor Green
} else {
    Write-Host " WARNING: GCC toolchain not found in PATH." -ForegroundColor Yellow
}

if (-not (Test-Command gcc)) {
    $msys2Root = Resolve-Msys2Root
    if ($msys2Root -and (Ensure-Msys2Toolchain -Msys2Root $msys2Root)) {
        Refresh-Path
        $toolchainPath = Use-Toolchain
    }
}

if (-not (Test-Command gcc)) {
    Write-Host " ERROR: GCC is required for CGO builds on Windows." -ForegroundColor Red
    Write-Host " Run scripts\\install.ps1 and enable MSYS2 build tools." -ForegroundColor Yellow
    Exit-WithPause 1
}

$gccCmd = Get-Command gcc -ErrorAction SilentlyContinue
$msys2Root = Resolve-Msys2Root
if ($gccCmd -and $msys2Root) {
    $expectedRoot = Join-Path $msys2Root "$(Resolve-Msys2Flavor)\\bin"
    if ($gccCmd.Path -notmatch [Regex]::Escape($expectedRoot)) {
        Write-Host " ERROR: GCC found, but not from MSYS2: $($gccCmd.Path)" -ForegroundColor Red
        Write-Host " Install the MSYS2 toolchain and re-run scripts\\install.ps1." -ForegroundColor Yellow
        Exit-WithPause 1
    }
} elseif ($gccCmd -and -not $msys2Root) {
    Write-Host " ERROR: GCC found, but MSYS2 is missing: $($gccCmd.Path)" -ForegroundColor Red
    Write-Host " Install the MSYS2 toolchain and re-run scripts\\install.ps1." -ForegroundColor Yellow
    Exit-WithPause 1
}

if (-not (Test-Gcc)) {
    if ($msys2Root -and (Ensure-Msys2Toolchain -Msys2Root $msys2Root)) {
        Refresh-Path
        if (-not (Test-Gcc)) {
            Write-Host " ERROR: GCC failed a test compile. The toolchain appears incomplete." -ForegroundColor Red
            Write-Host " Recommended fix: reinstall the MSYS2 toolchain (pacman -S --needed mingw-w64-ucrt-x86_64-toolchain)." -ForegroundColor Yellow
            Write-Host " If MSYS2 is missing, install it and re-run scripts\\install.ps1." -ForegroundColor Yellow
            Exit-WithPause 1
        }
    } else {
        Write-Host " ERROR: GCC failed a test compile. The toolchain appears incomplete." -ForegroundColor Red
        Write-Host " Recommended fix: reinstall the MSYS2 toolchain (pacman -S --needed mingw-w64-ucrt-x86_64-toolchain)." -ForegroundColor Yellow
        Write-Host " If MSYS2 is missing, install it and re-run scripts\\install.ps1." -ForegroundColor Yellow
        Exit-WithPause 1
    }
}

# Change to project directory
Set-Location $PROJECT_ROOT

if ($Clean) {
    Write-Host "Cleaning previous builds and cache..." -ForegroundColor Yellow
    go clean -cache -modcache -testcache 2>$null
    if (Test-Path $BUILD_OUTPUT) {
        Remove-Item $BUILD_OUTPUT -Force
    }
    Write-Host "Cache cleaned" -ForegroundColor Green
    Write-Host ""
}

Write-Section "Dependencies"
Write-Host "Downloading and verifying dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host " Failed to download dependencies" -ForegroundColor Red
    Exit-WithPause 1
}

go mod verify
if ($LASTEXITCODE -ne 0) {
    Write-Host " Failed to verify dependencies" -ForegroundColor Red
    Exit-WithPause 1
}
Write-Host " Dependencies verified" -ForegroundColor Green
Write-Host ""

Write-Section "Build"
Write-Host "Building VideoTools..." -ForegroundColor Yellow
Write-Host ""

# Embed Windows icon if windres is available
$rcFile = Join-Path $PROJECT_ROOT "scripts\videotools.rc"
$sysoFile = Join-Path $PROJECT_ROOT "videotools_windows_amd64.syso"
if (Test-Path $rcFile) {
    $windresCandidates = @()
    $windresCmd = Get-Command windres -ErrorAction SilentlyContinue
    if ($windresCmd) {
        $windresCandidates += $windresCmd.Path
    }
    $msys2Flavor = Resolve-Msys2Flavor
    $msys2Root = Resolve-Msys2Root
    if ($msys2Root) {
        $windresCandidates += (Join-Path $msys2Root "$msys2Flavor\\bin\\windres.exe")
    }
    $windresCandidates += @(
        "C:\msys64\$msys2Flavor\bin\windres.exe",
        "C:\msys64\usr\bin\windres.exe",
        "C:\MinGW\bin\windres.exe"
    )
    $windresPath = $windresCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    if ($windresPath) {
        & $windresPath $rcFile -O coff -o $sysoFile | Out-Null
        if (-not (Test-Path $sysoFile)) {
            Write-Host "  windres did not produce $sysoFile; icon may be missing" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  windres not found; Windows icon will not be embedded in the EXE" -ForegroundColor Yellow
    }
}

# Fyne needs CGO for GLFW/OpenGL bindings
$env:CGO_ENABLED = "1"

# Detect number of CPU cores for parallel compilation
$numCores = (Get-CimInstance Win32_ComputerSystem).NumberOfLogicalProcessors
if (-not $numCores -or $numCores -lt 1) {
    $numCores = 4  # Fallback to 4 if detection fails
}
Write-Host "Using $numCores parallel build processes" -ForegroundColor Cyan

# Build the application with optimizations
# -p: Number of parallel build processes (use all cores)
# -ldflags="-s -w": Strip debug info and symbol table (faster linking, smaller binary)
# -trimpath: Remove absolute file paths from binary (faster builds, smaller binary)
go build -p $numCores -ldflags="-s -w" -trimpath -o $BUILD_OUTPUT .

if ($LASTEXITCODE -eq 0) {
    Write-Host " Build successful!" -ForegroundColor Green
    Write-Host ""
    Write-Section "Build Complete"

    # Get file size
    $fileSize = (Get-Item $BUILD_OUTPUT).Length
    $fileSizeMB = [math]::Round($fileSize / 1MB, 2)

    Write-Host "Output: $BUILD_OUTPUT" -ForegroundColor White
    Write-Host "Size: $fileSizeMB MB" -ForegroundColor White
    Write-Host ""
    Create-StartMenuShortcut -ProjectRoot $PROJECT_ROOT -ExePath $BUILD_OUTPUT

    Write-Host "To run:" -ForegroundColor Yellow
    Write-Host "  .\VideoTools.exe" -ForegroundColor White
    Write-Host ""

    Write-Section "Packaging"
    Write-Host "Packaging build artifacts..." -ForegroundColor Yellow
    if (-not (Test-Path $distDir)) {
        New-Item -ItemType Directory -Path $distDir -Force | Out-Null
    }

    $pkgDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "vt-build-$([Guid]::NewGuid())") -Force
    Copy-Item $BUILD_OUTPUT -Destination $pkgDir.FullName -Force
    if (Test-Path (Join-Path $PROJECT_ROOT "README.md")) {
        Copy-Item (Join-Path $PROJECT_ROOT "README.md") -Destination $pkgDir.FullName -Force
    }
    if (Test-Path (Join-Path $PROJECT_ROOT "LICENSE")) {
        Copy-Item (Join-Path $PROJECT_ROOT "LICENSE") -Destination $pkgDir.FullName -Force
    }
    if (Test-Path (Join-Path $PROJECT_ROOT "ffmpeg.exe")) {
        Copy-Item (Join-Path $PROJECT_ROOT "ffmpeg.exe") -Destination $pkgDir.FullName -Force
    }
    if (Test-Path (Join-Path $PROJECT_ROOT "ffprobe.exe")) {
        Copy-Item (Join-Path $PROJECT_ROOT "ffprobe.exe") -Destination $pkgDir.FullName -Force
    }

    $artifactPath = Join-Path $distDir $artifactName
    if (Test-Path $artifactPath) {
        Remove-Item $artifactPath -Force
    }
    Compress-Archive -Path (Join-Path $pkgDir.FullName "*") -DestinationPath $artifactPath

    $publishedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    $buildJson = @{
        channel = $channel
        version = $version
        git = $gitCommit
        published_at = $publishedAt
        artifact = $artifactName
    } | ConvertTo-Json -Depth 3
    Set-Content -Path (Join-Path $distDir "build.json") -Value $buildJson -Encoding UTF8

    Remove-Item $pkgDir.FullName -Recurse -Force
    Write-Host "Build package: $artifactPath" -ForegroundColor White
    Write-Host "Build metadata: $(Join-Path $distDir 'build.json')" -ForegroundColor White
    Write-Host ""

    # Check if ffmpeg is available
    if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
        Write-Host "  Warning: ffmpeg not found in PATH" -ForegroundColor Yellow
        Write-Host "   VideoTools requires ffmpeg to convert videos" -ForegroundColor Yellow
        Write-Host "   Run: .\scripts\\windows\\support\\install-deps-windows.ps1" -ForegroundColor Yellow
        Write-Host ""
    }

    # Check for NVIDIA GPU
    try {
        $nvidiaGpu = Get-WmiObject Win32_VideoController | Where-Object { $_.Name -like "*NVIDIA*" }
        if ($nvidiaGpu) {
            Write-Host " NVIDIA GPU detected: $($nvidiaGpu.Name)" -ForegroundColor Green
            Write-Host "   Hardware encoding (NVENC) will be available" -ForegroundColor Green
            Write-Host ""
        }
    } catch {
        # GPU detection failed, not critical
    }
} else {
    Write-Host "Build failed!" -ForegroundColor Red
    Exit-WithPause 1
}

Exit-WithPause 0
