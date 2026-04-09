# Custom Presets Design

## Overview

Custom presets allow users to save their frequently used conversion settings and quickly apply them later. This saves time for users who encode with the same settings repeatedly.

## Problem

Users must manually configure settings each time they encode:
- Same codec, quality, and format preferences
- Same audio settings
- Same filters
- Same output naming convention

## Design Vision

### Preset Storage

- **Location:** User config directory (`~/.config/VideoTools/presets.json`)
- **Format:** JSON with all convert settings
- **Default preset:** Can set one default that loads on startup

---

## UI Layout

### Preset Panel in Convert Module

```
┌─────────────────────────────────────────┐
│ Preset: [My Preset 1 ▼] [★] [Save] [Del]│
├─────────────────────────────────────────┤
│ Current Settings (loaded from preset):   │
│ ┌─────────────────────────────────────┐ │
│ │ Format:  MP4 (H.264)                │ │
│ │ Codec:   H.264                      │ │
│ │ Quality: CRF 20                     │ │
│ │ Audio:   AAC 192k                   │ │
│ │ Filters: Denoise Medium            │ │
│ └─────────────────────────────────────┘ │
├─────────────────────────────────────────┤
│ [Load Preset...] [Save as New...]       │
│ [Export Preset...] [Import Preset...]   │
└─────────────────────────────────────────┘
```

### Save Preset Dialog

```
┌─────────────────────────────────────────┐
│ Save Preset                             │
├─────────────────────────────────────────┤
│ Name: [________________________]        │
│                                             │
│ Description: (optional)                    │
│ [________________________________]        │
│                                             │
│ [✓] Save as default preset               │
├─────────────────────────────────────────┤
│        [Cancel]  [Save]                   │
└─────────────────────────────────────────┘
```

### Import/Export

- **Export:** Save preset to `.vtpreset` JSON file
- **Import:** Load preset from `.vtpreset` file

---

## Preset Data Structure

```go
type ConvertPreset struct {
    ID          string    `json:"id"`          // UUID
    Name        string    `json:"name"`         // User-defined name
    Description string    `json:"description"`  // Optional description
    IsDefault   bool      `json:"isDefault"`    // Default on startup
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
    
    // All convert config fields
    ConvertConfig convertConfig `json:"config"`
}

type PresetStore struct {
    Presets []ConvertPreset `json:"presets"`
    Version int              `json:"version"`
}
```

### Preset File Example

```json
{
  "version": 1,
  "presets": [
    {
      "id": "uuid-1234-5678",
      "name": "Web Upload",
      "description": "Optimized for web upload",
      "isDefault": true,
      "createdAt": "2026-04-09T10:00:00Z",
      "updatedAt": "2026-04-09T10:00:00Z",
      "config": {
        "selectedFormat": {"label": "MP4 (H.264)", "ext": ".mp4", "videoCodec": "libx264"},
        "videoCodec": "H.264",
        "quality": "Standard (CRF 23)",
        "encoderPreset": "medium",
        "audioCodec": "AAC",
        "audioBitrate": "192k",
        "deinterlace": "off",
        "denoise": "medium"
      }
    }
  ]
}
```

---

## Implementation Details

### Save Preset

```go
func SavePreset(name string, cfg convertConfig) error {
    preset := ConvertPreset{
        ID:          generateUUID(),
        Name:        name,
        Config:      cfg,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    
    store, err := loadPresetStore()
    if err != nil {
        store = &PresetStore{Version: 1}
    }
    
    // Check for existing preset with same name
    for i, p := range store.Presets {
        if p.Name == name {
            store.Presets[i] = preset
            return savePresetStore(store)
        }
    }
    
    store.Presets = append(store.Presets, preset)
    return savePresetStore(store)
}
```

### Load Preset

```go
func LoadPreset(id string) (convertConfig, error) {
    store, err := loadPresetStore()
    if err != nil {
        return convertConfig{}, err
    }
    
    for _, p := range store.Presets {
        if p.ID == id {
            return p.Config, nil
        }
    }
    
    return convertConfig{}, fmt.Errorf("preset not found: %s", id)
}
```

### Set Default

```go
func SetDefaultPreset(id string) error {
    store, err := loadPresetStore()
    if err != nil {
        return err
    }
    
    for i := range store.Presets {
        store.Presets[i].IsDefault = (store.Presets[i].ID == id)
    }
    
    return savePresetStore(store)
}
```

### Export Preset

```go
func ExportPreset(id string, path string) error {
    preset, err := loadPreset(id)
    if err != nil {
        return err
    }
    
    data, err := json.MarshalIndent(preset, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(path, data, 0644)
}
```

### Import Preset

```go
func ImportPreset(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    
    var preset ConvertPreset
    if err := json.Unmarshal(data, &preset); err != nil {
        return err
    }
    
    // Generate new ID to avoid conflicts
    preset.ID = generateUUID()
    preset.IsDefault = false
    
    store, err := loadPresetStore()
    if err != nil {
        store = &PresetStore{Version: 1}
    }
    
    store.Presets = append(store.Presets, preset)
    return savePresetStore(store)
}
```

---

## Files to Modify

1. **`internal/app/presets/*.go`** (new package)
   - `store.go` — Save/load presets
   - `export.go` — Import/export

2. **`main.go`**
   - Add preset dropdown to Convert UI
   - Add save/load/delete buttons
   - Add import/export menu items

3. **`internal/convert/types.go`**
   - Ensure all config fields are serializable

4. **i18n**
   - Add preset labels

---

## Testing Checklist

- [ ] Save preset with current settings
- [ ] Load preset applies all settings
- [ ] Delete preset removes from store
- [ ] Set default loads on startup
- [ ] Export creates valid .vtpreset file
- [ ] Import loads preset correctly
- [ ] Import with same name asks to overwrite
- [ ] Presets persist across app restarts

---

## Reference

- Reference: Custom presets documentation

---

*Last updated: 2026-04-09*
*Status: Ready for implementation*