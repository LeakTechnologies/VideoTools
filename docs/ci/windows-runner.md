# Windows Runner Setup for Forgejo (VideoTools CI)

This guide covers setting up a Windows runner for Forgejo to support VideoTools CI builds.

## Overview

VideoTools requires a Windows environment with:
- Go toolchain (1.25.1+)
- MSYS2 UCRT64 toolchain for CGO compilation
- PowerShell execution for build scripts
- Git for version control

## Step 1 - Forgejo Runner Prerequisites

### Forgejo Actions Enablement
1. In Forgejo admin UI, navigate to **Settings > Actions**
2. Ensure Actions is enabled (default in Forgejo v1.21+)
3. Note your Forgejo instance URL: `https://git.leaktechnologies.dev`

### Runner Registration Token
1. Go to **Settings > Actions > Runners**
2. Click **Create new runner**
3. Copy the registration token (single-use, expires after first registration)
4. Choose runner scope: global, organization, or repository

### Runner Binary
Forgejo uses the official `forgejo-runner` binary:
- Download from `https://code.forgejo.org/forgejo/runner/releases`
- Windows binaries are `.exe` files
- Current stable version: 3.5.1 (check releases for latest)

## Step 2 - Create Windows Service Account

```powershell
# Create dedicated user for CI runner
New-LocalUser -Name "ci-runner" -PasswordNeverExpires -UserMayNotChangePassword

# Add to necessary groups (minimal permissions)
Add-LocalGroupMember -Group "Users" -Member "ci-runner"

# Create workspace directories
New-Item -ItemType Directory -Force -Path "C:\ForgejoRunner\bin"
New-Item -ItemType Directory -Force -Path "C:\ForgejoRunner\work"
New-Item -ItemType Directory -Force -Path "C:\ForgejoRunner\config"
New-Item -ItemType Directory -Force -Path "C:\ForgejoRunner\logs"

# Set permissions
icacls "C:\ForgejoRunner" /grant "ci-runner:(OI)(CI)F" /T
```

## Step 3 - Install and Register Runner

### Download Runner Binary
```powershell
cd C:\ForgejoRunner\bin
# Replace with latest version URL
Invoke-WebRequest -Uri "https://code.forgejo.org/forgejo/runner/releases/download/v3.5.1/forgejo-runner-3.5.1-windows-amd64.exe" -OutFile "forgejo-runner.exe"
```

### Register Runner
```powershell
cd C:\ForgejoRunner
.\bin\forgejo-runner.exe register --config config\runner.yaml
```

Interactive prompts:
- **Forgejo instance URL**: `https://git.leaktechnologies.dev`
- **Registration token**: [paste from step 1]
- **Runner name**: `win-runner-01`
- **Runner group**: [default]
- **Labels**: `windows,x64,videotools,ucrt64`

### Generate Configuration
```powershell
.\bin\forgejo-runner.exe generate-config > config\runner.yaml
```

Edit `config\runner.yaml` for VideoTools needs:
```yaml
runner:
  capacity: 1  # Number of concurrent jobs
  timeout: 3h  # Maximum job duration
  insecure: false
  fetch_timeout: 5s
  fetch_interval: 2s
  labels:
    - "windows:host"
    - "x64:host"
    - "videotools:host"
    - "ucrt64:host"

cache:
  enabled: true
  dir: "C:\ForgejoRunner\cache"
  host: ""
  port: 0
  external_server: ""
  threshold: 3

network:
  host: ""
  container_network: ""
  docker_host: ""
  privileged: false
  default_caps: []
  default_volumes: []
  valid_volumes: []
  workdir_parent: "C:\ForgejoRunner\work"

container:
  network: ""
  privileged: false
  options: []
  valid_volumes: []
  docker_host: ""
  force_pull: false
  force_rebuild: false
```

## Step 4 - Install as Windows Service

Using NSSM (Non-Sucking Service Manager):

### Install NSSM
```powershell
# Download and extract NSSM
Invoke-WebRequest -Uri "https://nssm.cc/release/nssm-2.24.zip" -OutFile "nssm.zip"
Expand-Archive -Path "nssm.zip" -DestinationPath "."
Copy-Item ".\nssm-2.24\win64\nssm.exe" -Destination "C:\Windows\System32"
```

