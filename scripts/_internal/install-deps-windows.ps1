# VideoTools Dependency Installer for Windows
# Installs build and runtime dependencies using MSYS2 + winget

param(
    [switch]$SkipFFmpeg = $false,
    [switch]$SkipGStreamer = $false,
    [switch]$InstallBuildTools = $false,
    [switch]$SkipBuildTools = $false,
    [switch]$InstallPython = $false,
    [switch]$SkipPython = $false,
    [string]$DvdStylerUrl = "",
    [string]$DvdStylerExeUrl = "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/DVDStyler-3.2.1.-win64.exe",
    [string]$DvdStylerExeArgs = "/S",
    [string]$DvdStylerZip = "",
    [switch]$SkipDvdStyler = $false,
    [switch]$InstallWhisper = $false,
    [switch]$SkipWhisper = $false,
    [string]$WhisperModelUrl = "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/media/branch/master/mirrors/raw/whisper-model.bin",
    [string]$WhisperModelPath = "",
    [switch]$PreferWinget = $false,
    [string]$GStreamerVersion = "1.26.10",
    [string]$GStreamerRuntimeUrl = "",
    [string]$GStreamerDevelUrl = "",
    [string]$GStreamerRuntimeMsi = "",
    [string]$GStreamerDevelMsi = ""
)

$ErrorActionPreference = "Stop"
$PreferWinget = $PSBoundParameters.ContainsKey("PreferWinget")

Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "  VideoTools Windows Installation" -ForegroundColor Cyan
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[WARN]   Running without Administrator privileges." -ForegroundColor Yellow
    Write-Host "         GStreamer install requires Administrator when missing." -ForegroundColor Yellow
    Write-Host ""
}

