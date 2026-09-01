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

Write-Host "Verify complete: native_media build + vet green." -ForegroundColor Green