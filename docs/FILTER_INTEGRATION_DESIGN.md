# Filter Integration Design

## Problem

Currently filters and upscale are separate modules, forcing users to jump between them when making adjustments. This breaks workflow continuity.

## Design Vision

### Two-Module Approach

1. **Upscale Module** (enhanced)
   - Contains all upscale/AI options
   - **Integrated filters section** - same filters as standalone module
   - Single place for all video enhancement adjustments
   - Order: Input → Filters → Upscale → Output

2. **Filters Module** (standalone)
   - Apply filters WITHOUT upscale
   - Use cases: color correction on existing good resolution, denoise without resize, etc.
   - Lightweight: just filter chain → output

### UI Layout for Upscale

```
┌─────────────────────────────────────────┐
│ [Upscale] tabs: [AI Settings] [Filters] │
├─────────────────────────────────────────┤
│ Preview                                 │
├─────────────────────────────────────────┤
│ Filters Section (NEW)                   │
│ ├─ Denoise: [○ Off ○ Low ● Med ○ High] │
│ ├─ Deblock: [○ Off ● On]               │
│ ├─ Sharpen: [slider 0-100]              │
│ ├─ Color:  [○ Off ● On]                │
│ │    ├─ Brightness: [====●===]         │
│ │    ├─ Contrast:   [====●===]         │
│ │    ├─ Saturation: [====●===]         │
│ │    └─ Gamma:      [====●===]         │
│ └─ Deinterlace: [○ Off ○ On]           │
├─────────────────────────────────────────┤
│ AI Upscale Section (existing)           │
│ ├─ Model: [○ RealESRGAN ● RealCUGAN]   │
│ ├─ Scale: [○ 2x ● 4x]                  │
│ └─ Tile:  [○ Off ● On]                 │
├─────────────────────────────────────────┤
│ [Preview] [Add to Queue]                │
└─────────────────────────────────────────┘
```

### Implementation Approach

#### Phase 1: Move Filters into Upscale Module
1. Extract current filter UI code from `main.go` buildFiltersView
2. Add as tab/section in upscale module
3. Ensure filter parameters passed to upscale execution

#### Phase 2: Refactor Execute Pipeline
- `executeUpscaleJob` needs to accept filter config
- Build filter chain BEFORE upscale in FFmpeg pipeline
- Order: source → filters → AI upscale → output

#### Phase 3: Keep Filters Standalone
- Current filters module stays as-is
- For non-upscale filter jobs
- Can be queued independently

### Files to Modify

1. **`main.go`**
   - Extract filter UI to separate function
   - Add filter section to upscale view
   - Update `executeUpscaleJob` to accept filters

2. **`upscale_module.go`** (or `internal/app/modules/upscale/`)
   - Add filter tab/section
   - Wire filter parameters to execution

3. **`filters_module.go`**
   - Keep as standalone
   - Minimal changes needed

### Key Technical Changes

#### Filter Execution Order
```go
// Current order in executeUpscaleJob:
// input → upscale → output

// New order:
// input → [denoise → deblock → sharpen → color] → upscale → output
```

#### Config Structure
```go
type upscaleConfig struct {
    // Existing fields
    Model   string
    Scale   int
    Tile    bool
    
    // New filter fields
    Denoise     string  // off, low, medium, high
    Deblock     bool
    Sharpen     float64 // 0-100
    ColorCorrect colorCorrectConfig
    
    // Existing fields
    // ...
}
```

### Testing Checklist

- [ ] Upscale with denoise only
- [ ] Upscale with sharpen only
- [ ] Upscale with color correction only
- [ ] Upscale with ALL filters combined
- [ ] Verify filter order: filters applied BEFORE upscale
- [ ] Filters standalone still works (no upscale)
- [ ] Preview updates when filters changed

### Benefits

1. **Workflow continuity** - All adjustments in one module
2. **Performance** - Filter + upscale in single encode
3. **Flexibility** - Standalone filters for simple jobs
4. **Discoverability** - All options visible at once