### Install Service
```powershell
nssm install ForgejoRunner "C:\ForgejoRunner\bin\forgejo-runner.exe"
nssm set ForgejoRunner Arguments "daemon --config C:\ForgejoRunner\config\runner.yaml"
nssm set ForgejoRunner DisplayName "Forgejo CI Runner"
nssm set ForgejoRunner Description "Forgejo Actions runner for VideoTools builds"
nssm set ForgejoRunner Start SERVICE_AUTO_START
nssm set ForgejoRunner AppStdout "C:\ForgejoRunner\logs\runner.log"
nssm set ForgejoRunner AppStderr "C:\ForgejoRunner\logs\runner-error.log"
nssm set ForgejoRunner AppRotateFiles 1
nssm set ForgejoRunner AppRotateOnline 1
nssm set ForgejoRunner AppRotateBytes 10485760
```

### Set Service Permissions
```powershell
# Run as ci-runner user
nssm set ForgejoRunner ObjectName ".\ci-runner" [password]
```

### Start Service
```powershell
nssm start ForgejoRunner
sc query ForgejoRunner
```

## Step 5 - Toolchain Bootstrap for VideoTools

Create `C:\ForgejoRunner\bootstrap.ps1`:

```powershell
# VideoTools CI Runner Bootstrap
$ErrorActionPreference = "Stop"

Write-Host "Bootstrapping VideoTools CI environment..." -ForegroundColor Cyan

# Install Go (pinned version)
$goVersion = "1.25.1"
$goInstaller = "go$goVersion.windows-amd64.msi"
$goUrl = "https://go.dev/dl/$goInstaller"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Installing Go $goVersion..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $goUrl -OutFile $goInstaller
    Start-Process -FilePath "msiexec.exe" -ArgumentList "/i $goInstaller /quiet" -Wait
    Remove-Item $goInstaller
    [System.Environment]::SetEnvironmentVariable("PATH", [System.Environment]::GetEnvironmentVariable("PATH", "Machine") + ";C:\Program Files\Go\bin", "Machine")
}

# Install Git if missing
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
    Write-Host "Installing Git..." -ForegroundColor Yellow
    winget install --id Git.Git -e --source winget
}

# Install MSYS2 UCRT64 toolchain
$msys2Root = "C:\msys64"
if (-not (Test-Path $msys2Root)) {
    Write-Host "Installing MSYS2..." -ForegroundColor Yellow
    winget install --id MSYS2.MSYS2 -e --source winget
    
    # Initialize MSYS2 and install toolchain
    & "$msys2Root\usr\bin\bash.exe" -lc "pacman -Syu --noconfirm"
    & "$msys2Root\usr\bin\bash.exe" -lc "pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-toolchain base-devel"
}

# Add MSYS2 to PATH for ci-runner user
$msys2Bin = "$msys2Root\ucrt64\bin"
if ($env:PATH -notmatch [Regex]::Escape($msys2Bin)) {
    [System.Environment]::SetEnvironmentVariable("PATH", $env:PATH + ";$msys2Bin", "User")
}

# Set PowerShell execution policy
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force

# Verify installations
Write-Host "Environment verification:" -ForegroundColor Green
Write-Host "Go: $(go version)" -ForegroundColor White
Write-Host "Git: $(git --version)" -ForegroundColor White
Write-Host "GCC: $(gcc --version | Select-Object -First 1)" -ForegroundColor White
```

Run bootstrap as ci-runner:
```powershell
# Run as ci-runner user
Runas /user:ci-runner "powershell -ExecutionPolicy Bypass -File C:\ForgejoRunner\bootstrap.ps1"
```

## Step 6 - VideoTools Workflow

Update `.forgejo/workflows/videotools-windows.yml`:

