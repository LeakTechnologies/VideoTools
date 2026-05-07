# Build FFmpeg shared DLLs for VideoTools runtime
# Output: ffmpeg-shared\ (dlls + executables)
# These DLLs are bundled in the release ZIP so users never need to download them.

$ErrorActionPreference = "Stop"

$ffmpegVersion = "8.1"
$workDir = "C:\ffmpeg-shared-build"
$outputDir = "C:\ffmpeg-shared"

Write-Host "[INFO] Building FFmpeg ${ffmpegVersion} shared DLLs..."

# Clean previous builds
Remove-Item -Recurse -Force $workDir -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $outputDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $workDir | Out-Null
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

# Use MSYS2 bash for the build
$bashPath = "C:\msys64\usr\bin\bash.exe"
if (-not (Test-Path $bashPath)) {
    Write-Error "MSYS2 not found at C:\msys64"
    exit 1
}

function Invoke-Bash([string]$cmd) {
    & $bashPath -lc "( $cmd ) 2>&1" | ForEach-Object { Write-Host "$_" }
    if ($LASTEXITCODE -ne 0) {
        Write-Error "bash command failed (exit $LASTEXITCODE): $cmd"
        exit 1
    }
}

# Build x264 (static, for linking into FFmpeg shared)
Invoke-Bash @"
cd /c/ffmpeg-shared-build
rm -rf x264-src
git clone --depth=1 https://code.videolan.org/videolan/x264.git x264-src
cd x264-src
./configure --prefix=/c/ffmpeg-shared --enable-static --disable-shared --disable-cli --disable-interlaced --disable-asm
make -j`nproc` && make install
"@

# Build x265 (static, for linking into FFmpeg shared)
Invoke-Bash @"
cd /c/ffmpeg-shared-build
rm -rf x265-src x265-build
git clone --depth=1 https://bitbucket.org/multicoreware/x265_git.git x265-src
cd x265-src && git tag 4.1.0
mkdir x265-build && cd x265-build
cmake -G 'MSYS Makefiles' \
    -DCMAKE_INSTALL_PREFIX=/c/ffmpeg-shared \
    -DENABLE_SHARED=OFF -DENABLE_CLI=OFF \
    -DCMAKE_BUILD_TYPE=Release \
    ../x265-src/source
make -j`nproc` && make install
"@

# Build FFmpeg shared (DLLs)
Invoke-Bash @"
cd /c/ffmpeg-shared-build
rm -rf ffmpeg-src ffmpeg.tar.bz2
wget -q -O ffmpeg.tar.bz2 'https://ffmpeg.org/releases/ffmpeg-${ffmpegVersion}.tar.bz2'
tar xf ffmpeg.tar.bz2
mv ffmpeg-${ffmpegVersion} ffmpeg-src
cd ffmpeg-src
./configure \
    --prefix=/c/ffmpeg-shared \
    --enable-shared --disable-static \
    --disable-doc --disable-programs \
    --enable-gpl --enable-version3 \
    --enable-libx264 --enable-libx265 \
    --disable-vaapi --disable-iconv \
    --extra-cflags='-I/c/ffmpeg-shared/include' \
    --extra-ldflags='-L/c/ffmpeg-shared/lib'
make -j`nproc` && make install
"@

# Copy only the DLLs we need (no executables, no development files)
$dllSource = Join-Path $outputDir "bin"
$dllDest = Join-Path $outputDir "dll"
New-Item -ItemType Directory -Force -Path $dllDest | Out-Null

Get-ChildItem -Path $dllSource -Filter "*.dll" | ForEach-Object {
    Copy-Item $_.FullName -Destination $dllDest -Force
    Write-Host "[INFO] Bundled: $($_.Name)"
}

# Clean up build artifacts, keep only dlls
Remove-Item (Join-Path $outputDir "bin") -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $outputDir "lib") -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $outputDir "include") -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $outputDir "share") -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "[INFO] FFmpeg shared DLLs built and bundled in: $outputDir\dll"
Write-Host "[INFO] DLLs:"
Get-ChildItem -Path $dllDest | ForEach-Object { Write-Host "  $($_.Name)" }
