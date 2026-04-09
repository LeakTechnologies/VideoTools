# Batch Queue Design

## Overview

Batch queue allows users to add multiple files to the queue at once, rather than one at a time. This is essential for workflows involving many videos.

## Problem

Currently users must add files one-by-one to the queue. This is time-consuming when working with:
- Multiple episode files from a series
- All files in a folder
- Entire disc title set

## Design Vision

### Three Batch Add Modes

1. **Add Multiple Files** — Select specific files (existing file dialog)
2. **Add Folder** — Add all video files from a folder
3. **Add Disc Titles** — Add all titles from DVD/Blu-ray

---

## UI Layout

### Option 1: Menu Bar Integration

```
File menu:
├── [Add Files to Queue...]     (Ctrl+Shift+A)
├── [Add Folder to Queue...]    (Ctrl+Shift+F)
└── [Add Disc Titles to Queue]
```

### Option 2: Queue Panel Buttons

```
┌─────────────────────────────────────────┐
│ Queue                            [─][□][×] │
├─────────────────────────────────────────┤
│ [+ Add] [Folder] [Disc] [Clear] [Start] │
├─────────────────────────────────────────┤
│ ┌─────────────────────────────────────┐ │
│ │ 1. video1.mp4     [Encoding...] 45%│ │
│ │ 2. video2.mp4     [Pending]        │ │
│ │ 3. video3.mp4     [Pending]        │ │
│ │ 4. video4.mp4     [Pending]        │ │
│ └─────────────────────────────────────┘ │
├─────────────────────────────────────────┤
│ Total: 4 jobs | Estimated: 45 min     │
└─────────────────────────────────────────┘
```

### Option 3: File Manager Integration

In File Manager, select multiple files → Right-click → "Add to Queue"

---

## Batch Add Dialogs

### Add Folder Dialog

```
┌─────────────────────────────────────────┐
│ Add Folder to Queue                     │
├─────────────────────────────────────────┤
│ Folder: [_________________] [Browse]   │
│                                     [✓] │
│ Include: [V] .mp4                      │
│         [V] .mkv                      │
│         [V] .mov                      │
│         [V] .avi                      │
│         [V] .wmv                      │
├─────────────────────────────────────────┤
│ Found: 12 video files                   │
├─────────────────────────────────────────┤
│ Preset: [Current Settings ▼]           │
│         [Save as Default]               │
├─────────────────────────────────────────┤
│        [Cancel]  [Add to Queue]         │
└─────────────────────────────────────────┘
```

### Add Disc Titles Dialog

```
┌─────────────────────────────────────────┐
│ Add Disc Titles to Queue                │
├─────────────────────────────────────────┐
│ Source: [E:\] [Browse]                  │
├─────────────────────────────────────────┤
│ Detected Titles:                         │
│ ┌─────────────────────────────────────┐ │
│ │ [✓] Title 1 - 00:45:30 - 1080p      │ │
│ │ [✓] Title 2 - 00:42:15 - 1080p      │ │
│ │ [ ] Title 3 - 00:10:00 - 480p       │ │
│ │ [✓] Title 4 - 01:15:00 - 720p      │ │
│ └─────────────────────────────────────┘ │
├─────────────────────────────────────────┤
│ [Select All] [Deselect All]            │
├─────────────────────────────────────────┤
│ Preset: [Use Disc Settings] [Custom ▼] │
├─────────────────────────────────────────┤
│        [Cancel]  [Add to Queue]         │
└─────────────────────────────────────────┘
```

---

## Implementation Details

### Queue Job Structure

```go
type QueueJob struct {
    ID          string
    SourcePath  string
    OutputPath  string
    Config      convertConfig
    Status      JobStatus // pending, encoding, completed, failed
    Progress    float64   // 0-100
    Error       string
    AddedAt     time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
}

type BatchAddRequest struct {
    Paths        []string
    FolderPath   string
    DiscPath     string
    FileTypes    []string // mp4, mkv, etc.
    Preset       string   // "current", "default", or custom config
    UseSourceDir bool     // output to same folder as source
}
```

