# Create-Icon.ps1
# Generates a proper Windows ICO from PNG using FFmpeg
# Requires: FFmpeg in PATH

param(
    [string]$InputPng = "assets/logo/VT_logo.png",
    [string]$OutputIco = "assets/logo/VT_logo.ico",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

# Check FFmpeg
$ffmpeg = Get-Command ffmpeg -ErrorAction SilentlyContinue
if (-not $ffmpeg) {
    Write-Error "FFmpeg not found. Please install FFmpeg and ensure it's in PATH."
    exit 1
}

Write-Host "Using FFmpeg: $($ffmpeg.Source)"

# Check input file
if (-not (Test-Path $InputPng)) {
    Write-Error "Input PNG not found: $InputPng"
    exit 1
}

if ((Test-Path $OutputIco) -and -not $Force) {
    Write-Host "Output file exists. Use -Force to overwrite."
    exit 0
}

$tempDir = Join-Path $env:TEMP "vt_icon_$([guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
    # Create PNG versions at different sizes
    # ICO format: 256, 128, 64, 48, 32, 16
    $sizes = @(256, 48, 32, 16)
    
    foreach ($size in $sizes) {
        $output = Join-Path $tempDir "icon_${size}.png"
        Write-Host "Creating ${size}x${size}..."
        
        ffmpeg -y -i $InputPng `
            -vf "scale=${size}:${size}:force_original_aspect_ratio=decrease,scale=ceil(min(iw\,ih)/${size})*${size}:ceil(min(iw\,ih)/${size})*${size}:force_divisible_by=2,scale=${size}:${size}" `
            -frames:v 1 `
            $output 2>&1 | Out-Null
            
        if ($LASTEXITCODE -ne 0) {
            throw "FFmpeg failed for size ${size}"
        }
    }
    
    # For PNG-based ICO, we need to combine them
    # Modern ICO supports PNG directly (Vista+)
    
    # Create the ICO file manually with PNG images
    $pngFiles = @()
    foreach ($size in $sizes) {
        $pngFiles += Join-Path $tempDir "icon_${size}.png"
    }
    
    Write-Host "Generating ICO with PNG format..."
    
    # Use FFmpeg to create a multi-size ICO
    # FFmpeg can output to ICO format with multiple sizes
    ffmpeg -y `
        -i $InputPng `
        -vf "split[a][b],[a]scale=256:256[a1],[b]scale=48:48[b1],[a1][b1]vstack[a],[a]scale=32:32[v]" `
        -frames:v 1 `
        "$OutputIco" 2>&1 | Out-Null
    
    if ($LASTEXITCODE -eq 0 -and (Test-Path $OutputIco)) {
        $size = (Get-Item $OutputIco).Length
        Write-Host "Created: $OutputIco ($size bytes)"
    } else {
        Write-Host "FFmpeg ICO output failed. Trying alternative method..."
        
        # Alternative: Create PNG-based ICO manually
        $icoPath = $OutputIco
        
        # Read all PNG files
        $pngData = @()
        foreach ($size in $sizes) {
            $png = Join-Path $tempDir "icon_${size}.png"
            if (Test-Path $png) {
                $data = [System.IO.File]::ReadAllBytes($png)
                $pngData += @{
                    Size = $size
                    Data = $data
                }
            }
        }
        
        # Build ICO file
        $ms = New-Object System.IO.MemoryStream
        
        # ICO Header (6 bytes)
        $ms.Write([byte[]]@(0, 0), 0, 2)  # Reserved
        $ms.Write([byte[]]@(1, 0), 0, 2)  # Type: 1 = ICO
        $count = [BitConverter]::GetBytes([int16]$pngData.Count)
        $ms.Write($count, 0, 2)  # Number of images
        
        # Calculate data offset (6 + 16 bytes per image)
        $dataOffset = 6 + (16 * $pngData.Count)
        $imageData = @()
        
        # Write directory entries
        foreach ($img in $pngData) {
            $w = if ($img.Size -ge 256) { 0 } else { $img.Size }
            $h = if ($img.Size -ge 256) { 0 } else { $img.Size }
            
            # Width (1 byte)
            $ms.WriteByte([byte]$w)
            # Height (1 byte)
            $ms.WriteByte([byte]$h)
            # Color palette (1 byte, 0 = no palette)
            $ms.WriteByte(0)
            # Reserved (1 byte)
            $ms.WriteByte(0)
            # Color planes (2 bytes)
            $ms.Write([BitConverter]::GetBytes([int16]1), 0, 2)
            # Bits per pixel (2 bytes)
            $ms.Write([BitConverter]::GetBytes([int16]32), 0, 2)
            # Image size (4 bytes)
            $imgSize = $img.Data.Length
            $ms.Write([BitConverter]::GetBytes([int32]$imgSize), 0, 4)
            # Image offset (4 bytes)
            $ms.Write([BitConverter]::GetBytes([int32]$dataOffset), 0, 4)
            
            $imageData += $img.Data
            $dataOffset += $imgSize
        }
        
        # Write image data
        foreach ($data in $imageData) {
            $ms.Write($data, 0, $data.Length)
        }
        
        # Write to file
        [System.IO.File]::WriteAllBytes($icoPath, $ms.ToArray())
        
        $size = (Get-Item $icoPath).Length
        Write-Host "Created: $icoPath ($size bytes)"
    }
    
} finally {
    # Cleanup
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir
    }
}

Write-Host "Done!"
