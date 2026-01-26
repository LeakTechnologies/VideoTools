# VideoTools Build Script for Windows
# Builds the VideoTools application with proper error handling

param(
    [switch]$Clean = $false,
    [switch]$SkipTests = $false
)

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
        exit 1
    }
    exit 0
}

Write-Host "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•" -ForegroundColor Cyan
Write-Host "  VideoTools Build Script (Windows)" -ForegroundColor Cyan
Write-Host "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•" -ForegroundColor Cyan
Write-Host ""

# Get project root (parent of scripts directory)
$PROJECT_ROOT = Split-Path -Parent $PSScriptRoot
$BUILD_OUTPUT = Join-Path $PROJECT_ROOT "VideoTools.exe"
$appVersion = (Get-Content (Join-Path $PROJECT_ROOT "main.go") | Select-String -Pattern 'appVersion' | Select-Object -First 1).ToString()
if ($appVersion -match '"([^"]+)"') {
    $appVersion = $matches[1]
} else {
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

# Check if Go is installed
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "âŒ ERROR: Go is not installed. Please run scripts\_internal\install-deps-windows.ps1 first." -ForegroundColor Red
    exit 1
}

Write-Host "ðŸ“¦ Go version:" -ForegroundColor Green
go version
Write-Host ""

# Change to project directory
Set-Location $PROJECT_ROOT

if ($Clean) {
    Write-Host "ðŸ§¹ Cleaning previous builds and cache..." -ForegroundColor Yellow
    go clean -cache -modcache -testcache 2>$null
    if (Test-Path $BUILD_OUTPUT) {
        Remove-Item $BUILD_OUTPUT -Force
    }
    Write-Host "âœ“ Cache cleaned" -ForegroundColor Green
    Write-Host ""
}

Write-Host "â¬‡ï¸  Downloading and verifying dependencies..." -ForegroundColor Yellow
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host "âŒ Failed to download dependencies" -ForegroundColor Red
    exit 1
}

go mod verify
if ($LASTEXITCODE -ne 0) {
    Write-Host "âŒ Failed to verify dependencies" -ForegroundColor Red
    exit 1
}
Write-Host "âœ“ Dependencies verified" -ForegroundColor Green
Write-Host ""

Write-Host "ðŸ”¨ Building VideoTools..." -ForegroundColor Yellow
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
    $windresCandidates += @(
        "C:\msys64\mingw64\bin\windres.exe",
        "C:\msys64\usr\bin\windres.exe",
        "C:\MinGW\bin\windres.exe"
    )
    $windresPath = $windresCandidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    if ($windresPath) {
        & $windresPath $rcFile -O coff -o $sysoFile | Out-Null
        if (-not (Test-Path $sysoFile)) {
            Write-Host "âš ï¸  windres did not produce $sysoFile; icon may be missing" -ForegroundColor Yellow
        }
    } else {
        Write-Host "âš ï¸  windres not found; Windows icon will not be embedded in the EXE" -ForegroundColor Yellow
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
    Write-Host "âœ“ Build successful!" -ForegroundColor Green
    Write-Host ""
    Write-Host "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•" -ForegroundColor Cyan
    Write-Host "âœ… BUILD COMPLETE" -ForegroundColor Green
    Write-Host "â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•â•" -ForegroundColor Cyan
    Write-Host ""

    # Get file size
    $fileSize = (Get-Item $BUILD_OUTPUT).Length
    $fileSizeMB = [math]::Round($fileSize / 1MB, 2)

    Write-Host "Output: $BUILD_OUTPUT" -ForegroundColor White
    Write-Host "Size: $fileSizeMB MB" -ForegroundColor White
    Write-Host ""
    Write-Host "To run:" -ForegroundColor Yellow
    Write-Host "  .\VideoTools.exe" -ForegroundColor White
    Write-Host ""

    Write-Host "ðŸ“¦ Packaging build artifacts..." -ForegroundColor Yellow
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
    Write-Host "Build metadata: $(Join-Path $distDir "build.json")" -ForegroundColor White
    Write-Host ""

    # Check if ffmpeg is available
    if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
        Write-Host "âš ï¸  Warning: ffmpeg not found in PATH" -ForegroundColor Yellow
        Write-Host "   VideoTools requires ffmpeg to convert videos" -ForegroundColor Yellow
        Write-Host "   Run: .\scripts\_internal\install-deps-windows.ps1" -ForegroundColor Yellow
        Write-Host ""
    }

    # Check for NVIDIA GPU
    try {
        $nvidiaGpu = Get-WmiObject Win32_VideoController | Where-Object { $_.Name -like "*NVIDIA*" }
        if ($nvidiaGpu) {
            Write-Host "ðŸŽ® NVIDIA GPU detected: $($nvidiaGpu.Name)" -ForegroundColor Green
            Write-Host "   Hardware encoding (NVENC) will be available" -ForegroundColor Green
            Write-Host ""
        }
    } catch {
        # GPU detection failed, not critical
    }

} else {
    Write-Host "âŒ Build failed!" -ForegroundColor Red
    exit 1
}
