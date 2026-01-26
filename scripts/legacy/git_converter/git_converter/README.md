# GIT Converter v2.7 - Setup Instructions

## Overview
GIT Converter v2.7 is a modular video conversion tool with automatic hardware detection and optimization. It supports AV1 and HEVC encoding with various quality settings, color correction options, and smart upscaling.

## File Structure
```
git_converter/
├── git_converter.sh          # Main script
├── modules/
│   ├── hardware.sh          # GPU detection & encoder selection
│   ├── quality.sh          # Quality settings & parameters
│   ├── filters.sh          # Scaling, color, FPS options
│   └── encode.sh           # Core ffmpeg execution
└── README.md              # This file
```

## Setup Instructions

### 1. File Structure Setup
- Ensure all files remain in the same directory structure
- The `modules/` folder must be alongside the main script
- Do not rename or move individual module files

### 2. Make Script Executable
**Windows:**
1. Right-click on `git_converter.sh`
2. Properties → Permissions → "Allow executing file"
3. Click OK

**Or in Git Bash:**
```bash
chmod +x git_converter.sh
```

### 3. Dependencies
- **FFmpeg** with encoder support (libx265, libsvtav1, hevc_amf, etc.)
- **Git Bash** (for Windows users)
- **GPU Drivers** (NVIDIA, AMD, or Intel) for hardware acceleration

## Usage Methods

### Method 1: Double-Click (Batch Processing)
1. **Double-click** `git_converter.sh`
2. **Hardware detection** runs automatically:
   ```
   Detecting hardware and optimal encoder...
    ✓ AMD GPU detected
    Testing hevc_amf...
       hevc_amf: 0.8s
    ✓ Selected: hevc_amf (fastest encoder)
   ```
3. **Follow menu prompts** for resolution, FPS, quality, and color correction
4. **Processes all videos** in the same directory

### Method 2: Drag & Drop (Selective Processing)
1. **Select video files** in Windows Explorer
2. **Drag them onto** `git_converter.sh`
3. **Same menu flow** as double-click method
4. **Processes only dragged files**

## Menu Options

### Resolution Settings
- **Source**: No upscaling (original resolution)
- **720p/1080p/1440p/4K**: Fixed resolution upscaling
- **2X/4X**: Relative upscaling (double/quadruple size)

### Scaling Algorithm (when upscaling)
- **Bicubic**: Fast, good quality
- **Lanczos**: Best quality, slower
- **Bilinear**: Fastest, basic quality

### FPS Settings
- **Original FPS**: Maintains source frame rate
- **60 FPS**: Converts to smooth 60fps motion

### Quality Modes
- **High Quality (CRF 18)**: Recommended balance
- **Near-Lossless (CRF 16)**: Maximum quality
- **Good Quality (CRF 20)**: Balanced size/quality
- **Custom bitrate**: Manual bitrate control

### Color Correction Options
1. **No correction**: Original colors
2. **Pink skin tones**: Fixes Topaz AI color loss
3. **Warm boost**: Increases warmth
4. **Cool boost**: Increases cool tones
5. **2000s DVD Restore**: Fixes DVD degradation
6. **90s Quality Restore**: Restores 90s media
7. **VHS Quality Restore**: Repairs VHS degradation
8. **Anime Preservation**: Protects anime art style

## Hardware Detection

The script automatically detects and tests:
- **NVIDIA GPUs**: Uses NVENC encoders
- **AMD GPUs**: Uses AMF encoders  
- **Intel GPUs**: Uses Quick Sync Video
- **No GPU**: Uses optimized CPU encoders

### Encoder Priority
1. **Hardware encoders** (GPU acceleration)
2. **SVT-AV1** (fast CPU encoding)
3. **libx265** (fallback CPU encoding)

## Output Structure

### Converted Folder
- Automatically created in script directory
- Contains all converted files
- Original files remain untouched

### File Naming Convention
```
[original_name]_[fps]_[color]__cv.[extension]

Examples:
- vacation.mp4 → vacation__cv.mkv
- wedding.mp4 → wedding_60fps__cv.mkv
- old_tape.avi → old_tape_vhsrestore__cv.mkv
- anime.mkv → anime_anime__cv.mkv
```

## Performance Features

### Optimizations
- **Hardware detection**: Uses fastest available encoder
- **Modular loading**: Faster startup, less memory
- **CRF encoding**: Consistent quality vs variable bitrate
- **Smart filter chains**: Proper ffmpeg parameter ordering

### Quality Settings
- **AV1**: Best compression, smaller files
- **HEVC**: Faster encoding, good compression
- **CRF 16-22**: Near-lossless to balanced quality
- **GPU acceleration**: 10x+ faster than CPU encoding

## Troubleshooting

### Common Issues

#### "No video files found"
- **Cause**: Script folder contains no supported videos
- **Solution**: Move script to folder with videos or drag files onto it

#### "No working encoder found"
- **Cause**: FFmpeg missing encoder support
- **Solution**: Install GPU drivers or rebuild FFmpeg with encoder support

#### "Failed to process"
- **Cause**: Permission issues, disk space, or corrupted file
- **Solution**: Check file permissions and available disk space

#### Slow encoding (0.1 FPS)
- **Cause**: Using slow CPU encoder (libaom-av1)
- **Solution**: Script auto-selects fastest encoder, ensure GPU drivers are installed

### Supported Formats
- **Input**: MP4, MKV, MOV, AVI, WMV, TS, M2TS
- **Output**: MKV (recommended) or MP4
- **Codecs**: AV1, HEVC (H.265)

### Performance Tips
- **For quality**: Use Lanczos scaling + CRF 16-18
- **For speed**: Use Bicubic scaling + CRF 20
- **For storage**: Use HEVC + CRF 20-22
- **For old content**: Use appropriate color correction option

## Advanced Usage

### Keyboard Shortcuts
- **Enter**: Confirm selections
- **Numbers**: Quick menu selection
- **Ctrl+C**: Cancel script

### Custom Workflows
- **Topaz AI videos**: Use "Pink skin tones" correction
- **DVD content**: Use "2000s DVD Restore"
- **Anime**: Use "Anime Preservation" mode
- **Family videos**: Use "Warm color boost"

### Batch Processing
- Place multiple videos in same folder
- Double-click script for batch conversion
- All files processed with same settings

## Version Information
- **Version**: v2.7 Modular Edition
- **Author**: LeakTechnologies
- **Release**: December 2025
- **Architecture**: Modular for performance optimization

## Support
For issues or feature requests, ensure:
1. All files in correct directory structure
2. FFmpeg properly installed with encoder support
3. GPU drivers up to date (if using hardware acceleration)
4. Sufficient disk space for output files