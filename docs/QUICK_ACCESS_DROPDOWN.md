# Quick Access Files Dropdown

## Overview

The **Files dropdown** is a context-aware quick access menu in the main menu header that provides instant access to file operations based on the current module. It replaces a simple button with a dropdown that adapts to what the user is doing.

## Location

Appears in the main menu header, between the sidebar toggle (☰) and the Queue tile.

```
┌─────────────────────────────────────────────────────────────┐
│ VideoTools          ☰ Files   Logs          Queue: 0/0    │
└─────────────────────────────────────────────────────────────┘
```

## Features

### 1. Open Files...
Opens the file picker for the current module. Enables quick loading of new media without navigating through the module tiles.

**Behavior varies by module:**
- **Convert/Merge/Trim**: Opens video file picker
- **Audio**: Opens video/audio file picker
- **Subtitles**: Opens subtitle file picker (.srt, .vtt, .mks)
- **Author**: Opens video folder/file picker for disc authoring

### 2. Open Output Folder
Opens the output directory of the current job or the default output folder for the current module.

**Shown only when:**
- A file is loaded in the current module, OR
- The module has a default output directory

### 3. Recent Files
Lists the 10 most recently opened files across all modules.

**Each entry shows:**
- File name (truncated if > 30 chars)
- Module it was opened in (in parentheses)

**Clicking a recent file:**
- Opens the file in the module it was last used with
- Loads the file directly as if the user had dropped it

## Context Awareness

The dropdown adapts its options based on the current module:

| Module | Open Files | Open Output | Recent Files |
|--------|------------|-------------|--------------|
| Convert | ✓ | ✓ | ✓ |
| Merge | ✓ | ✓ | ✓ |
| Trim | ✓ | ✓ | ✓ |
| Filters | ✓ | ✓ | ✓ |
| Audio | ✓ | ✓ | ✓ |
| Subtitles | ✓ | ✓ | ✓ |
| Author | ✓ | ✓ | ✓ |
| Rip | ✓ | ✓ | ✓ |
| Inspect | ✓ | - | ✓ |
| Player | ✓ | - | ✓ |
| Upscale | ✓ | ✓ | ✓ |
| Compare | ✓ | - | ✓ |
| Thumbnail | ✓ | ✓ | ✓ |
| Settings | - | - | - |

## Data Structure

```go
type FilesDropdownData struct {
    CurrentModule string      // Current active module ID
    RecentFiles   []RecentFile
    OnFileClick   func(string)
    OnOpenFolder  func()
    OnOpenMore    func()
}

type RecentFile struct {
    Path        string
    DisplayName string
    Module      string
}
```

## Implementation

- **UI Package**: `internal/ui/mainmenu.go`
- **Build Function**: `buildFilesDropdown()`
- **Labels**: All user-visible strings are i18n'd in `internal/i18n/`
- **Triggers**: Button click shows Fyne pop-up menu

## Future Enhancements

1. **Favorites** — Star frequently used folders
2. **Search** — Quick search within recent files
3. **Drag out** — Drag files from dropdown to desktop
4. **Pinned files** — Pin important files to top
