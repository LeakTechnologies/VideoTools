# VideoTools CI Runner Bootstrap Script for Windows
# Sets up the toolchain for VideoTools builds on Windows CI runners

param(
    [switch]$Force = $false,
    [switch]$SkipUpdates = $false
)

# Error handling
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

function Write-Header {
    param([string]$Title)
    $line = "════════════════════════════════════════════════════════════════"
    Write-Host $line -ForegroundColor Cyan
    Write-Host "  $Title" -ForegroundColor Cyan
    Write-Host $line -ForegroundColor Cyan
    Write-Host ""
}

function Write-Section {
    param([string]$Title)
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

function Invoke-WithRetry {
    param(
        [scriptblock]$ScriptBlock,
        [int]$MaxRetries = 3,
        [int]$Delay = 5
    )
    $attempt = 1
    while ($attempt -le $MaxRetries) {
        try {
            & $ScriptBlock
            return
        } catch {
            if ($attempt -eq $MaxRetries) {
                throw
            }
            Write-Host "Attempt $attempt failed, retrying in $Delay seconds..." -ForegroundColor Yellow
            Start-Sleep -Seconds $Delay
            $attempt++
        }
    }
}

Write-Header "VideoTools CI Runner Bootstrap"

# Get system information
$osInfo = Get-CimInstance Win32_OperatingSystem
$cpuInfo = Get-CimInstance Win32_Processor
$memInfo = Get-CimInstance Win32_ComputerSystem

Write-Host "System Information:" -ForegroundColor Green
Write-Host "  OS: $($osInfo.Caption) $($osInfo.Version) (Build $($osInfo.BuildNumber))" -ForegroundColor White
Write-Host "  CPU: $($cpuInfo.Name)" -ForegroundColor White
Write-Host "  RAM: $([math]::Round($memInfo.TotalPhysicalMemory / 1GB, 2)) GB" -ForegroundColor White
Write-Host ""

Write-Section "PowerShell Configuration"

# Set execution policy
try {
    $currentPolicy = Get-ExecutionPolicy -Scope CurrentUser
    if ($currentPolicy -ne "RemoteSigned" -and $currentPolicy -ne "Unrestricted") {
        Write-Host "Setting execution policy to RemoteSigned..." -ForegroundColor Yellow
        Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
    }
    Write-Host "Execution policy: $(Get-ExecutionPolicy -Scope CurrentUser)" -ForegroundColor Green
} catch {
    Write-Host "Failed to set execution policy: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Section "Package Managers"

# Install winget if not available
if (-not (Test-Command winget)) {
    Write-Host "Installing winget..." -ForegroundColor Yellow
    try {
        # Download and install Microsoft Store App Installer
        $wingetUrl = "https://github.com/microsoft/winget-cli/releases/latest/download/Microsoft.DesktopAppInstaller_8wekyb3d8bbwe.msixbundle"
        $wingetPath = Join-Path $env:TEMP "Microsoft.DesktopAppInstaller.msixbundle"
        Invoke-WithRetry { 
            Invoke-WebRequest -Uri $wingetUrl -OutFile $wingetPath -UseBasicParsing
        }
        Add-AppxPackage -Path $wingetPath
        Remove-Item $wingetPath -Force
        Write-Host "Winget installed successfully" -ForegroundColor Green
    } catch {
        Write-Host "Failed to install winget: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "Winget: $(winget --version)" -ForegroundColor Green
}

Write-Section "Git Installation"

# Install Git if missing
if (-not (Test-Command git)) {
    Write-Host "Installing Git..." -ForegroundColor Yellow
    try {
        Invoke-WithRetry {
            winget install --id Git.Git -e --source winget --accept-package-agreements --accept-source-agreements
        }
        # Add Git to PATH for current session
        $gitPath = "${env:ProgramFiles}\Git\cmd"
        if ($env:PATH -notmatch [Regex]::Escape($gitPath)) {
            $env:PATH = "$gitPath;$env:PATH"
        }
        Write-Host "Git installed successfully" -ForegroundColor Green
    } catch {
        Write-Host "Failed to install Git: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "Git: $(git --version)" -ForegroundColor Green
}

Write-Section "Go Installation"

# Go version pinned to match go.mod
$goVersion = "1.26.1"
$goInstalled = $false

if (Test-Command go) {
    $installedVersion = (go version).Split(' ')[2] -replace 'go', ''
    if ($installedVersion -eq $goVersion) {
        Write-Host "Go $goVersion is already installed" -ForegroundColor Green
        $goInstalled = $true
    } elseif (-not $Force) {
        Write-Host "Go $installedVersion found, but $goVersion is required. Use -Force to reinstall." -ForegroundColor Yellow
    }
}

if (-not $goInstalled) {
    Write-Host "Installing Go $goVersion..." -ForegroundColor Yellow
    try {
        $goInstaller = "go$goVersion.windows-amd64.msi"
        $goUrl = "https://go.dev/dl/$goInstaller"
        $goPath = Join-Path $env:TEMP $goInstaller
        
        Invoke-WithRetry {
            Invoke-WebRequest -Uri $goUrl -OutFile $goPath -UseBasicParsing
        }
        
        $installArgs = "/i `"$goPath`" /quiet INSTALLDIR=`"${env:ProgramFiles}\Go`""
        Start-Process -FilePath "msiexec.exe" -ArgumentList $installArgs -Wait -NoNewWindow
        
        Remove-Item $goPath -Force
        
        # Add Go to PATH
        $goBinPath = "${env:ProgramFiles}\Go\bin"
        $machinePath = [System.Environment]::GetEnvironmentVariable("PATH", "Machine")
        if ($machinePath -notmatch [Regex]::Escape($goBinPath)) {
            [System.Environment]::SetEnvironmentVariable("PATH", "$machinePath;$goBinPath", "Machine")
        }
        $env:PATH = "$goBinPath;$env:PATH"
        
        Write-Host "Go $goVersion installed successfully" -ForegroundColor Green
    } catch {
        Write-Host "Failed to install Go: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
}

Write-Section "MSYS2 Installation"

$msys2Root = "E:\dependencies\msys64"
$msys2Installed = $false

if (Test-Path $msys2Root) {
    Write-Host "MSYS2 found at $msys2Root" -ForegroundColor Green
    $msys2Installed = $true
} else {
    Write-Host "Installing MSYS2..." -ForegroundColor Yellow
    try {
        Invoke-WithRetry {
            winget install --id MSYS2.MSYS2 -e --source winget --accept-package-agreements --accept-source-agreements
        }
        Write-Host "MSYS2 installed successfully" -ForegroundColor Green
    } catch {
        Write-Host "Failed to install MSYS2: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
}

Write-Section "MSYS2 Toolchain"

# Initialize MSYS2 and install toolchain
$bashPath = Join-Path $msys2Root "usr\bin\bash.exe"
if (-not (Test-Path $bashPath)) {
    Write-Host "MSYS2 bash not found at $bashPath" -ForegroundColor Red
    exit 1
}

# Update MSYS2 package database
if (-not $SkipUpdates) {
    Write-Host "Updating MSYS2 package database..." -ForegroundColor Yellow
    try {
        & $bashPath -lc "pacman -Syu --noconfirm" -ErrorAction Stop
        Write-Host "MSYS2 package database updated" -ForegroundColor Green
    } catch {
        Write-Host "Failed to update MSYS2: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
}

# Install toolchain packages
Write-Host "Installing MSYS2 toolchain packages..." -ForegroundColor Yellow
$packages = @(
    "base-devel",
    "mingw-w64-ucrt-x86_64-toolchain",
    "git",
    "wget",
    "curl"
)

$packageList = $packages -join " "
try {
    & $bashPath -lc "pacman -S --needed --noconfirm $packageList" -ErrorAction Stop
    Write-Host "MSYS2 toolchain packages installed" -ForegroundColor Green
} catch {
    Write-Host "Failed to install MSYS2 packages: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Section "Environment Configuration"

# Add MSYS2 UCRT64 to PATH
$msys2Bin = Join-Path $msys2Root "ucrt64\bin"
$machinePath = [System.Environment]::GetEnvironmentVariable("PATH", "Machine")
$userPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")

# Update machine PATH if needed
if ($machinePath -notmatch [Regex]::Escape($msys2Bin)) {
    try {
        [System.Environment]::SetEnvironmentVariable("PATH", "$machinePath;$msys2Bin", "Machine")
        Write-Host "Added MSYS2 UCRT64 to machine PATH" -ForegroundColor Green
    } catch {
        Write-Host "Warning: Could not update machine PATH. Updating session PATH only." -ForegroundColor Yellow
        $env:PATH = "$msys2Bin;$env:PATH"
    }
} else {
    Write-Host "MSYS2 UCRT64 already in PATH" -ForegroundColor Green
}

# Update current session PATH
if ($env:PATH -notmatch [Regex]::Escape($msys2Bin)) {
    $env:PATH = "$msys2Bin;$env:PATH"
}

Write-Section "Environment Verification"

# Verify installations
$allGood = $true

# Test Go
if (Test-Command go) {
    $goVersion = (go version)
    Write-Host "Go: $goVersion" -ForegroundColor Green
} else {
    Write-Host "ERROR: Go not found in PATH" -ForegroundColor Red
    $allGood = $false
}

# Test Git
if (Test-Command git) {
    $gitVersion = (git --version)
    Write-Host "Git: $gitVersion" -ForegroundColor Green
} else {
    Write-Host "ERROR: Git not found in PATH" -ForegroundColor Red
    $allGood = $false
}

# Test GCC
if (Test-Command gcc) {
    $gccVersion = (gcc --version | Select-Object -First 1)
    Write-Host "GCC: $gccVersion" -ForegroundColor Green
} else {
    Write-Host "ERROR: GCC not found in PATH" -ForegroundColor Red
    $allGood = $false
}

# Test G++
if (Test-Command g++) {
    $gppVersion = (g++ --version | Select-Object -First 1)
    Write-Host "G++: $gppVersion" -ForegroundColor Green
} else {
    Write-Host "ERROR: G++ not found in PATH" -ForegroundColor Red
    $allGood = $false
}

# Test windres (for Windows resources)
if (Test-Command windres) {
    $windresVersion = (windres --version | Select-Object -First 1)
    Write-Host "Windres: $windresVersion" -ForegroundColor Green
} else {
    Write-Host "WARNING: Windres not found (Windows icon embedding disabled)" -ForegroundColor Yellow
}

Write-Section "Test Compilation"

# Test compilation
$tempDir = Join-Path $env:TEMP "vt-gcc-test"
try {
    New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
    $cfile = Join-Path $tempDir "test.c"
    $ofile = Join-Path $tempDir "test.o"
    $exeFile = Join-Path $tempDir "test.exe"
    
    Set-Content -Path $cfile -Value @"
#include <stdio.h>
int main() {
    printf("Hello from VideoTools CI bootstrap!\\n");
    return 0;
}
"@ -Encoding ASCII
    
    & gcc -c $cfile -o $ofile 2>$null
    & gcc $ofile -o $exeFile 2>$null
    
    if (Test-Path $exeFile) {
        $output = & $exeFile 2>$null
        Write-Host "Test compilation successful: $output" -ForegroundColor Green
        Remove-Item $exeFile -Force
    } else {
        Write-Host "ERROR: Test compilation failed" -ForegroundColor Red
        $allGood = $false
    }
    
    Remove-Item $cfile -Force -ErrorAction SilentlyContinue
    Remove-Item $ofile -Force -ErrorAction SilentlyContinue
    Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
} catch {
    Write-Host "ERROR: Test compilation failed: $($_.Exception.Message)" -ForegroundColor Red
    $allGood = $false
}

Write-Section "Environment Variables"

# Set useful environment variables
[System.Environment]::SetEnvironmentVariable("VT_MSYS2_ROOT", $msys2Root, "User")
[System.Environment]::SetEnvironmentVariable("VT_MSYS2_FLAVOR", "ucrt64", "User")
Write-Host "Set VT_MSYS2_ROOT=$msys2Root" -ForegroundColor Green
Write-Host "Set VT_MSYS2_FLAVOR=ucrt64" -ForegroundColor Green

Write-Host ""
if ($allGood) {
    Write-Header "Bootstrap Complete"
    Write-Host "VideoTools CI environment is ready!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Environment summary:" -ForegroundColor Cyan
    Write-Host "  Go: $(go version)" -ForegroundColor White
    Write-Host "  Git: $(git --version)" -ForegroundColor White
    Write-Host "  GCC: $(gcc --version | Select-Object -First 1)" -ForegroundColor White
    Write-Host "  MSYS2: $msys2Root" -ForegroundColor White
    Write-Host ""
    Write-Host "The runner is ready for VideoTools builds." -ForegroundColor Green
    exit 0
} else {
    Write-Header "Bootstrap Failed"
    Write-Host "Some components failed to install. Check the errors above." -ForegroundColor Red
    exit 1
}