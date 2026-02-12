# VideoTools Dependency Installer for Windows
# Installs build and runtime dependencies using Chocolatey

param(
    [switch]$SkipFFmpeg = $false,
    [switch]$SkipGStreamer = $false,
    [switch]$SkipDVDStyler = $false,
    [switch]$InstallBuildTools = $false,
    [switch]$SkipBuildTools = $false,
    [switch]$InstallPython = $false,
    [switch]$SkipPython = $false,
    [switch]$SkipWhisper = $false,
    [switch]$InstallWhisper = $false,
    [string]$GStreamerVersion = "1.28.0",
    [string]$GStreamerRuntimeMsi = "",
    [string]$GStreamerDevelMsi = "",
    [switch]$PreferWinget = $false,
    [switch]$Silent = $false,
    [switch]$Auto = $false
)

# Colors for output
$RED = [ConsoleColor]::Red
$GREEN = [ConsoleColor]::Green
$YELLOW = [ConsoleColor]::Yellow
$BLUE = [ConsoleColor]::Blue
$CYAN = [ConsoleColor]::Cyan
$NC = [ConsoleColor]::White

# Configuration
$PROJECT_ROOT = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))

# Dependency status tracking (global scope)
$global:DependencyStatus = @{
    "golang" = $false
    "git" = $false
    "ffmpeg" = $false
    "python" = $false
    "gstreamer" = $false
    "whisper" = $false
    "dvdstyler" = $false
}