function Test-Command {
    param($Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Test-Pip {
    if (Test-Command pip) {
        return $true
    }
    if (Test-Command pip3) {
        return $true
    }
    return $false
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

function Get-MirrorHeaders {
    param(
        [string]$Accept = "application/octet-stream"
    )
    $headers = @{
        "Accept" = $Accept
    }
    $token = $env:VT_MIRROR_TOKEN
    $basic = $env:VT_MIRROR_BASIC
    if ($token) {
        $headers["Authorization"] = "token $token"
    } elseif ($basic) {
        $bytes = [Text.Encoding]::UTF8.GetBytes($basic)
        $headers["Authorization"] = "Basic " + [Convert]::ToBase64String($bytes)
    }
    return $headers
}

function Download-File {
    param(
        [string]$Url,
        [string]$Destination,
        [string]$UserAgent,
        [hashtable]$Headers = $null
    )
    if (Test-Path $Destination) {
        Remove-Item -Force $Destination
    }
    if (Test-Command curl.exe) {
        $curlArgs = @("-L", "--retry", "3", "--progress-bar", "--user-agent", $UserAgent, "-o", $Destination)
        if ($Headers) {
            foreach ($key in $Headers.Keys) {
                $curlArgs += @("-H", "${key}: $($Headers[$key])")
            }
        }
        $curlArgs += $Url
        & curl.exe @curlArgs | Out-Null
        return ($LASTEXITCODE -eq 0) -and (Test-Path $Destination)
    }

    $progressPreference = $ProgressPreference
    $ProgressPreference = "SilentlyContinue"
    try {
        Invoke-WebRequest -Uri $Url -OutFile $Destination -UseBasicParsing -UserAgent $UserAgent -Headers $Headers -MaximumRedirection 10
        return Test-Path $Destination
    } catch {
        return $false
    } finally {
        $ProgressPreference = $progressPreference
    }
}

function Find-GStreamerBin {
    $paths = @(
        "$env:ProgramFiles\GStreamer\1.0\msvc_x86_64\bin",
        "${env:ProgramFiles(x86)}\GStreamer\1.0\msvc_x86_64\bin",
        "C:\gstreamer\1.0\msvc_x86_64\bin",
        "C:\gstreamer\1.0\x86_64\bin"
    )
    foreach ($path in $paths) {
        if (-not $path) {
            continue
        }
        $gstExe = Join-Path $path "gst-launch-1.0.exe"
        if (Test-Path $gstExe) {
            return $path
        }
    }
    return $null
}

function Find-DVDStylerBin {
    $paths = @(
        "${env:ProgramFiles}\DVDStyler\bin",
        "${env:ProgramFiles(x86)}\DVDStyler\bin",
        "${env:ProgramFiles}\DVDStyler",
        "${env:ProgramFiles(x86)}\DVDStyler"
    )
    foreach ($path in $paths) {
        if (-not $path) {
            continue
        }
        $dvdauthor = Join-Path $path "dvdauthor.exe"
        $mkisofs = Join-Path $path "mkisofs.exe"
        if ((Test-Path $dvdauthor) -and (Test-Path $mkisofs)) {
            return $path
        }
    }
    return $null
}

function Add-ToUserPath {
    param(
        [string]$PathItem
    )
    if (-not $PathItem) {
        return
    }
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    if ($env:Path -match [Regex]::Escape($PathItem)) {
        return
    }
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notmatch [Regex]::Escape($PathItem)) {
        [Environment]::SetEnvironmentVariable("Path", "$PathItem;$userPath", "User")
    }
    $env:Path = "$PathItem;$env:Path"
}

function Find-ExeInRoots {
    param(
        [string]$ExeName,
        [string[]]$Roots
    )
    foreach ($root in $Roots) {
        if (-not $root -or -not (Test-Path $root)) {
            continue
        }
        try {
            $match = Get-ChildItem -Path $root -Recurse -Filter $ExeName -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($match) {
                return $match.DirectoryName
            }
        } catch {
            # Ignore search errors and continue.
        }
    }
    return $null
}

function Ensure-Msys2 {
    $msysRoot = "C:\msys64"
    $pacmanPath = Join-Path $msysRoot "usr\bin\pacman.exe"
    if (Test-Path $pacmanPath) {
        Write-Host "[OK]  MSYS2 already installed" -ForegroundColor Green
        return $true
    }

    if (Test-Command winget) {
        Write-Host "Installing MSYS2 via winget..." -ForegroundColor Yellow
        & winget install --id MSYS2.MSYS2 --silent --accept-package-agreements --accept-source-agreements
        if ($LASTEXITCODE -eq 0 -and (Test-Path $pacmanPath)) {
            Write-Host "[OK]  MSYS2 installed" -ForegroundColor Green
            return $true
        }
    }

    Write-Host "[ERROR]  MSYS2 not found and winget is unavailable." -ForegroundColor Red
    Write-Host "Install MSYS2 from https://www.msys2.org/ and re-run this installer." -ForegroundColor Yellow
    return $false
}

function Install-Msys2Packages {
    param(
        [string[]]$Packages
    )
    if (-not $Packages -or $Packages.Count -eq 0) {
        return $true
    }

    if (-not (Ensure-Msys2)) {
        return $false
    }

    $pacmanPath = "C:\msys64\usr\bin\pacman.exe"
    if (-not (Test-Path $pacmanPath)) {
        Write-Host "[ERROR]  pacman not found after MSYS2 install." -ForegroundColor Red
        return $false
    }

    Write-Host "Installing MSYS2 packages: $($Packages -join ', ')" -ForegroundColor Yellow
    & $pacmanPath -Sy --noconfirm --noprogressbar | Out-Null
    & $pacmanPath -S --needed --noconfirm @Packages | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[WARN]  MSYS2 package install failed. Open the MSYS2 shell and run:" -ForegroundColor Yellow
        Write-Host "        pacman -S --needed --noconfirm $($Packages -join ' ')" -ForegroundColor Yellow
        return $false
    }

    $mingwBin = "C:\msys64\mingw64\bin"
    if (Test-Path $mingwBin) {
        Add-ToUserPath -PathItem $mingwBin
    }
    return $true
}

function Install-GoViaWinget {
    if (Test-Command go) {
        return $true
    }
    if (-not (Test-Command winget)) {
        Write-Host "[WARN]  winget not available; install Go from https://go.dev/dl/." -ForegroundColor Yellow
        return $false
    }
    Write-Host "Installing Go via winget..." -ForegroundColor Yellow
    & winget install --id GoLang.Go --silent --accept-package-agreements --accept-source-agreements
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    return (Test-Command go)
}

function Install-PythonViaWinget {
    if (Test-Pip) {
        return $true
    }
    if (-not (Test-Command winget)) {
        Write-Host "[WARN]  winget not available; install Python from https://www.python.org/downloads/." -ForegroundColor Yellow
        return $false
    }
    Write-Host "Installing Python via winget..." -ForegroundColor Yellow
    & winget install --id Python.Python.3 --silent --accept-package-agreements --accept-source-agreements
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    return (Test-Pip)
}

function Install-FFmpegPortable {
    param(
        [string]$Url
    )

    $ffmpegRoot = Join-Path $env:LOCALAPPDATA "VideoTools\ffmpeg\bin"
    if (-not (Test-Path $ffmpegRoot)) {
        New-Item -ItemType Directory -Path $ffmpegRoot -Force | Out-Null
    }

    $ffmpegZip = Join-Path $env:TEMP "ffmpeg-windows.zip"
    $ffmpegExtract = Join-Path $env:TEMP ("ffmpeg-extract-" + [System.Guid]::NewGuid().ToString())

    Write-Host "Downloading FFmpeg..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $Url -OutFile $ffmpegZip -UseBasicParsing

    Write-Host "Extracting FFmpeg..." -ForegroundColor Yellow
    Expand-Archive -Path $ffmpegZip -DestinationPath $ffmpegExtract -Force

    $binDir = Get-ChildItem -Path $ffmpegExtract -Recurse -Directory -Filter "bin" | Select-Object -First 1
    if (-not $binDir) {
        throw "FFmpeg bin directory not found in downloaded archive"
    }

    $ffmpegExe = Join-Path $binDir.FullName "ffmpeg.exe"
    $ffprobeExe = Join-Path $binDir.FullName "ffprobe.exe"
    if (-not (Test-Path $ffmpegExe)) {
        throw "FFmpeg executable not found in downloaded archive"
    }

    Copy-Item $ffmpegExe -Destination $ffmpegRoot -Force
    Copy-Item $ffprobeExe -Destination $ffmpegRoot -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notmatch [Regex]::Escape($ffmpegRoot)) {
        [Environment]::SetEnvironmentVariable("Path", "$ffmpegRoot;$userPath", "User")
    }
    $env:Path = "$ffmpegRoot;$env:Path"

    Remove-Item -Force $ffmpegZip
    Remove-Item -Recurse -Force $ffmpegExtract

    Write-Host "[OK]  FFmpeg installed to $ffmpegRoot" -ForegroundColor Green
}

