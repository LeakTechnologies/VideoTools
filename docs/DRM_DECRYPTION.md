# DRM/Decryption Support

## Current Implementation

### DVD CSS (Content Scramble System)

**Status**: Implemented

**Package**: `internal/dvd/css`

**Capabilities**:
- CSS encryption detection (`IsCSSEncrypted`, `IsEncrypted`)
- Stream cipher decryption (`Decryptor`, `DecryptSector`)
- VOB file decryption (`DecryptVOB`, `DecryptVOBSet`)
- Read-through decryption (`DecryptReader`)

**Usage**:
```go
import "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/css"

// Check if encrypted
encrypted, err := css.IsCSSEncrypted("VIDEO_TS.IFO")

// Decrypt with known title key
d := css.NewDecryptor(titleKey)
reader := css.NewDecryptReader(vobFile, d)
```

**Integration**:
- Automatically detects CSS encryption in rip executor
- Decrypts during streaming to avoid temp files
- Integrated into `internal/app/modules/rip/executor.go`

## Future Work

### Blu-ray AACS (Advanced Access Content System)

**Status**: Planned

**Complexity**: High
- AES-128 encryption
- Media Key Block (MKB) processing
- Volume ID extraction
- Host certificate authentication

**Package**: `internal/dvd/aacs` (to be created)

### Blu-ray BD+

**Status**: Planned

**Complexity**: Very High
- Virtual machine based protection
- Dynamic code execution
- Requires BD+ tables

**Package**: `internal/dvd/bdplus` (to be created)

### DIVX DRM

**Status**: Not Started

**Complexity**: Medium
- Various proprietary schemes
- May require format-specific handling

## Architecture

```
internal/dvd/
├── css/        # DVD CSS decryption (implemented)
├── aacs/       # Blu-ray AACS (planned)
├── bdplus/     # Blu-ray BD+ (planned)
├── ifo/        # IFO parsing
├── spu/         # Subpicture Unit
├── theme/       # Menu theming
├── udf/         # UDF filesystem
└── vob/         # VOB parsing
```

## Legal Note

This decryption support is for **archival purposes only**:
- Users ripping their own legally-owned media
- Media server backup (e.g., Plex, Jellyfin)
- Preservation of physical media

VideoTools does not circumvent copy protection on media the user does not own.

---

## DVD Region Codes

The Author module supports setting region codes during DVD creation:

### Implementation Status

- [x] Region constants defined in `internal/dvd/region/region.go`
- [x] Region field added to `appState.authorPlaybackRegion`
- [ ] UI selector in Author settings (pending)
- [ ] VMG_Category field populated during IFO generation (pending)

### Region Codes

| Code | Value | Description |
|------|-------|-------------|
| `RegionFree` | `0x00FFFFFE` | Plays everywhere (default for archival) |
| `Region1` | `0x00000001` | USA, Canada, US Territories |
| `Region2` | `0x00000002` | Europe, Japan, South Africa, Middle East |
| `Region3` | `0x00000004` | Southeast Asia, East Asia |
| `Region4` | `0x00000008` | Latin America, Australia, New Zealand |
| `Region5` | `0x00000010` | Africa, Russia, North Korea, South Asia |
| `Region6` | `0x00000020` | China |
| `Region7` | `0x00000040` | Reserved (special use) |
| `Region8` | `0x00000080` | International venues (airlines, cruise ships) |

### Usage

```go
import "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/region"

// Region-free (recommended for archival)
cat := region.RegionFreeCategory()

// Multi-region (e.g., regions 1, 2, 4)
cat := region.Category(region.Region1 | region.Region2 | region.Region4)
```

### UI Integration (Planned)

Author settings will include a region dropdown:
- **Region-free (recommended)** - Default
- **Region 1** - USA/Canada
- **Region 2** - Europe/Japan
- **Multi-region...** - Submenu with checkbox combinations
- **Custom...** - Hex input for advanced users

### Author Workflow

1. User selects region in Author settings
2. Region stored in `appState.authorPlaybackRegion`
3. During IFO generation, `VMG_Category` field populated with region mask
4. Resulting DVD/ISO plays in players matching the region