# VideoTools CI Build Script for Windows
# Called by .forgejo/workflows/dev-packages.yml
# Reads signing secrets from environment; produces dist\windows\<version>_windows.zip

$ErrorActionPreference = 'Continue'
$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $projectRoot

# --- Version ---
$appVersion = ""
if (Test-Path "VERSION") { $appVersion = (Get-Content "VERSION" -Raw).Trim() }
if ([string]::IsNullOrWhiteSpace($appVersion)) {
    $line = (Get-Content "main.go" | Select-String 'appVersion' | Select-Object -First 1).ToString()
    $parts = $line -split [char]34
    if ($parts.Length -ge 2) { $appVersion = $parts[1] }
}
if ([string]::IsNullOrWhiteSpace($appVersion)) { $appVersion = "v0.1.1-dev" }
$gitCommit = ""
try { $gitCommit = (git rev-parse --short HEAD 2>$null).Trim() } catch {}
if (-not $gitCommit) { $gitCommit = "nogit" }

# --- Toolchain ---
$msys2Bin = "C:\msys64\ucrt64\bin"
$pkgConfigExe = Join-Path $msys2Bin "pkg-config.exe"
$env:PATH = "$msys2Bin;C:\msys64\usr\bin;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
$env:CGO_CFLAGS = "-IC:\ffmpeg-static\include -IC:\msys64\ucrt64\include -g0"
$env:PKG_CONFIG_PATH = "C:\ffmpeg-static\lib\pkgconfig;C:\msys64\ucrt64\lib\pkgconfig"

# Promote static archives from MSYS2 into the ffmpeg prefix (first -L dir)
# so ld picks lib<name>.a over lib<name>.dll.a — keeps the exe free of
# MinGW runtime DLL dependencies (libbz2-1.dll, zlib1.dll, libstdc++-6.dll).
foreach ($lib in @("bz2", "z", "lzma", "iconv", "stdc++")) {
    $src = "C:\msys64\ucrt64\lib\lib${lib}.a"
    $dst = "C:\ffmpeg-static\lib\lib${lib}.a"
    if ((Test-Path $src) -and -not (Test-Path $dst)) {
        Copy-Item $src $dst -Force
        Write-Host "[INFO] Promoted static archive: lib${lib}.a"
    }
}

$ffmpegPkgs = @("libavcodec","libavformat","libswscale","libavutil","libswresample","libavfilter")
$staticLibs = (& $pkgConfigExe --libs --static $ffmpegPkgs 2>$null) -join " "
if (-not $staticLibs) {
    Write-Error "pkg-config returned no flags for FFmpeg - check C:\ffmpeg-static"
    exit 1
}
$staticLibs = (($staticLibs -split '\s+') | Where-Object { $_ -ne '-lsupc++' }) -join ' '
$env:CGO_LDFLAGS = "$staticLibs -LC:\msys64\ucrt64\lib -loleaut32 -lgdi32 -lpsapi -lavrt -lmfplat -static-libgcc -static-libstdc++ -Wl,-Bstatic,-lpthread -Wl,-Bdynamic"
$env:CGO_LDFLAGS_ALLOW = "-Wl,.*"
Write-Host "[INFO] CGO_LDFLAGS: $env:CGO_LDFLAGS"

# --- Windows resource (icon) ---
$buildOutput = Join-Path $projectRoot "VideoTools.exe"
$rcFile     = Join-Path $projectRoot "scripts\videotools.rc"
$sysoFile   = Join-Path $projectRoot "videotools_windows_amd64.syso"
if (Test-Path $rcFile) {
    $windresCmd = Get-Command windres -ErrorAction SilentlyContinue
    if ($windresCmd) {
        & $windresCmd.Path $rcFile -O coff -o $sysoFile | Out-Null
    } else {
        Write-Host "[WARN] windres not found; icon will not be embedded."
    }
}

# --- Build ---
go mod download
go mod verify

