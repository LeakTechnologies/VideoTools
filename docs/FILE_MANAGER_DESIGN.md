# File Manager Design

## Overview

The **File Manager** provides a built-in file explorer for quick navigation between projects and files. Unlike system file managers, it's optimized for VideoTools workflows with context menus that launch files directly into appropriate modules.

## Core Principles

- **Lightweight** — Minimal UI, fast startup, low memory
- **Easy navigation** — Breadcrumb trail, recent folders, favorites
- **Multi-task** — Open multiple folders in tabs
- **Module integration** — Right-click to send files to Convert, Audio, Subtitles, etc.

## UI Design

```
┌─────────────────────────────────────────────────────────────────────┐
│ ▼ Home  ▼ Documents  ▼ VideoProjects  [+ New Tab]                  │
├─────────────────────────────────────────────────────────────────────┤
│ /home/user/Videos/                              [🔍 Search]        │
├──────────────────────────────────────────────────────────────────────┤
│ Name                  │ Size    │ Modified    │ Type              │
├───────────────────────┼─────────┼─────────────┼───────────────────┤
│ 📁 Project_Season1    │         │ Apr 2 2026  │ Folder            │
│ 📁 Raw_Footage       │         │ Mar 28 2026 │ Folder            │
│ 🎬 interview_01.mp4  │ 2.4 GB  │ Mar 15 2026 │ Video (MP4)       │
│ 🎬 interview_02.mp4 │ 1.8 GB  │ Mar 15 2026 │ Video (MP4)       │
│ 🔊 ambient_audio.wav │ 45 MB   │ Mar 10 2026 │ Audio (WAV)       │
│ 📝 notes.txt         │ 4 KB    │ Mar 1 2026  │ Text              │
└──────────────────────────────────────────────────────────────────────┘
```

### Layout

1. **Tab Bar** — Multiple open folders, closeable tabs
2. **Breadcrumb** — Clickable path segments
3. **Toolbar** — Back, Forward, Up, Search, View toggle
4. **File List** — Sortable columns (Name, Size, Modified, Type)
5. **Status Bar** — Item count, selected size

### Icons with Colour Pills

```
🎬 → [Convert]   (red pill)
🔊 → [Audio]     (orange pill)  
📝 → [Subtitles] (yellow pill)
📁 → [Folder]    (grey pill)
```

## Features

### Phase 1: Basic Navigation
- [x] List files with name, size, date, type
- [x] Navigate folders (double-click, breadcrumb)
- [x] Back/Forward history
- [x] Sort by column click
- [ ] Search filter

### Phase 2: Multi-Tab
- [ ] Open folders in new tabs
- [ ] Drag tabs to reorder
- [ ] Close tab (X button)
- [ ] Middle-click to open in new tab

### Phase 3: Module Integration
- [ ] Right-click context menu
- [ ] "Open in Convert" → red pill
- [ ] "Open in Audio" → orange pill
- [ ] "Open in Subtitles" → yellow pill
- [ ] Module-appropriate file filtering
- [ ] "Open in Inspect" → green pill
- [ ] "Open in Author" → teal pill

### Phase 4: Favorites & Recent
- [ ] Star folders as favorites
- [ ] Recent folders list
- [ ] Quick access sidebar

## Technical Implementation

### Entry Points
- Main menu → "File Manager" button
- Drag file from explorer into app → opens in File Manager
- Keyboard shortcut: Ctrl+F (global search)

### File Type Detection
```go
var fileTypeIcons = map[string]struct{
    icon string
    module string
}{
    ".mp4":  {"🎬", "convert"},
    ".mkv":  {"🎬", "convert"},
    ".avi":  {"🎬", "convert"},
    ".wav":  {"🔊", "audio"},
    ".mp3":  {"🔊", "audio"},
    ".srt":  {"📝", "subtitles"},
    ".ass":  {"📝", "subtitles"},
    ".vob":  {"🎬", "author"},
    ".iso":  {"💿", "burn"},
}
```

### Context Menu Structure

```
┌─────────────────────────────────────┐
│ Open                    Ctrl+Enter │
├─────────────────────────────────────┤
│ Open in → Convert      [●]         │
│           Audio        [●]         │
│           Subtitles    [●]         │
│           Inspect      [●]         │
│           Author       [●]         │
├─────────────────────────────────────┤
│ Copy                     Ctrl+C    │
│ Rename                   F2        │
│ Delete                   Del       │
├─────────────────────────────────────┤
│ Properties                       │
└─────────────────────────────────────┘
```

### State Management

```go
type FileManagerState struct {
    Tabs        []Tab
    ActiveTab   int
    History     []string  // back/forward
    Favorites   []string
    Recent      []string
}

type Tab struct {
    Path    string
    Files   []FileEntry
    Scroll  int
}
```

## File Naming

- Module file: `filemanager_module.go`
- Platform helpers: `filemanager_windows.go`, `filemanager_linux.go`
- UI components: `internal/ui/filemanager.go`

## Testing Checklist

- [ ] Navigate to folder with 100+ files (performance)
- [ ] Open 5 tabs, switch between them
- [ ] Right-click video → "Open in Convert" launches module
- [ ] Right-click audio → "Open in Audio" launches module
- [ ] Sort by size (large files first)
- [ ] Breadcrumb click navigates to parent
- [ ] Back/Forward buttons work