```yaml
name: VideoTools Windows Build

on:
  workflow_dispatch:
  push:
    branches: [master, main]
    paths:
      - "internal/**"
      - "main.go"
      - "go.mod"
      - "go.sum"
      - "scripts/**"
      - "packaging/windows/**"
  pull_request:
    branches: [master, main]
    paths:
      - "internal/**"
      - "main.go"
      - "go.mod"
      - "go.sum"
      - "scripts/**"
      - "packaging/windows/**"

jobs:
  windows-build:
    runs-on: windows
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
          
      - name: Setup MSYS2
        uses: msys2/setup-msys2@v2
        with:
          update: true
          msystem: UCRT64
          location: C:\msys64
          install: >-
            mingw-w64-ucrt-x86_64-toolchain
            
      - name: Build VideoTools
        shell: pwsh
        env:
          VT_MSYS2_ROOT: C:\msys64
          VT_MSYS2_FLAVOR: ucrt64
        run: |
          $env:GOOS = "windows"
          $env:GOARCH = "amd64"
          $env:CGO_ENABLED = "1"
          
          # Add MSYS2 to PATH
          $toolchainBin = "C:\msys64\ucrt64\bin"
          $env:Path = "$toolchainBin;$env:Path"
          $env:CC = "gcc.exe"
          $env:CXX = "g++.exe"
          
          # Build
          go mod download
          go mod verify
          go build -ldflags="-s -w" -trimpath -o VideoTools.exe .
          
      - name: Run Tests
        shell: pwsh
        env:
          VT_MSYS2_ROOT: C:\msys64
          VT_MSYS2_FLAVOR: ucrt64
        run: |
          $toolchainBin = "C:\msys64\ucrt64\bin"
          $env:Path = "$toolchainBin;$env:Path"
          go test -v ./...
          
      - name: Package Artifacts
        shell: pwsh
        run: |
          mkdir -p dist/windows
          Copy-Item VideoTools.exe dist/windows/
          Copy-Item README.md dist/windows/ -ErrorAction SilentlyContinue
          Compress-Archive -Path dist/windows/* -DestinationPath VideoTools-windows.zip
          
      - name: Upload Artifacts
        uses: actions/upload-artifact@v3
        with:
          name: VideoTools-windows
          path: VideoTools-windows.zip
```

## Step 7 - Security & Hardening

### Security Checklist
- [ ] Runner token stored securely during registration (single-use)
- [ ] Service runs as non-privileged `ci-runner` user
- [ ] No admin privileges for runner account
- [ ] Secrets only accessed via Forgejo's secret system
- [ ] Workspace cleanup after each job
- [ ] Logs rotated and monitored

### Network Security
- Runner needs outbound HTTPS to Forgejo instance
- No inbound ports required
- Consider firewall rules limiting outbound to necessary domains

### Secrets Management
Never log or echo secrets in workflows:
```yaml
# Correct: use environment variables
env:
  MY_SECRET: ${{ secrets.MY_SECRET }}
  
# Incorrect: don't echo secrets
- run: echo ${{ secrets.MY_SECRET }}
```

## Troubleshooting

### Runner Shows Offline
1. Check service status: `sc query ForgejoRunner`
2. Check logs: `Get-Content C:\ForgejoRunner\logs\runner.log -Tail 50`
3. Verify network connectivity to Forgejo instance
4. Check runner configuration in `config\runner.yaml`

### Jobs Stuck Queued
1. Verify labels match: runner has `windows`, job uses `runs-on: windows`
2. Check runner capacity in config
3. Ensure runner is online in Forgejo UI

### Service Registration Fails
1. Run PowerShell as Administrator
2. Verify ci-runner user exists and has correct password
3. Check file permissions on C:\ForgejoRunner

### Toolchain Issues
1. Verify PATH includes MSYS2 UCRT64 bin directory
2. Test GCC compilation: `gcc --version`
3. Check Go installation: `go version`

### TLS/CA Issues
- Ensure Windows trusts your Forgejo instance's SSL certificate
- Add cert to Windows certificate store if using self-signed certs

## Maintenance

### Updates
1. Stop service: `nssm stop ForgejoRunner`
2. Replace binary in `C:\ForgejoRunner\bin\`
3. Start service: `nssm start ForgejoRunner`
4. Update config if needed

### Monitoring
- Monitor `C:\ForgejoRunner\logs\runner.log`
- Check disk space for workspace and cache
- Review runner performance in Forgejo UI

## Validation

After setup, verify:
1. Runner appears online in Forgejo UI with correct labels
2. Test workflow runs successfully
3. Artifacts are produced correctly
4. Cleanup works after job completion

The runner should now be ready for VideoTools Windows CI builds on your Forgejo instance.