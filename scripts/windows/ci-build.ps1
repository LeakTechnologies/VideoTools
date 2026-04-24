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
$msys2Bin = "E:\dependencies\msys64\ucrt64\bin"
$pkgConfigExe = Join-Path $msys2Bin "pkg-config.exe"
$env:PATH = "$msys2Bin;E:\dependencies\msys64\usr\bin;$env:PATH"
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
# -g0: disable debug info in CGO intermediate files; FFmpeg headers produce
# enormous .s temp files in C:\Windows\Temp that exhaust disk space otherwise.
$env:CGO_CFLAGS = "-IE:\dependencies\ffmpeg-static\include -IE:\dependencies\msys64\ucrt64\include -g0"
$env:PKG_CONFIG_PATH = "E:\dependencies\ffmpeg-static\lib\pkgconfig;E:\dependencies\msys64\ucrt64\lib\pkgconfig"

# Promote bz2 and zlib static archives from MSYS2 into the ffmpeg prefix
# so the linker finds them first via -LE:/dependencies/ffmpeg-static/lib.
# x264 and x265 are built from source directly into /e/dependencies/ffmpeg-static and
# must NOT be replaced here -- their static archives have no __declspec(dllimport).
foreach ($lib in @("bz2", "z")) {
    $src = "E:\dependencies\msys64\ucrt64\lib\lib${lib}.a"
    $dst = "E:\dependencies\ffmpeg-static\lib\lib${lib}.a"
    if ((Test-Path $src) -and -not (Test-Path $dst)) {
        Copy-Item $src $dst -Force
        Write-Host "[INFO] Promoted static archive: lib${lib}.a"
    }
}

$ffmpegPkgs = @("libavcodec","libavformat","libswscale","libavutil","libswresample","libavfilter")
$staticLibs = (& $pkgConfigExe --libs --static $ffmpegPkgs 2>$null) -join " "
if (-not $staticLibs) {
    Write-Error "pkg-config returned no flags for FFmpeg - check E:\dependencies\ffmpeg-static"
    exit 1
}
# -lsupc++ appears transitively from x265.pc (required for FFmpeg's configure link
# test to resolve vtable symbols against the GCC-private static archive). It must
# NOT appear in the final binary link alongside -lstdc++ (the DLL import stub):
# both define std::type_info::operator== and the linker rejects multiple definitions.
# The import-stub thunks from -lstdc++ are sufficient for the final binary.
$staticLibs = (($staticLibs -split '\s+') | Where-Object { $_ -ne '-lsupc++' }) -join ' '
# Only add libs that pkg-config consistently omits from FFmpeg's .pc files on Windows.
# -static-libstdc++: prevents libstdc++-6.dll runtime dependency (stdc++ appears in FFmpeg's pkg-config output)
$env:CGO_LDFLAGS = "$staticLibs -LE:\dependencies\msys64\ucrt64\lib -loleaut32 -lgdi32 -lpsapi -lavrt -lmfplat -static-libgcc -static-libstdc++ -Wl,-Bstatic,-lpthread -Wl,-Bdynamic"
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

# --- DLL dependency report (non-fatal) ---
$objdumpExe = Join-Path $msys2Bin "objdump.exe"
if (Test-Path $objdumpExe) {
    Write-Host "[INFO] DLL imports in ${buildOutput}:"
    & $objdumpExe -p $buildOutput 2>$null | Select-String "DLL Name" | ForEach-Object { Write-Host "  $_" }
} else {
    Write-Host "[WARN] objdump not found; skipping DLL import report"
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
Copy-Item $buildOutput -Destination $pkgDir.FullName -Force
if (Test-Path (Join-Path $projectRoot "README.md")) {
    Copy-Item (Join-Path $projectRoot "README.md") -Destination $pkgDir.FullName -Force
}
$artifactPath = Join-Path $distDir "${appVersion}_windows.zip"
if (Test-Path $artifactPath) { Remove-Item $artifactPath -Force }
Compress-Archive -Path (Join-Path $pkgDir.FullName "*") -DestinationPath $artifactPath
Remove-Item $pkgDir.FullName -Recurse -Force
Write-Host "[INFO] Package: $artifactPath"
