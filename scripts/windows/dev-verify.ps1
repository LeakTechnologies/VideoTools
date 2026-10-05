# VideoTools Local Dev Verify Script for Windows
# Runs `go build -tags=native_media ./...` + `go vet` against the local FFmpeg
# toolchain. Use this instead of bare `go build` — the native_media engine
# needs CGO + the -Wl,--stack linker flag, which bare `go build` rejects
# without CGO_LDFLAGS_ALLOW (see AGENTS.md: Windows CGo build gate).

# Set console encoding to UTF-8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = 'Continue'

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $projectRoot

# --- Toolchain ---
$env:CGO_ENABLED = "1"
# Allow -Wl,--stack,N in #cgo LDFLAGS (sets PE default thread stack to 4 MB)
$env:CGO_LDFLAGS_ALLOW = "-Wl,.*"

# Set GCC explicitly if discoverable (WinLibs / MSYS2 / system).
$gccPath = (Get-Command gcc -ErrorAction SilentlyContinue).Source
if ($gccPath) {
    $env:CC = "`"$gccPath`""
    $env:CXX = "`"$($gccPath -replace 'gcc\.exe$', 'g++.exe')`""
}

Write-Host "Toolchain:" -ForegroundColor Cyan
Write-Host "  CC = $env:CC"
Write-Host "  CGO_LDFLAGS_ALLOW = $env:CGO_LDFLAGS_ALLOW"
Write-Host ""

# --- FFmpeg runtime DLLs on PATH ---
# `go build` only links; nothing loads the DLLs, so a missing PATH entry is
# invisible at build time. `go test` produces a real executable that loads
# avcodec/avformat/... at startup, so a missing PATH entry makes every test
# package die with exit 0xc0000135 (STATUS_DLL_NOT_FOUND) BEFORE any test
# runs — which reads exactly like a regression but is purely environmental.
# The local-dev CGo LDFLAGS point at C:/ffmpeg/lib, so C:/ffmpeg/bin holds the
# matching runtime DLLs.
$ffmpegBin = "C:\ffmpeg\bin"
if (Test-Path $ffmpegBin) {
    if ($env:PATH -notlike "*$ffmpegBin*") {
        $env:PATH = "$ffmpegBin;$env:PATH"
    }
    Write-Host "  FFmpeg bin = $ffmpegBin (added to PATH)" -ForegroundColor Cyan
} else {
    Write-Host "  WARNING: $ffmpegBin not found. 'go build' will still work, but 'go test'" -ForegroundColor Yellow
    Write-Host "           will fail with 0xc0000135 (DLL not found) for any package that" -ForegroundColor Yellow
    Write-Host "           links FFmpeg." -ForegroundColor Yellow
}
Write-Host ""

# --- Build (native media engine) ---
Write-Host "Building (tags=native_media):" -ForegroundColor Cyan
go build -tags native_media ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "BUILD FAILED (exit $LASTEXITCODE)" -ForegroundColor Red
    exit $LASTEXITCODE
}
Write-Host "  build OK" -ForegroundColor Green
Write-Host ""

# --- Vet key packages ---
Write-Host "Vetting:" -ForegroundColor Cyan
go vet -tags native_media ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "VET FAILED (exit $LASTEXITCODE)" -ForegroundColor Red
    exit $LASTEXITCODE
}
Write-Host "  vet OK" -ForegroundColor Green
Write-Host ""

# --- Tests ---
# Run here (rather than left to the developer) so the DLL PATH fix above is
# always in effect. A package that dies with 0xc0000135 is environmental, not
# a code regression: check for the missing C:\ffmpeg\bin before investigating.
Write-Host "Testing (tags=native_media):" -ForegroundColor Cyan
$testOutput = go test -tags native_media ./... 2>&1
$testExit = $LASTEXITCODE
$testOutput | ForEach-Object { Write-Host "  $_" }
if ($testExit -ne 0) {
    Write-Host "TESTS FAILED (exit $testExit)" -ForegroundColor Red
    if ($testOutput -match '0xc0000135') {
        Write-Host "  0xc0000135 = STATUS_DLL_NOT_FOUND: FFmpeg runtime DLLs are not on PATH." -ForegroundColor Red
        Write-Host "  Confirm C:\ffmpeg\bin exists and holds avcodec/avformat DLLs." -ForegroundColor Red
    }
    exit $testExit
}
Write-Host "  tests OK" -ForegroundColor Green
Write-Host ""

Write-Host "Verify complete: native_media build + vet + tests green." -ForegroundColor Green