$ldflags = "-s -w -H windowsgui -X main.buildCommit=$gitCommit"
go build -p 4 -tags native_media -ldflags="$ldflags" -trimpath -o $buildOutput .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# --- DLL dependency gate (fatal) ---
# The Windows product ships as fully static binaries; any MinGW runtime DLL
# reference means the static promotion failed and the exe will not start on
# user machines.
$objdumpExe = Join-Path $msys2Bin "objdump.exe"
if (Test-Path $objdumpExe) {
    Write-Host "[INFO] DLL imports in ${buildOutput}:"
    $imports = & $objdumpExe -p $buildOutput 2>$null | Select-String "DLL Name"
    $imports | ForEach-Object { Write-Host "  $_" }
    $bad = $imports | Where-Object { $_ -match 'lib(bz2|lzma|iconv|stdc\+\+|winpthread|gcc)|zlib1' }
    if ($bad) {
        Write-Error "[ERROR] ${buildOutput} depends on non-system DLLs:`n$($bad -join "`n")"
        exit 1
    }
} else {
    Write-Host "[WARN] objdump not found; skipping DLL import gate"
}

# --- Sign (optional, non-fatal) ---
$hasPfx      = $env:VT_SIGN_PFX_B64 -and $env:VT_SIGN_PASSWORD
$hasSignPath = $env:SIGNPATH_API_TOKEN -and $env:SIGNPATH_ORGANIZATION_ID
$signScript  = Join-Path $projectRoot "scripts\windows\support\sign-exe.ps1"
if (($hasPfx -or $hasSignPath) -and (Test-Path $signScript)) {
    try {
        if ($hasPfx -and -not $hasSignPath) {
            $pfxPath = Join-Path $env:TEMP "vt-sign.pfx"
            [IO.File]::WriteAllBytes($pfxPath, [Convert]::FromBase64String($env:VT_SIGN_PFX_B64))
            $tsUrl = if ($env:VT_SIGN_TIMESTAMP) { $env:VT_SIGN_TIMESTAMP } else { "http://timestamp.digicert.com" }
            & $signScript -ExePath $buildOutput -CertPath $pfxPath -CertPassword $env:VT_SIGN_PASSWORD -TimestampUrl $tsUrl
        } else {
            & $signScript -ExePath $buildOutput
        }
        if ($LASTEXITCODE -ne 0) { throw "sign-exe.ps1 exited $LASTEXITCODE" }
    } catch {
        Write-Host "[sign] WARNING: Signing failed: $_"
        Write-Host "[sign] Continuing with unsigned binary."
    }
} else {
    Write-Host "[sign] Skipping: no signing credentials configured."
}

# --- Package ---
$distDir = Join-Path $projectRoot "dist\windows"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
$pkgDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP "vt-build-$([Guid]::NewGuid())") -Force

# Main executable
Copy-Item $buildOutput -Destination $pkgDir.FullName -Force

# Bundle static ffmpeg.exe/ffprobe.exe in package root — the whole product is
# three self-contained binaries; there is no DLL/ folder (settled 2026-07-04).
$ffmpegBinSource = "C:\ffmpeg-static\bin"
foreach ($tool in @("ffmpeg.exe", "ffprobe.exe")) {
    $src = Join-Path $ffmpegBinSource $tool
    if (-not (Test-Path $src)) {
        Write-Error "[ERROR] $src not found. The static FFmpeg build must run with programs enabled (no --disable-programs) and --extra-ldflags=-static."
        exit 1
    }
    Copy-Item $src -Destination $pkgDir.FullName -Force
    # Gate the sidecars too — a static-link regression here would ship a
    # binary that needs MinGW DLLs the user does not have.
    if (Test-Path $objdumpExe) {
        $bad = & $objdumpExe -p $src 2>$null | Select-String "DLL Name" |
            Where-Object { $_ -match 'lib(bz2|lzma|iconv|stdc\+\+|winpthread|gcc)|zlib1' }
        if ($bad) {
            Write-Error "[ERROR] $tool depends on non-system DLLs:`n$($bad -join "`n")"
            exit 1
        }
    }
}
Write-Host "[INFO] Bundled static ffmpeg.exe and ffprobe.exe in package root"

# README
if (Test-Path (Join-Path $projectRoot "README.md")) {
    Copy-Item (Join-Path $projectRoot "README.md") -Destination $pkgDir.FullName -Force
}

$artifactPath = Join-Path $distDir "${appVersion}_windows.zip"
if (Test-Path $artifactPath) { Remove-Item $artifactPath -Force }
Compress-Archive -Path (Join-Path $pkgDir.FullName "*") -DestinationPath $artifactPath
Remove-Item $pkgDir.FullName -Recurse -Force
Write-Host "[INFO] Package: $artifactPath"
