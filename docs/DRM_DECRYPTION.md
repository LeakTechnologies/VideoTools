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
import "git.leaktechnologies.dev/stu/VideoTools/internal/dvd/css"

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