function Test-AllDependencies {
    Write-Color "Checking existing dependencies..." $CYAN
    
    # Check Chocolatey packages
    if (Test-Command choco) {
        $golangResult = Test-PackageInstalled -PackageName "golang"
        if ($golangResult) {
            $global:DependencyStatus.golang = $true
            Write-Color "[OK] Go programming language already installed" $GREEN
        }
        
        $gitResult = Test-PackageInstalled -PackageName "git"
        if ($gitResult) {
            $global:DependencyStatus.git = $true
            Write-Color "[OK] Git version control already installed" $GREEN
        }
        
        $ffmpegResult = Test-PackageInstalled -PackageName "ffmpeg"
        if ($ffmpegResult) {
            $global:DependencyStatus.ffmpeg = $true
            Write-Color "[OK] FFmpeg video processing already installed" $GREEN
        }
        
        # Check for Python directly via command first, then fallback to Chocolatey
        if ((Test-Command python) -and (Test-Command pip)) {
            $global:DependencyStatus.python = $true
            Write-Color "[OK] Python with pip already installed" $GREEN
        } elseif (Test-PackageInstalled -PackageName "python") {
            # Fallback: Check if Chocolatey has it but pip might not be in PATH yet
            if (Test-Command pip) {
                $global:DependencyStatus.python = $true
                Write-Color "[OK] Python with pip already installed" $GREEN
            } else {
                Write-Color "[WARN] Python installed via Chocolatey but pip not found in PATH" $YELLOW
            }
        }
    }
    
    # Check GStreamer - FIXED: Look for correct DLL name and add directory fallback
    $gstreamerPaths = @(
        "C:\GStreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\Program Files\GStreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\Program Files (x86)\GStreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\gstreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\Program Files\gstreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\Program Files (x86)\gstreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll",
        "C:\msys64\mingw64\bin\gstreamer-1.0-0.dll",
        "C:\GStreamer\1.0\x86_64\bin\gstreamer-1.0-0.dll",
        "C:\gstreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll"
    )
    
    $foundGStreamer = $false
    foreach ($path in $gstreamerPaths) {
        if (Test-Path $path) {
            $global:DependencyStatus.gstreamer = $true
            Write-Color "[OK] GStreamer already installed" $GREEN
            Write-Color "       Found at: $path" $CYAN
            $foundGStreamer = $true
            break
        }
    }
    
    # Fallback: Check for GStreamer directory existence if DLL not found
    if (-not $foundGStreamer) {
        $gstreamerDirs = @(
            "C:\Program Files\GStreamer",
            "C:\Program Files (x86)\GStreamer",
            "C:\GStreamer",
            "C:\gstreamer"
        )
        
        foreach ($dir in $gstreamerDirs) {
            if (Test-Path $dir) {
                # Check if bin subdirectory exists with any gstreamer DLL
                $binPath = Join-Path $dir "1.0\msvc_x86_64\bin"
                if (Test-Path $binPath) {
                    $gstreamerDlls = Get-ChildItem -Path $binPath -Filter "gstreamer*.dll" -ErrorAction SilentlyContinue
                    if ($gstreamerDlls.Count -gt 0) {
                        $global:DependencyStatus.gstreamer = $true
                        Write-Color "[OK] GStreamer already installed" $GREEN
                        Write-Color "       Found at: $dir" $CYAN
                        $foundGStreamer = $true
                        break
                    }
                }
            }
        }
    }
    
    # Check DVDStyler - FIXED: Add more comprehensive search
    $dvdstylerPaths = @(
        "C:\Program Files\DVDStyler\DVDStyler.exe",
        "C:\Program Files (x86)\DVDStyler\DVDStyler.exe",
        "C:\DVDStyler\DVDStyler.exe",
        "C:\Program Files\DVDStyler\bin\DVDStyler.exe",
        "C:\Program Files (x86)\DVDStyler\bin\DVDStyler.exe",
        "C:\DVDStyler\bin\DVDStyler.exe"
    )
    
    $foundDVDStyler = $false
    foreach ($path in $dvdstylerPaths) {
        if (Test-Path $path) {
            $global:DependencyStatus.dvdstyler = $true
            Write-Color "[OK] DVDStyler already installed" $GREEN
            $foundDVDStyler = $true
            break
        }
    }
    
    # Fallback: Check for DVDStyler directory existence
    if (-not $foundDVDStyler) {
        $dvdstylerDirs = @(
            "C:\Program Files\DVDStyler",
            "C:\Program Files (x86)\DVDStyler",
            "C:\DVDStyler"
        )
        
        foreach ($dir in $dvdstylerDirs) {
            if (Test-Path $dir) {
                # Check if main executable exists
                $exeFiles = Get-ChildItem -Path $dir -Filter "DVDStyler.exe" -ErrorAction SilentlyContinue
                if ($exeFiles.Count -gt 0) {
                    $global:DependencyStatus.dvdstyler = $true
                    Write-Color "[OK] DVDStyler already installed" $GREEN
                    $foundDVDStyler = $true
                    break
                }
            }
        }
    }
    
    # Check Whisper model
    $modelDir = Join-Path $env:USERPROFILE "Videos\VideoTools\models"
    $whisperPaths = @(
        (Join-Path $modelDir "whisper-small.bin"),
        (Join-Path $modelDir "ggml-small.bin"),
        (Join-Path $modelDir "whisper-model.bin")
    )
    
    foreach ($path in $whisperPaths) {
        if (Test-Path $path) {
            $global:DependencyStatus.whisper = $true
            Write-Color "[OK] Whisper model already exists" $GREEN
            break
        }
    }
    
    Write-Host ""
}

function Write-Color {
    param(
        [string]$Message,
        [ConsoleColor]$Color = $NC
    )
    Write-Host $Message -ForegroundColor $Color
}

function Write-Header {
    param([string]$Title)
    $line = "==============================================================="
    Write-Color $line $CYAN
    Write-Color "  $Title" $CYAN
    Write-Color $line $CYAN
    Write-Host ""
}

function Write-ProgressBar {
    param(
        [string]$Activity,
        [string]$Status = "Processing...",
        [int]$PercentComplete = 0,
        [int]$SecondsRemaining = -1
    )
    
    $progressParams = @{
        Activity = $Activity
        Status = $Status
        PercentComplete = $PercentComplete
    }
    
    if ($SecondsRemaining -ge 0) {
        $progressParams.SecondsRemaining = $SecondsRemaining
    }
    
    Write-Progress @progressParams
}