function Install-GStreamerMsi {
    param(
        [string]$RuntimeUrl,
        [string]$DevelUrl,
        [string]$RuntimeMsi,
        [string]$DevelMsi
    )

    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    if (-not $isAdmin) {
        Write-Host "[ERROR]  GStreamer requires Administrator privileges to install." -ForegroundColor Red
        Write-Host "Run PowerShell as Administrator and re-run this installer." -ForegroundColor Yellow
        exit 1
    }

    $tempDir = Join-Path $env:TEMP ("gstreamer-" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

    $runtimeMsiPath = Join-Path $tempDir "gstreamer-runtime.msi"
    $develMsiPath = Join-Path $tempDir "gstreamer-devel.msi"

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
    $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

    if (-not $RuntimeUrl) {
        $RuntimeUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/$GStreamerVersion/msvc/gstreamer-1.0-msvc-x86_64-$GStreamerVersion.msi"
    }
    if (-not $DevelUrl) {
        $DevelUrl = "https://gstreamer.freedesktop.org/data/pkg/windows/$GStreamerVersion/msvc/gstreamer-1.0-devel-msvc-x86_64-$GStreamerVersion.msi"
    }

    $defaultRuntimeUrls = @($RuntimeUrl)
    $defaultDevelUrls = @($DevelUrl)

    function Get-UrlCandidates {
        param(
            [string]$PrimaryUrl,
            [string[]]$Fallbacks
        )
        $urls = New-Object System.Collections.Generic.List[string]
        if ($PrimaryUrl) {
            $urls.Add($PrimaryUrl)
        }
        foreach ($fallback in $Fallbacks) {
            if (-not $urls.Contains($fallback)) {
                $urls.Add($fallback)
            }
        }
        return $urls
    }

    function Invoke-DownloadFile {
        param(
            [string[]]$Urls,
            [string]$Destination
        )

        $lastUrl = ""
        foreach ($url in $Urls) {
            $lastUrl = $url
            $downloadOk = $false
            if (Test-Path $Destination) {
                Remove-Item -Force $Destination
            }
            try {
                Invoke-WebRequest -Uri $url -OutFile $Destination -UseBasicParsing -UserAgent $userAgent -Headers @{
                    "Accept" = "application/octet-stream"
                } -MaximumRedirection 10
                $downloadOk = $true
            } catch {
                $downloadOk = $false
            }

            if (-not $downloadOk) {
                try {
                    Start-BitsTransfer -Source $url -Destination $Destination -ErrorAction Stop
                    $downloadOk = $true
                } catch {
                    $downloadOk = $false
                }
            }

            if (-not $downloadOk -and (Test-Command curl.exe)) {
                & curl.exe -L --retry 3 --user-agent $userAgent -o $Destination $url | Out-Null
                if ($LASTEXITCODE -eq 0) {
                    $downloadOk = $true
                }
            }

            if (-not $downloadOk) {
                continue
            }

            if (-not (Test-Path $Destination)) {
                continue
            }

            $fileSize = (Get-Item $Destination).Length
            if ($fileSize -lt 1048576) {
                continue
            }

            return $true
        }

        if ($lastUrl) {
            Write-Host "[ERROR]  Failed to download GStreamer MSI from: $lastUrl" -ForegroundColor Red
        }
        return $false
    }

    function Install-GStreamerViaWinget {
        param(
            [string[]]$RuntimeIds,
            [string[]]$DevelIds
        )

        if (-not $PreferWinget) {
            return $false
        }
        if (-not (Test-Command winget)) {
            return $false
        }

        Write-Host "Attempting GStreamer install via winget..." -ForegroundColor Yellow
        $wingetArgs = @("--silent", "--accept-package-agreements", "--accept-source-agreements")

        $runtimeOk = $false
        foreach ($id in $RuntimeIds) {
            & winget install --id $id @wingetArgs
            if ($LASTEXITCODE -eq 0) {
                $runtimeOk = $true
                break
            }
        }

        if (-not $runtimeOk) {
            return $false
        }

        $develOk = $true
        if ($DevelIds.Count -gt 0) {
            $develOk = $false
            foreach ($id in $DevelIds) {
                & winget install --id $id @wingetArgs
                if ($LASTEXITCODE -eq 0) {
                    $develOk = $true
                    break
                }
            }
        }

        if (-not $develOk) {
            return $false
        }

        if (Test-Command gst-launch-1.0) {
            return $true
        }

        $binPath = Find-GStreamerBin
        if ($binPath) {
            Add-ToUserPath -PathItem $binPath
            if (Test-Command gst-launch-1.0) {
                return $true
            }
        }

        Write-Host "[WARN]  GStreamer winget install did not expose gst-launch-1.0. Falling back to MSI." -ForegroundColor Yellow
        return $false
    }

    $existingGstBin = Find-GStreamerBin
    if ($existingGstBin) {
        Add-ToUserPath -PathItem $existingGstBin
        if (Test-Command gst-launch-1.0) {
            return
        }
    }

    $gstBinFromDisk = Find-ExeInRoots -ExeName "gst-launch-1.0.exe" -Roots @(
        "$env:ProgramFiles",
        "$env:ProgramFiles(x86)",
        "C:\gstreamer"
    )
    if ($gstBinFromDisk) {
        Add-ToUserPath -PathItem $gstBinFromDisk
        if (Test-Command gst-launch-1.0) {
            return
        }
    }

    if ($PreferWinget -and -not $RuntimeMsi -and -not $DevelMsi) {
        $wingetRuntimeIds = @("GStreamer.GStreamer")
        $wingetDevelIds = @("GStreamer.GStreamer.Devel", "GStreamer.GStreamer.Dev")
        if (Install-GStreamerViaWinget -RuntimeIds $wingetRuntimeIds -DevelIds $wingetDevelIds) {
            return
        }
    }

    if ($RuntimeMsi) {
        if (-not (Test-Path $RuntimeMsi)) {
            throw "GStreamer runtime MSI not found: $RuntimeMsi"
        }
        Copy-Item -Path $RuntimeMsi -Destination $runtimeMsiPath -Force
    } else {
        Write-Host "Downloading GStreamer runtime..." -ForegroundColor Yellow
        $runtimeUrls = Get-UrlCandidates -PrimaryUrl $RuntimeUrl -Fallbacks $defaultRuntimeUrls
        $runtimeOk = Invoke-DownloadFile -Urls $runtimeUrls -Destination $runtimeMsiPath
        if (-not $runtimeOk) {
            Write-Host "[ERROR]  Failed to download GStreamer runtime MSI." -ForegroundColor Red
            Write-Host "Manual download: https://gstreamer.freedesktop.org/data/pkg/windows/$GStreamerVersion/msvc/" -ForegroundColor Yellow
            Write-Host "Then re-run with -GStreamerRuntimeMsi and -GStreamerDevelMsi." -ForegroundColor Yellow
            if (-not $PreferWinget) {
                $wingetRuntimeIds = @("GStreamer.GStreamer")
                $wingetDevelIds = @("GStreamer.GStreamer.Devel", "GStreamer.GStreamer.Dev")
                if (Install-GStreamerViaWinget -RuntimeIds $wingetRuntimeIds -DevelIds $wingetDevelIds) {
                    return
                }
            }
            throw "Failed to download GStreamer runtime MSI."
        }
    }

    if ($DevelMsi) {
        if (-not (Test-Path $DevelMsi)) {
            throw "GStreamer development MSI not found: $DevelMsi"
        }
        Copy-Item -Path $DevelMsi -Destination $develMsiPath -Force
    } else {
        Write-Host "Downloading GStreamer development files..." -ForegroundColor Yellow
        $develUrls = Get-UrlCandidates -PrimaryUrl $DevelUrl -Fallbacks $defaultDevelUrls
        $develOk = Invoke-DownloadFile -Urls $develUrls -Destination $develMsiPath
        if (-not $develOk) {
            Write-Host "[ERROR]  Failed to download GStreamer development MSI." -ForegroundColor Red
            Write-Host "Manual download: https://gstreamer.freedesktop.org/data/pkg/windows/$GStreamerVersion/msvc/" -ForegroundColor Yellow
            Write-Host "Then re-run with -GStreamerRuntimeMsi and -GStreamerDevelMsi." -ForegroundColor Yellow
            if (-not $PreferWinget) {
                $wingetRuntimeIds = @("GStreamer.GStreamer")
                $wingetDevelIds = @("GStreamer.GStreamer.Devel", "GStreamer.GStreamer.Dev")
                if (Install-GStreamerViaWinget -RuntimeIds $wingetRuntimeIds -DevelIds $wingetDevelIds) {
                    return
                }
            }
            throw "Failed to download GStreamer development MSI."
        }
    }

    if ((Get-Item $runtimeMsiPath).Length -lt 1048576) {
        throw "GStreamer runtime MSI download is too small. Provide a local MSI with -GStreamerRuntimeMsi."
    }
    if ((Get-Item $develMsiPath).Length -lt 1048576) {
        throw "GStreamer development MSI download is too small. Provide a local MSI with -GStreamerDevelMsi."
    }

    Write-Host "Installing GStreamer runtime..." -ForegroundColor Yellow
    $runtime = Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$runtimeMsiPath`" /qn /norestart" -Wait -PassThru
    if ($runtime.ExitCode -ne 0) {
        throw "GStreamer runtime install failed with exit code $($runtime.ExitCode)"
    }

    Write-Host "Installing GStreamer development files..." -ForegroundColor Yellow
    $devel = Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$develMsiPath`" /qn /norestart" -Wait -PassThru -Timeout 300
    if ($devel.ExitCode -ne 0) {
        throw "GStreamer dev install failed with exit code $($devel.ExitCode)"
    }

    $gstBin = Find-GStreamerBin
    if ($gstBin) {
        Add-ToUserPath -PathItem $gstBin
    }

    Remove-Item -Recurse -Force $tempDir
}

function Ensure-DVDStylerTools {
    if ($SkipDvdStyler) {
        Write-Host "[SKIP] DVD authoring tools skipped (DVDStyler)" -ForegroundColor Yellow
        return
    }
    $toolsRoot = Join-Path $PSScriptRoot "tools"
    $dvdstylerDir = Join-Path $toolsRoot "dvdstyler"
    $dvdstylerBin = Join-Path $dvdstylerDir "bin"
    $dvdstylerReferer = "https://sourceforge.net/projects/dvdstyler/"
    $dvdstylerVersion = "3.2.2"
    $dvdstylerZipName = "DVDStyler-$dvdstylerVersion-win64.zip"
    $dvdstylerExeUrl = ""
    $mirrorUrls = @(
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/media/branch/master/mirrors/raw/DVDStyler-3.2.1-win64.exe",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/media/branch/master/mirrors/raw/DVDStyler-3.2.1-win64.exe?download=1",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/DVDStyler-3.2.1.-win64.exe",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/DVDStyler-3.2.1-win64.exe",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/DVDStyler-3.2.1.-win64.exe?download=1",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/DVDStyler-3.2.1-win64.exe?download=1"
    )
    $sourceForgeUrls = @(
        "https://downloads.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://netcologne.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://cfhcable.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://pilotfiber.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://versaweb.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://liquidtelecom.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://master.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://ufpr.dl.sourceforge.net/project/dvdstyler/DVDStyler/$dvdstylerVersion/$dvdstylerZipName",
        "https://sourceforge.net/projects/dvdstyler/files/DVDStyler/$dvdstylerVersion/$dvdstylerZipName/download"
    )
    $dvdstylerUrls = @()
    function Install-DVDStylerViaWinget {
        if (-not $PreferWinget) {
            return $false
        }
        if (-not (Test-Command winget)) {
            return $false
        }
        Write-Host "Attempting DVDStyler install via winget..." -ForegroundColor Yellow
        $wingetArgs = @("--silent", "--accept-package-agreements", "--accept-source-agreements")
        $wingetIds = @("DVDStyler.DVDStyler")
        foreach ($id in $wingetIds) {
            & winget install --id $id @wingetArgs
            if ($LASTEXITCODE -eq 0) {
                $binPath = Find-DVDStylerBin
                if ($binPath) {
                    Add-ToUserPath -PathItem $binPath
                    if (Test-Command dvdauthor -and Test-Command mkisofs) {
                        Write-Host "[OK]  DVD authoring tools installed via winget" -ForegroundColor Green
                        return $true
                    }
                }
                Write-Host "[WARN]  DVDStyler winget install missing dvdauthor/mkisofs. Falling back to portable ZIP." -ForegroundColor Yellow
                return $false
            }
        }
        return $false
    }

    $dvdstylerZip = Join-Path $env:TEMP "dvdstyler-win64.zip"
    $dvdstylerExe = Join-Path $env:TEMP "dvdstyler-win64.exe"
    $needsDVDTools = (-not (Test-Command dvdauthor)) -or (-not (Test-Command mkisofs))
    if ($needsDVDTools) {
        $existingBin = Find-DVDStylerBin
        if ($existingBin) {
            Add-ToUserPath -PathItem $existingBin
            $needsDVDTools = (-not (Test-Command dvdauthor)) -or (-not (Test-Command mkisofs))
        }
    }
    if ($needsDVDTools) {
        $dvdBinFromDisk = Find-ExeInRoots -ExeName "dvdauthor.exe" -Roots @(
            "$env:ProgramFiles",
            "$env:ProgramFiles(x86)"
        )
        if ($dvdBinFromDisk) {
            Add-ToUserPath -PathItem $dvdBinFromDisk
            $needsDVDTools = (-not (Test-Command dvdauthor)) -or (-not (Test-Command mkisofs))
        }
    }

    if (-not $needsDVDTools) {
        return
    }

    Write-Host ""
    Write-Host "Optional module: DVD authoring tools (DVDStyler portable)" -ForegroundColor Yellow
    $dvdChoice = Read-Host "Install DVD authoring tools? (y/N)"
    if ($dvdChoice -ne "y" -and $dvdChoice -ne "Y") {
        $SkipDvdStyler = $true
        Write-Host "[SKIP] DVD authoring tools skipped" -ForegroundColor Yellow
        return
    }

    Write-Host "Installing DVD authoring tools (DVDStyler portable)..." -ForegroundColor Yellow
    if (-not (Test-Path $toolsRoot)) {
        New-Item -ItemType Directory -Force -Path $toolsRoot | Out-Null
    }
    if (Test-Path $dvdstylerDir) {
        Remove-Item -Recurse -Force $dvdstylerDir
    }

    $dvdZipProvided = $PSBoundParameters.ContainsKey("DvdStylerZip") -and $DvdStylerZip
    $dvdUrlProvided = $PSBoundParameters.ContainsKey("DvdStylerUrl") -and $DvdStylerUrl
    $dvdExeProvided = $PSBoundParameters.ContainsKey("DvdStylerExeUrl") -and $DvdStylerExeUrl
    if ($dvdExeProvided) {
        $dvdstylerExeUrl = $DvdStylerExeUrl
    }
    if ($dvdUrlProvided) {
        $env:VT_DVDSTYLER_URL = $DvdStylerUrl
        $dvdstylerUrls += @($DvdStylerUrl)
    }
    if ($dvdExeProvided) {
        $dvdstylerUrls += @($DvdStylerExeUrl)
    }
    if ($dvdstylerUrls.Count -eq 0) {
        $dvdstylerUrls += $mirrorUrls
    }
    if ($env:VT_DVDSTYLER_ALLOW_SOURCEFORGE -eq "1") {
        $dvdstylerUrls += $sourceForgeUrls
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
    $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    $downloaded = $false
    $downloadedType = ""
    $downloadedPath = ""
    $lastUrl = ""
    if ($dvdZipProvided) {
        if (Test-Path $DvdStylerZip) {
            Copy-Item -Path $DvdStylerZip -Destination $dvdstylerZip -Force
            $lastUrl = $DvdStylerZip
            try {
                $fs = [System.IO.File]::OpenRead($dvdstylerZip)
                try {
                    $fileSize = (Get-Item $dvdstylerZip).Length
                    if ($fileSize -ge 102400) {
                        $sig = New-Object byte[] 2
                        $null = $fs.Read($sig, 0, 2)
                        if ($sig[0] -eq 0x50 -and $sig[1] -eq 0x4B) {
                            $downloaded = $true
                            $downloadedType = "zip"
                            $downloadedPath = $dvdstylerZip
                        } elseif ($sig[0] -eq 0x4D -and $sig[1] -eq 0x5A) {
                            $downloaded = $true
                            $downloadedType = "exe"
                            $downloadedPath = $dvdstylerZip
                        }
                    }
                } finally {
                    $fs.Close()
                }
            } catch {
                # Fall through to error handling below.
            }
            if (-not $downloaded) {
                Write-Host "[ERROR]  Provided DVDStyler archive is not a valid ZIP or EXE: $DvdStylerZip" -ForegroundColor Red
                exit 1
            }
        } else {
            Write-Host "[ERROR]  Provided DVDStyler ZIP not found: $DvdStylerZip" -ForegroundColor Red
            exit 1
        }
    } else {
        foreach ($url in $dvdstylerUrls) {
            $lastUrl = $url
            $downloadOk = $false
            $downloadTarget = $dvdstylerZip
            $acceptHeader = "application/zip"
            if ($url.ToLower().EndsWith(".exe")) {
                $downloadTarget = $dvdstylerExe
                $acceptHeader = "application/octet-stream"
            }
            if (Test-Path $downloadTarget) {
                Remove-Item -Force $downloadTarget
            }
            $headers = Get-MirrorHeaders -Accept $acceptHeader
            $headers["Referer"] = $dvdstylerReferer
            $downloadOk = Download-File -Url $url -Destination $downloadTarget -UserAgent $userAgent -Headers $headers

            if (-not $downloadOk -or -not (Test-Path $downloadTarget)) {
                continue
            }

            try {
                $fs = [System.IO.File]::OpenRead($downloadTarget)
                try {
                    $fileSize = (Get-Item $downloadTarget).Length
                    if ($fileSize -lt 102400) {
                        Write-Host "[WARN]  DVDStyler mirror download is too small. If the mirror is private, set VT_MIRROR_TOKEN or VT_MIRROR_BASIC." -ForegroundColor Yellow
                        continue
                    }
                    $sig = New-Object byte[] 2
                    $null = $fs.Read($sig, 0, 2)
                    if ($sig[0] -eq 0x50 -and $sig[1] -eq 0x4B) {
                        $downloaded = $true
                        $downloadedType = "zip"
                        $downloadedPath = $downloadTarget
                        break
                    }
                    if ($sig[0] -eq 0x4D -and $sig[1] -eq 0x5A) {
                        $downloaded = $true
                        $downloadedType = "exe"
                        $downloadedPath = $downloadTarget
                        break
                    }
                } finally {
                    $fs.Close()
                }
            } catch {
                # Try next URL
            }
        }
    }
    if (-not $downloaded) {
        if (Install-DVDStylerViaWinget) {
            return
        }
        Write-Host "[WARN]  Failed to download DVDStyler archive (invalid ZIP/EXE)" -ForegroundColor Yellow
        Write-Host "Last URL tried: $lastUrl" -ForegroundColor Yellow
        Write-Host "Tip: Set VT_DVDSTYLER_URL to a direct ZIP or EXE link and retry." -ForegroundColor Yellow
        Write-Host "Manual download page: https://sourceforge.net/projects/dvdstyler/files/DVDStyler/$dvdstylerVersion/" -ForegroundColor Yellow
        Write-Host "After download, extract and ensure bin\\dvdauthor.exe and bin\\mkisofs.exe are on PATH." -ForegroundColor Yellow
        Write-Host "[SKIP] DVD authoring tools skipped due to download failure" -ForegroundColor Yellow
        return
    }

    if ($downloadedType -eq "exe") {
        Write-Host "Installing DVDStyler from installer..." -ForegroundColor Yellow
        try {
            $proc = Start-Process -FilePath $downloadedPath -ArgumentList $DvdStylerExeArgs -Wait -PassThru
            if ($proc.ExitCode -ne 0) {
                throw "DVDStyler installer returned exit code $($proc.ExitCode)"
            }
        } catch {
            Write-Host "[WARN]  DVDStyler installer failed: $($_.Exception.Message)" -ForegroundColor Yellow
            if (Install-DVDStylerViaWinget) {
                return
            }
            Write-Host "[SKIP] DVD authoring tools skipped due to installer failure" -ForegroundColor Yellow
            return
        } finally {
            if (Test-Path $downloadedPath) {
                Remove-Item -Force $downloadedPath
            }
        }
        $binPath = Find-DVDStylerBin
        if ($binPath) {
            Add-ToUserPath -PathItem $binPath
        }
        if (Test-Command dvdauthor -and Test-Command mkisofs) {
            Write-Host "[OK]  DVD authoring tools installed via DVDStyler installer" -ForegroundColor Green
            return
        }
        Write-Host "[WARN]  DVDStyler installer did not expose dvdauthor/mkisofs on PATH." -ForegroundColor Yellow
        Write-Host "[SKIP] DVD authoring tools skipped after installer" -ForegroundColor Yellow
        return
    }

    $extractRoot = Join-Path $env:TEMP ("dvdstyler-extract-" + [System.Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force -Path $extractRoot | Out-Null
    Expand-Archive -Path $dvdstylerZip -DestinationPath $extractRoot -Force

    $entries = Get-ChildItem -Path $extractRoot
    if ($entries.Count -eq 1 -and $entries[0].PSIsContainer) {
        Copy-Item -Path (Join-Path $entries[0].FullName "*") -Destination $dvdstylerDir -Recurse -Force
    } else {
        Copy-Item -Path (Join-Path $extractRoot "*") -Destination $dvdstylerDir -Recurse -Force
    }

    Remove-Item -Force $dvdstylerZip
    Remove-Item -Recurse -Force $extractRoot

    if (Test-Path $dvdstylerBin) {
        $env:Path = "$dvdstylerBin;$env:Path"
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notmatch [Regex]::Escape($dvdstylerBin)) {
            [Environment]::SetEnvironmentVariable("Path", "$dvdstylerBin;$userPath", "User")
        }
        Write-Host "[OK]  DVD authoring tools installed to $dvdstylerDir" -ForegroundColor Green
    } else {
        Write-Host "[ERROR]  DVDStyler tools missing after install" -ForegroundColor Red
        exit 1
    }
}

function Ensure-WhisperModel {
    if ($SkipWhisper) {
        Write-Host "[SKIP] Whisper model skipped" -ForegroundColor Yellow
        return
    }

    if (-not $WhisperModelPath) {
        $WhisperModelPath = Join-Path $env:LOCALAPPDATA "VideoTools\whisper\whisper-model.bin"
    }

    if (Test-Path $WhisperModelPath) {
        return
    }

    if (-not $InstallWhisper) {
        Write-Host ""
        Write-Host "Optional module: Subtitle transcription (Whisper small model)" -ForegroundColor Yellow
        $whisperChoice = Read-Host "Install Whisper model? (y/N)"
        if ($whisperChoice -eq "y" -or $whisperChoice -eq "Y") {
            $InstallWhisper = $true
        } else {
            $SkipWhisper = $true
        }
        Write-Host ""
    }

    if ($SkipWhisper -or -not $InstallWhisper) {
        Write-Host "[SKIP] Whisper model skipped" -ForegroundColor Yellow
        return
    }

    $modelDir = Split-Path -Parent $WhisperModelPath
    if (-not (Test-Path $modelDir)) {
        New-Item -ItemType Directory -Force -Path $modelDir | Out-Null
    }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor 3072
    $userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    $whisperUrls = @(
        $WhisperModelUrl,
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/media/branch/master/mirrors/raw/whisper-model.bin?download=1",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/whisper-model.bin",
        "https://git.leaktechnologies.dev/lt_mirror/lt_mirror/src/branch/master/mirrors/raw/whisper-model.bin?download=1"
    )

    Write-Host "Downloading Whisper model..." -ForegroundColor Yellow
    $downloadOk = $false
    $lastWhisperUrl = ""
    foreach ($url in $whisperUrls) {
        $lastWhisperUrl = $url
        if (Test-Path $WhisperModelPath) {
            Remove-Item -Force $WhisperModelPath
        }
        $headers = Get-MirrorHeaders -Accept "application/octet-stream"
        $downloadOk = Download-File -Url $url -Destination $WhisperModelPath -UserAgent $userAgent -Headers $headers

        if (-not $downloadOk -or -not (Test-Path $WhisperModelPath)) {
            continue
        }

        $fileSize = (Get-Item $WhisperModelPath).Length
        if ($fileSize -ge 1048576) {
            break
        }
        $downloadOk = $false
    }

    if (-not $downloadOk -or -not (Test-Path $WhisperModelPath)) {
        Write-Host "[WARN]  Failed to download Whisper model." -ForegroundColor Yellow
        Write-Host "Last URL tried: $lastWhisperUrl" -ForegroundColor Yellow
        Write-Host "[SKIP] Whisper model skipped due to download failure" -ForegroundColor Yellow
        return
    }

    $fileSize = (Get-Item $WhisperModelPath).Length
    if ($fileSize -lt 1048576) {
        Write-Host "[WARN]  Whisper model download is too small. If the mirror is private, set VT_MIRROR_TOKEN or VT_MIRROR_BASIC." -ForegroundColor Yellow
        Write-Host "Last URL tried: $lastWhisperUrl" -ForegroundColor Yellow
        Write-Host "[SKIP] Whisper model skipped due to download failure" -ForegroundColor Yellow
        Remove-Item -Force $WhisperModelPath
        return
    }

    Write-Host "[OK]  Whisper model downloaded to $WhisperModelPath" -ForegroundColor Green
}

Write-Host "Checking system..." -ForegroundColor Yellow
Write-Host ""

$osVersion = [System.Environment]::OSVersion.Version
Write-Host "Windows Version: $($osVersion.Major).$($osVersion.Minor) (Build $($osVersion.Build))" -ForegroundColor Cyan

if ($osVersion.Major -lt 10) {
    Write-Host "[WARN]   Windows 10 or later is recommended" -ForegroundColor Yellow
}
Write-Host ""

if (-not (Test-Pip)) {
    if (-not $SkipPython -and -not $InstallPython) {
        Write-Host "Optional module: Python + pip (AI tooling and optional modules)" -ForegroundColor Yellow
        $pyChoice = Read-Host "Install Python + pip? (y/N)"
        if ($pyChoice -eq "y" -or $pyChoice -eq "Y") {
            $InstallPython = $true
        } else {
            $SkipPython = $true
        }
        Write-Host ""
    }
}

if (-not (Test-Command gst-launch-1.0) -and -not $SkipGStreamer -and -not $isAdmin) {
    Write-Host "[ERROR]  GStreamer requires Administrator privileges to install." -ForegroundColor Red
    Write-Host "Run PowerShell as Administrator and re-run this installer." -ForegroundColor Yellow
    exit 1
}

if ($InstallBuildTools -eq $false -and $SkipBuildTools -eq $false) {
    $needsBuildTools = (-not (Test-Command go)) -or (-not (Test-Command gcc))
    if ($needsBuildTools) {
        Write-Host "Build tools missing; installing Go + MSYS2 MinGW-w64 automatically." -ForegroundColor Yellow
        $InstallBuildTools = $true
    }
}

if ($InstallBuildTools) {
    $goOk = Install-GoViaWinget
    if (-not $goOk) {
        Write-Host "[WARN]  Go install not completed; build tools may be incomplete." -ForegroundColor Yellow
    }

    if (-not (Test-Command gcc)) {
        $gccOk = Install-Msys2Packages -Packages @("mingw-w64-x86_64-gcc")
        if (-not $gccOk) {
            Write-Host "[WARN]  MSYS2 GCC install did not complete; GCC may be unavailable." -ForegroundColor Yellow
        }
    } else {
        $mingwBin = "C:\msys64\mingw64\bin"
        if (Test-Path $mingwBin) {
            Add-ToUserPath -PathItem $mingwBin
        }
    }
}

if ($InstallPython) {
    $pythonOk = Install-PythonViaWinget
    if (-not $pythonOk) {
        Write-Host "[WARN]  Python install not completed; pip may be unavailable." -ForegroundColor Yellow
    }
}

if ($InstallBuildTools -and (Test-Command gcc)) {
    if (-not (Test-Gcc)) {
        Write-Host "[WARN]  GCC test compile failed. The MSYS2 toolchain may be incomplete." -ForegroundColor Yellow
        $repairChoice = Read-Host "Reinstall MSYS2 GCC package now? (y/N)"
        if ($repairChoice -eq "y" -or $repairChoice -eq "Y") {
            Write-Host "Reinstalling MSYS2 GCC..." -ForegroundColor Yellow
            Install-Msys2Packages -Packages @("mingw-w64-x86_64-gcc") | Out-Null
            if (Test-Gcc) {
                Write-Host "[OK]  GCC toolchain repaired" -ForegroundColor Green
            } else {
                Write-Host "[WARN]  GCC still failing after reinstall" -ForegroundColor Yellow
            }
        }
    }
}

if (-not $SkipFFmpeg -and -not (Test-Command ffmpeg)) {
    $ffmpegUrl = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
    Install-FFmpegPortable -Url $ffmpegUrl
}

if (-not $SkipGStreamer -and -not (Test-Command gst-launch-1.0)) {
    Write-Host "GStreamer is required for VideoTools playback." -ForegroundColor Yellow
    Install-GStreamerMsi -RuntimeUrl $GStreamerRuntimeUrl -DevelUrl $GStreamerDevelUrl -RuntimeMsi $GStreamerRuntimeMsi -DevelMsi $GStreamerDevelMsi
}

Ensure-DVDStylerTools
Ensure-WhisperModel

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host "[OK]  DEPENDENCIES INSTALLED" -ForegroundColor Green
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host ""

# Refresh environment variables
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")

Write-Host "Verifying installations..." -ForegroundColor Yellow
Write-Host ""

if (Test-Command go) {
    $goVersion = go version
    Write-Host "[OK]  Go: $goVersion" -ForegroundColor Green
} else {
    Write-Host "[WARN]   Go not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command gcc) {
    $gccVersion = gcc --version | Select-Object -First 1
    Write-Host "[OK]  GCC: $gccVersion" -ForegroundColor Green
} else {
    Write-Host "[WARN]   GCC not found in PATH (restart terminal)" -ForegroundColor Yellow
}

if (Test-Command ffmpeg) {
    $ffmpegVersion = ffmpeg -version | Select-Object -First 1
    Write-Host "[OK]  ffmpeg: $ffmpegVersion" -ForegroundColor Green
} else {
    if ($SkipFFmpeg) {
        Write-Host "[INFO]   ffmpeg skipped (use -SkipFFmpeg:`$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   ffmpeg not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Command gst-launch-1.0) {
    $gstVersion = gst-launch-1.0 --version | Select-Object -First 1
    Write-Host "[OK]  gstreamer: $gstVersion" -ForegroundColor Green
} else {
    if (-not $SkipGStreamer) {
        $gstBin = Find-GStreamerBin
        if ($gstBin) {
            Add-ToUserPath -PathItem $gstBin
        }
    }
    if (Test-Command gst-launch-1.0) {
        $gstVersion = gst-launch-1.0 --version | Select-Object -First 1
        Write-Host "[OK]  gstreamer: $gstVersion" -ForegroundColor Green
    } elseif ($SkipGStreamer) {
        Write-Host "[INFO]   gstreamer skipped (use -SkipGStreamer:`$false to install)" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   gstreamer not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Pip) {
    if (Test-Command pip) {
        $pipVersion = pip --version
        Write-Host "[OK]  pip: $pipVersion" -ForegroundColor Green
    } elseif (Test-Command pip3) {
        $pipVersion = pip3 --version
        Write-Host "[OK]  pip: $pipVersion" -ForegroundColor Green
    }
} else {
    if ($SkipPython) {
        Write-Host "[INFO]   Python + pip skipped" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   pip not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
}

if (Test-Command dvdauthor) {
    Write-Host "[OK]  dvdauthor: found" -ForegroundColor Green
} else {
    if ($SkipDvdStyler) {
        Write-Host "[INFO]   dvdauthor skipped (DVD authoring not installed)" -ForegroundColor Cyan
    } else {
    $dvdBin = Find-DVDStylerBin
    if ($dvdBin) {
        Add-ToUserPath -PathItem $dvdBin
    }
    if (Test-Command dvdauthor) {
        Write-Host "[OK]  dvdauthor: found" -ForegroundColor Green
    } else {
        Write-Host "[WARN]   dvdauthor not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
    }
}

if (Test-Command mkisofs) {
    Write-Host "[OK]  mkisofs: found" -ForegroundColor Green
} else {
    if ($SkipDvdStyler) {
        Write-Host "[INFO]   mkisofs skipped (DVD authoring not installed)" -ForegroundColor Cyan
    } else {
    $dvdBin = Find-DVDStylerBin
    if ($dvdBin) {
        Add-ToUserPath -PathItem $dvdBin
    }
    if (Test-Command mkisofs) {
        Write-Host "[OK]  mkisofs: found" -ForegroundColor Green
    } else {
        Write-Host "[WARN]   mkisofs not found in PATH (restart terminal)" -ForegroundColor Yellow
    }
    }
}

if (-not $WhisperModelPath) {
    $WhisperModelPath = Join-Path $env:LOCALAPPDATA "VideoTools\whisper\whisper-model.bin"
}
if (Test-Path $WhisperModelPath) {
    Write-Host "[OK]  whisper model: found" -ForegroundColor Green
} else {
    if ($SkipWhisper) {
        Write-Host "[INFO]   whisper model skipped" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN]   whisper model not found (rerun installer to download)" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "===============================================================" -ForegroundColor Cyan
Write-Host " Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Restart your terminal/PowerShell" -ForegroundColor White
Write-Host "  2. Build: .\\scripts\\build.ps1" -ForegroundColor White
Write-Host ""