### Adding Multiple Files

```go
func AddFilesToQueue(paths []string, cfg convertConfig) []QueueJob {
    var jobs []QueueJob
    for _, path := range paths {
        job := QueueJob{
            ID:         generateJobID(),
            SourcePath: path,
            Config:     cfg,
            Status:     Pending,
        }
        jobs = append(jobs, job)
    }
    queue.AddJobs(jobs)
    return jobs
}
```

### Adding Folder

```go
func AddFolderToQueue(folderPath string, fileTypes []string, cfg convertConfig) ([]QueueJob, error) {
    var jobs []QueueJob
    
    files, err := filepath.Glob(filepath.Join(folderPath, "*"))
    if err != nil {
        return nil, err
    }
    
    for _, file := range files {
        ext := strings.ToLower(filepath.Ext(file))
        if isVideoFile(ext, fileTypes) {
            job := createJobFromFile(file, cfg)
            jobs = append(jobs, job)
        }
    }
    
    queue.AddJobs(jobs)
    return jobs, nil
}
```

### Adding Disc Titles

```go
func AddDiscTitlesToQueue(drivePath string, titleIndices []int, cfg convertConfig) ([]QueueJob, error) {
    titles, err := scanDiscTitles(drivePath)
    if err != nil {
        return nil, err
    }
    
    var jobs []QueueJob
    for _, idx := range titleIndices {
        if idx < len(titles) {
            job := createJobFromDiscTitle(titles[idx], cfg)
            jobs = append(jobs, job)
        }
    }
    
    queue.AddJobs(jobs)
    return jobs, nil
}
```

---

## File Filters

### Default Video Extensions

| Extension | Description |
|-----------|-------------|
| .mp4 | MPEG-4 Part 14 |
| .mkv | Matroska |
| .mov | QuickTime |
| .avi | AVI |
| .wmv | Windows Media |
| .webm | WebM |
| .m4v | iTunes |
| .flv | Flash Video |
| .mpg/.mpeg | MPEG |
| .ts | MPEG-2 Transport Stream |
| .m2ts | MPEG-2 Transport Stream (Blu-ray) |

### Filtering Logic

```go
func isVideoFile(ext string, includeTypes []string) bool {
    videoExts := map[string]bool{
        ".mp4": true, ".mkv": true, ".mov": true,
        ".avi": true, ".wmv": true, ".webm": true,
        ".m4v": true, ".flv": true, ".mpg": true,
        ".mpeg": true, ".ts": true, ".m2ts": true,
    }
    
    if len(includeTypes) > 0 {
        for _, t := range includeTypes {
            if "."+t == ext {
                return true
            }
        }
        return false
    }
    
    return videoExts[ext]
}
```

---

## Queue Processing

### Sequential vs Parallel

| Mode | Description | Use Case |
|------|--------------|----------|
| Sequential | One job at a time | System resources limited |
| Parallel | Multiple jobs (N encoder instances) | Fast hardware |

### Settings

```go
type QueueSettings struct {
    ParallelJobs int    // 1-4, 0=auto
    OnComplete   string // "notify", "shutdown", "next"
    OnError      string // "pause", "continue", "stop"
}
```

---

## Files to Modify

1. **`main.go`**
   - Add batch add menu items
   - Add queue panel buttons

2. **`queue/*.go`**
   - Add batch add methods
   - Add folder scanning logic

3. **`internal/convert/types.go`**
   - Add batch add request types

4. **Disc scanning**
   - Use existing disc scanning from Rip module

5. **i18n**
   - Add batch add labels

---

## Testing Checklist

- [ ] Add multiple files via file dialog
- [ ] Add folder with all video files
- [ ] Add folder with specific file types only
- [ ] Add disc titles to queue
- [ ] Select/deselect titles in disc dialog
- [ ] Queue processes jobs in order
- [ ] Pause/resume queue works
- [ ] Clear queue removes all jobs

---

## Reference

- Reference: Queue system documentation

---

*Last updated: 2026-04-09*
*Status: Ready for implementation*