function Copy-FileWithProgress {
    param(
        [string]$Source,
        [string]$Destination,
        [string]$Activity = "Copying file"
    )
    
    try {
        $sourceFile = Get-Item $Source -ErrorAction Stop
        $totalBytes = $sourceFile.Length
        $bytesCopied = 0
        
        # Create file stream for reading
        $fileStream = [System.IO.File]::OpenRead($Source)
        $memoryStream = New-Object System.IO.MemoryStream
        $buffer = New-Object byte[] 8192
        
        try {
            # Copy with progress tracking
            while (($bytesRead = $fileStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
                $memoryStream.Write($buffer, 0, $bytesRead)
                $bytesCopied += $bytesRead
                
                $percentComplete = [math]::Round(($bytesCopied / $totalBytes) * 100, 1)
                Write-ProgressBar -Activity $Activity -Status "Copying... ($([math]::Round($bytesCopied/1MB, 1))MB / $([math]::Round($totalBytes/1MB, 1))MB)" -PercentComplete $percentComplete
            }
            
            # Write to destination
            $destinationBytes = $memoryStream.ToArray()
            [System.IO.File]::WriteAllBytes($Destination, $destinationBytes)
            
            Write-Progress -Activity $Activity -Completed
            Write-Color "[OK] File copied successfully" $GREEN
            
        } finally {
            $fileStream.Dispose()
            $memoryStream.Dispose()
        }
    } catch {
        Write-Progress -Activity $Activity -Completed
        Write-Color "[ERROR] Failed to copy file: $($_.Exception.Message)" $RED
        throw
    }
}

function Test-InstallerExitCode {
    param([int]$ExitCode, [string]$ComponentName)
    
    switch ($ExitCode) {
        0 { 
            Write-Color "[OK] $ComponentName installation completed" $GREEN
            return $true 
        }
        3010 { 
            Write-Color "[OK] $ComponentName installation completed (reboot required)" $GREEN
            return $true 
        }
        1602 { 
            Write-Color "[CANCELLED] User cancelled $ComponentName installation" $YELLOW
            return $false 
        }
        1603 { 
            Write-Color "[ERROR] $ComponentName installation failed" $RED
            return $false 
        }
        1641 { 
            Write-Color "[OK] $ComponentName installation completed (reboot initiated)" $GREEN
            return $true 
        }
        default { 
            Write-Color "[ERROR] $ComponentName installation failed (code: $ExitCode)" $RED
            return $false 
        }
    }
}

function Test-InstallationComplete {
    param(
        [string]$ComponentName,
        [hashtable]$ExpectedFiles
    )
    
    $allFilesFound = $true
    foreach ($file in $ExpectedFiles.GetEnumerator()) {
        if (Test-Path $file.Value) {
            Write-Color "[OK] Verified: $($file.Key)" $GREEN
        } else {
            Write-Color "[ERROR] Missing: $($file.Key)" $RED
            $allFilesFound = $false
        }
    }
    
    if ($allFilesFound) {
        Write-Color "[OK] $ComponentName verification completed" $GREEN
        return $true
    } else {
        Write-Color "[ERROR] $ComponentName verification failed" $RED
        return $false
    }
}

function Test-Command {
    param([string]$Command)
    try {
        Get-Command $Command -ErrorAction Stop | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Test-PackageInstalled {
    param([string]$PackageName)
    if (Test-Command choco) {
        try {
            $installed = choco list --local-only --exact $PackageName
            return $installed -match "$PackageName\s+\d"
        } catch {
            return $false
        }
    }
    return $false
}

function Install-Chocolatey {
    Write-Color "[1/4] Installing Chocolatey package manager..." $CYAN
    try {
        Set-ExecutionPolicy Bypass -Scope Process -Force
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
        iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
        
        # Refresh environment variables
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
        
        if (Test-Command choco) {
            Write-Color "[OK] Chocolatey installed successfully" $GREEN
            choco upgrade -y
        } else {
            throw "Chocolatey installation failed"
        }
    } catch {
        Write-Color "[ERROR] Failed to install Chocolatey: $($_.Exception.Message)" $RED
        return $false
    }
    return $true
}

function Install-Package {
    param(
        [string]$PackageName,
        [string]$DisplayName = $PackageName,
        [switch]$Required = $false
    )
    
    if (Test-Command choco) {
        try {
            if (Test-PackageInstalled -PackageName $PackageName) {
                Write-Color "[OK] $DisplayName already installed" $GREEN
                return $true
            } else {
                Write-Color "Installing $DisplayName..." $YELLOW
                $result = choco install $PackageName -y --accept-license --ignore-package-exit-codes
                if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 3010) {
                    Write-Color "[OK] $DisplayName installed successfully" $GREEN
                    return $true
                } else {
                    if ($Required) {
                        Write-Color "[ERROR] Failed to install required package $DisplayName" $RED
                        return $false
                    } else {
                        Write-Color "[SKIP] Failed to install $DisplayName (optional)" $YELLOW
                        return $true
                    }
                }
            }
        } catch {
            Write-Color "[ERROR] Error installing $DisplayName`: $($_.Exception.Message)" $RED
            if ($Required) { return $false }
            return $true
        }
    }
    return $false
}

function Install-GStreamer {
    if ($SkipGStreamer) {
        Write-Color "[SKIP] Skipping GStreamer installation" $YELLOW
        return $true
    }

    # Check if already installed globally (before any operations)
    if ($global:DependencyStatus.gstreamer) {
        Write-Color "[SKIP] GStreamer already installed, skipping" $GREEN
        return $true
    }

    Write-Color "[3/4] Installing GStreamer (required for video playback)..." $CYAN
    
    if ($PreferWinget -and (Test-Command winget)) {
        try {
            Write-Color "Attempting to install GStreamer via winget..." $YELLOW
            winget install --id GStreamer.GStreamer -e --accept-package-agreements --accept-source-agreements
            if ($LASTEXITCODE -eq 0) {
                Write-Color "[OK] GStreamer installed via winget" $GREEN
                return $true
            }
        } catch {
            Write-Color "Winget installation failed, trying MSI approach..." $YELLOW
        }
    }

    # Mirror installation approach
    try {

        Write-Color "Downloading GStreamer installer from mirror..." $YELLOW
        $installerExe = Join-Path $env:TEMP "gstreamer-installer.exe"
        $tempRepo = Join-Path $env:TEMP "lt_mirror_temp"
        
        try {
            # Clone mirror repo locally to get LFS files (GStreamer site blocks bots)
            if (Test-Path $tempRepo) {
                Remove-Item $tempRepo -Recurse -Force
            }
            
            Write-Color "Cloning mirror repository (GStreamer site blocks direct downloads)..." $YELLOW
            Write-ProgressBar -Activity "Downloading GStreamer" -Status "Connecting to repository..." -PercentComplete 10
            & git clone --depth 1 --progress https://git.leaktechnologies.dev/lt_mirror/lt_mirror.git $tempRepo
            Write-ProgressBar -Activity "Downloading GStreamer" -Completed
            if ($LASTEXITCODE -eq 0) {
                $sourceFile = Join-Path $tempRepo "mirrors\raw\gstreamer-1.0-msvc-x86_64-$($GStreamerVersion).exe"
                if (Test-Path $sourceFile) {
                    Copy-FileWithProgress -Source $sourceFile -Destination $installerExe -Activity "Downloading GStreamer installer"
                    Write-Color "[OK] GStreamer installer extracted from mirror" $GREEN
                } else {
                    throw "GStreamer file not found in cloned repository"
                }
            } else {
                throw "Failed to clone mirror repository"
            }
        } catch {
            Write-Color "[ERROR] Failed to get GStreamer from mirror: $($_.Exception.Message)" $RED
            Write-Color "       Manual install required: https://gstreamer.freedesktop.org/download/" $YELLOW
            return $false
        } finally {
            # Clean up temporary repository
            if (Test-Path $tempRepo) {
                Remove-Item $tempRepo -Recurse -Force -ErrorAction SilentlyContinue
            }
        }

        Write-Color "Installing GStreamer..." $YELLOW
        $process = Start-Process -FilePath $installerExe -ArgumentList "/S" -Wait -PassThru -NoNewWindow
        
        $installSuccess = Test-InstallerExitCode -ExitCode $process.ExitCode -ComponentName "GStreamer"
        if ($installSuccess) {
            # Verify installation
            $expectedFiles = @{
                "GStreamer DLL" = "C:\Program Files\GStreamer\1.0\msvc_x86_64\bin\gstreamer-1.0-0.dll"
            }
            $verificationSuccess = Test-InstallationComplete -ComponentName "GStreamer" -ExpectedFiles $expectedFiles
            
            if ($verificationSuccess) {
                $global:DependencyStatus.gstreamer = $true
                return $true
            } else {
                Write-Color "[ERROR] GStreamer installation verification failed" $RED
                return $false
            }
        } else {
            Write-Color "[ERROR] GStreamer installation failed or was cancelled" $RED
            return $false
        }
    } catch {
        Write-Color "[ERROR] Failed to install GStreamer: $($_.Exception.Message)" $RED
        return $false
    }
}

function Install-WhisperModel {
    if ($SkipWhisper) {
        Write-Color "[SKIP] Skipping Whisper model installation" $YELLOW
        return
    }

    # Check if already installed globally (before any operations)
    if ($global:DependencyStatus.whisper) {
        Write-Color "[SKIP] Whisper model already exists, skipping" $GREEN
        return
    }

    Write-Color "Installing Whisper model for subtitles..." $CYAN
    try {
        $modelDir = Join-Path $env:USERPROFILE "Videos\VideoTools\models"
        if (-not (Test-Path $modelDir)) {
            New-Item -ItemType Directory -Path $modelDir -Force | Out-Null
        }

        $modelPath = Join-Path $modelDir "whisper-small.bin"

        # Use lt_mirror for Whisper model (sourcing issues from HuggingFace)
        $tempRepo = Join-Path $env:TEMP "lt_mirror_temp"
        
        try {
            # Clone mirror repo locally to get Whisper model
            if (Test-Path $tempRepo) {
                Remove-Item $tempRepo -Recurse -Force
            }
            
            Write-Color "Getting Whisper model from lt_mirror..." $YELLOW
            Write-ProgressBar -Activity "Downloading Whisper model" -Status "Connecting to repository..." -PercentComplete 10
            & git clone --depth 1 --progress https://git.leaktechnologies.dev/lt_mirror/lt_mirror.git $tempRepo
            Write-ProgressBar -Activity "Downloading Whisper model" -Completed
            if ($LASTEXITCODE -eq 0) {
                $sourceFile = Join-Path $tempRepo "mirrors\raw\whisper-model.bin"
                if (Test-Path $sourceFile) {
                    Copy-FileWithProgress -Source $sourceFile -Destination $modelPath -Activity "Downloading Whisper model"
                    Write-Color "[OK] Whisper model installed from mirror" $GREEN
                } else {
                    throw "Whisper model not found in mirror"
                }
            } else {
                throw "Failed to clone mirror repository"
            }
        } catch {
            Write-Color "[SKIP] Failed to get Whisper model from mirror: $($_.Exception.Message)" $YELLOW
            Write-Color "       Manual install required" $YELLOW
        } finally {
            # Clean up temporary repository
            if (Test-Path $tempRepo) {
                Remove-Item $tempRepo -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    } catch {
        Write-Color "[SKIP] Whisper model installation failed: $($_.Exception.Message)" $YELLOW
    }
}

function Install-DVDStyler {
    if ($SkipDVDStyler) {
        Write-Color "[SKIP] Skipping DVDStyler installation" $YELLOW
        return $true
    }

    # Check if already installed globally (before any operations)
    if ($global:DependencyStatus.dvdstyler) {
        Write-Color "[SKIP] DVDStyler already installed, skipping" $GREEN
        return $true
    }

    Write-Color "[4/4] Installing DVDStyler (optional DVD authoring)..." $CYAN
    
    # Try winget first if available and preferred
    if ($PreferWinget -and (Test-Command winget)) {
        try {
            Write-Color "Attempting to install DVDStyler via winget..." $YELLOW
            winget install --id DVDStyler.DVDStyler -e --accept-package-agreements --accept-source-agreements
            if ($LASTEXITCODE -eq 0) {
                Write-Color "[OK] DVDStyler installed via winget" $GREEN
                return $true
            }
        } catch {
            Write-Color "Winget installation failed, trying mirror..." $YELLOW
        }
    }

    # Use lt_mirror for DVDStyler
    $installerExe = Join-Path $env:TEMP "DVDStyler-setup.exe"
    $tempRepo = Join-Path $env:TEMP "lt_mirror_temp"
    
    try {
        # Clone mirror repo locally to get DVDStyler
        if (Test-Path $tempRepo) {
            Remove-Item $tempRepo -Recurse -Force
        }
        
        Write-Color "Getting DVDStyler from lt_mirror..." $YELLOW
        Write-ProgressBar -Activity "Downloading DVDStyler" -Status "Connecting to repository..." -PercentComplete 10
        & git clone --depth 1 --progress https://git.leaktechnologies.dev/lt_mirror/lt_mirror.git $tempRepo
        Write-ProgressBar -Activity "Downloading DVDStyler" -Completed
        if ($LASTEXITCODE -eq 0) {
            $sourceFile = Join-Path $tempRepo "mirrors\raw\DVDStyler-3.2.1-win64.exe"
            if (Test-Path $sourceFile) {
                Copy-FileWithProgress -Source $sourceFile -Destination $installerExe -Activity "Downloading DVDStyler"
                Write-Color "Installing DVDStyler..." $YELLOW
                $process = Start-Process -FilePath $installerExe -ArgumentList "/S" -Wait -PassThru -NoNewWindow
                
                $installSuccess = Test-InstallerExitCode -ExitCode $process.ExitCode -ComponentName "DVDStyler"
                if ($installSuccess) {
                    # Verify installation
                    $expectedFiles = @{
                        "DVDStyler executable" = "C:\Program Files\DVDStyler\bin\DVDStyler.exe"
                    }
                    $verificationSuccess = Test-InstallationComplete -ComponentName "DVDStyler" -ExpectedFiles $expectedFiles
                    
                    if ($verificationSuccess) {
                        $global:DependencyStatus.dvdstyler = $true
                        return $true
                    } else {
                        Write-Color "[ERROR] DVDStyler installation verification failed" $RED
                        return $false
                    }
                } else {
                    Write-Color "[ERROR] DVDStyler installation failed or was cancelled" $RED
                    return $false
                }
            } else {
                throw "DVDStyler not found in mirror"
            }
        } else {
            throw "Failed to clone mirror repository"
        }
    } catch {
        Write-Color "[SKIP] Failed to install DVDStyler from mirror: $($_.Exception.Message)" $YELLOW
        return $false
    } finally {
        # Clean up temporary files
        if (Test-Path $tempRepo) {
            Remove-Item $tempRepo -Recurse -Force -ErrorAction SilentlyContinue
        }
        if (Test-Path $installerExe) {
            Remove-Item $installerExe -ErrorAction SilentlyContinue
        }
    }
}

# Main installation flow
Write-Header "VideoTools Windows Installation"

# Check all dependencies first
Test-AllDependencies

# Ask user about optional features (unless silent mode or already installed)
$installDVDStyler = $false
$installWhisper = $false

if (-not ($Silent -or $Auto)) {
    Write-Host ""
    Write-Color "Optional Features:" $CYAN
    Write-Host ""
    
    # Only ask about DVDStyler if not already installed
    if (-not $global:DependencyStatus.dvdstyler) {
        Write-Host "Install DVD Authoring Tools (DVDStyler)? [y/N]: " -ForegroundColor Yellow -NoNewline
        $responseDVD = Read-Host
        if ($responseDVD -match '^[Yy]') {
            $installDVDStyler = $true
        }
    } else {
        Write-Color "[SKIP] DVDStyler already installed, skipping prompt" $GREEN
    }
    
    # Only ask about Whisper if not already installed
    if (-not $global:DependencyStatus.whisper) {
        Write-Host "Install AI Subtitle Generation (Whisper models)? [y/N]: " -ForegroundColor Yellow -NoNewline
        $responseWhisper = Read-Host
        if ($responseWhisper -match '^[Yy]') {
            $installWhisper = $true
        }
    } else {
        Write-Color "[SKIP] Whisper model already exists, skipping prompt" $GREEN
    }
    Write-Host ""
}

# Check admin privileges for GStreamer MSI
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Color "[WARN] Running without Administrator privileges." $YELLOW
    Write-Color "       GStreamer install requires Administrator when missing." $YELLOW
    Write-Host ""
}

# Install Chocolatey
if (-not (Test-Command choco)) {
    if (-not (Install-Chocolatey)) {
        Write-Color "[ERROR] Chocolatey installation failed. Cannot proceed." $RED
        Write-Host "Press any key to close..." $CYAN
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }
}

# Determine build tools installation
$installBuild = $false
if ($InstallBuildTools) { $installBuild = $true }
elseif ($SkipBuildTools) { $installBuild = $false }
else {
    $installBuild = $true  # Default to installing build tools
}

# Install core packages
if ($installBuild) {
    Write-Color "[2/4] Installing build tools..." $CYAN
    if (-not $DependencyStatus.golang) {
        Install-Package -PackageName "golang" -DisplayName "Go programming language" -Required
    } else {
        Write-Color "[SKIP] Go already installed, skipping" $GREEN
    }
    if (-not $DependencyStatus.git) {
        Install-Package -PackageName "git" -DisplayName "Git version control" -Required
    } else {
        Write-Color "[SKIP] Git already installed, skipping" $GREEN
    }
} else {
    Write-Color "[SKIP] Skipping build tools installation" $YELLOW
}

# Install FFmpeg
if (-not $SkipFFmpeg) {
    if (-not $DependencyStatus.ffmpeg) {
        Install-Package -PackageName "ffmpeg" -DisplayName "FFmpeg video processing"
    } else {
        Write-Color "[SKIP] FFmpeg already installed, skipping" $GREEN
    }
} else {
    Write-Color "[SKIP] Skipping FFmpeg installation" $YELLOW
}

# Install Python
if ($InstallPython) {
    Install-Package -PackageName "python" -DisplayName "Python with pip"
} elseif ($SkipPython) {
    Write-Color "[SKIP] Skipping Python installation" $YELLOW
} elseif ($DependencyStatus.python) {
    Write-Color "[SKIP] Python already installed, skipping" $GREEN
} elseif ($Silent -or $Auto) {
    # In silent/auto mode, skip Python (optional)
    Write-Color "[SKIP] Skipping Python installation (silent mode)" $YELLOW
} else {
    Write-Host "Install Python + pip? [y/N]: " -ForegroundColor Yellow -NoNewline
    $response = Read-Host
    if ($response -match '^[Yy]') {
        Install-Package -PackageName "python" -DisplayName "Python with pip"
    } else {
        Write-Color "[SKIP] Skipping Python installation" $YELLOW
    }
}

# Install GStreamer - FIXED: Respect dependency status in all modes
if (-not $global:DependencyStatus.gstreamer) {
    if (-not (Install-GStreamer)) {
        Write-Color "[WARN] GStreamer installation failed. Video playback may not work." $YELLOW
        Write-Color "       You can install GStreamer manually from: https://gstreamer.freedesktop.org/download/" $YELLOW
    }
} else {
    Write-Color "[SKIP] GStreamer already installed, skipping" $GREEN
}

# Install Whisper model
if ($installWhisper) {
    Install-WhisperModel
} elseif ($SkipWhisper) {
    Write-Color "[SKIP] Skipping Whisper model installation" $YELLOW
} elseif ($DependencyStatus.whisper) {
    Write-Color "[SKIP] Whisper model already exists, skipping" $GREEN
} elseif ($Silent -or $Auto) {
    # In silent/auto mode, install Whisper automatically
    Install-WhisperModel
} else {
    Write-Color "[SKIP] Whisper model installation declined" $YELLOW
}

# Install DVDStyler - FIXED: Respect dependency status in all modes
if ($installDVDStyler) {
    if (-not $DependencyStatus.dvdstyler) {
        if (-not (Install-DVDStyler)) {
            Write-Color "[INFO] DVDStyler installation failed. DVD authoring tools unavailable." $YELLOW
        }
    } else {
        Write-Color "[SKIP] DVDStyler already installed, skipping" $GREEN
    }
} elseif ($SkipDVDStyler) {
    Write-Color "[SKIP] Skipping DVDStyler installation" $YELLOW
} elseif ($Silent -or $Auto) {
    # In silent/auto mode, install DVDStyler automatically if not present
    if (-not $DependencyStatus.dvdstyler) {
        Install-DVDStyler
    } else {
        Write-Color "[SKIP] DVDStyler already installed, skipping" $GREEN
    }
} else {
    Write-Color "[SKIP] DVDStyler installation declined" $YELLOW
}

# Create shortcuts
try {
    $startMenuPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\VideoTools"
    if (-not (Test-Path $startMenuPath)) {
        New-Item -ItemType Directory -Path $startMenuPath -Force | Out-Null
    }

    $buildShortcut = Join-Path $startMenuPath "Build VideoTools.lnk"
    $buildScript = Join-Path $PSScriptRoot "build.ps1"
    
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($buildShortcut)
    $shortcut.TargetPath = "powershell.exe"
    $shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$buildScript`""
    $shortcut.WorkingDirectory = $PSScriptRoot
    $shortcut.Save()
    
    Write-Color "[OK] Start menu shortcuts created" $GREEN
} catch {
    Write-Color "[SKIP] Failed to create shortcuts: $($_.Exception.Message)" $YELLOW
}

Write-Color "[SUCCESS] VideoTools dependencies installation completed!" $GREEN
Write-Host ""
if (-not $Silent) {
    Write-Color "Next steps:" $CYAN
    Write-Color "  1. Run: .\scripts\windows\build.bat" $NC
    Write-Color "  2. Run: .\VideoTools.exe" $NC
    Write-Host ""
        Write-Color "Optional components status:" $CYAN
    if ($global:DependencyStatus.gstreamer) {
        Write-Color "  - GStreamer: Video playback support (installed)" $GREEN
    } else {
        Write-Color "  - GStreamer: Video playback support (not installed)" $NC
    }
    if ($global:DependencyStatus.whisper) {
        Write-Color "  - Whisper: AI subtitle generation (installed)" $GREEN
    } else {
        Write-Color "  - Whisper: AI subtitle generation (not installed)" $NC
    }
    if ($global:DependencyStatus.dvdstyler) {
        Write-Color "  - DVDStyler: DVD authoring tools (installed)" $GREEN
    } else {
        Write-Color "  - DVDStyler: DVD authoring tools (not installed)" $NC
    }
}
Write-Host ""
if (-not $Silent) {
    Write-Host "Press any key to close..." -ForegroundColor Cyan
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}