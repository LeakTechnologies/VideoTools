# Filter Integration Design

## Problem

Currently filters and upscale are separate modules, forcing users to jump between them when making adjustments. This breaks workflow continuity. Additionally, the Convert module does not have video filters available - users must use the standalone Filters module separately.

## Design Vision

### Three-Module Approach

1. **Convert Module** (primary encoding)
   - Add video filters section for standard encoding
   - Use cases: encode with denoise, deinterlace, sharpen, etc.
   - Order: Input → Filters → Encode → Output

2. **Upscale Module** (enhanced)
   - Contains all upscale/AI options
   - **Integrated filters section** - same filters as Convert module
   - Single place for all video enhancement adjustments
   - Order: Input → Filters → Upscale → Output

3. **Filters Module** (standalone)
   - Apply filters WITHOUT encode/upscale
   - Use cases: color correction on existing good resolution, denoise without resize, etc.
   - Lightweight: just filter chain → output

---

## Part 1: Convert Module Filter Integration

### UI Layout for Convert

```
┌─────────────────────────────────────────┐
│ [Convert] tabs: [Video] [Audio] [Filters]│
├─────────────────────────────────────────┤
│ Source File: [________________] [Browse] │
│ Output:    [________________] [Browse]  │
├─────────────────────────────────────────┤
│ Format: [MP4 (H.264) ▼]                  │
│ Codec:  [H.264 ▼]     Quality: [CRF 23] │
├─────────────────────────────────────────┤
│ [Filters Tab Content]                   │
├─────────────────────────────────────────┤
│ Deinterlace: [○ Off ○ Auto ● On]        │
│ Denoise:    [○ Off ○ Low ○ Med ○ High] │
│ Sharpen:    [========●====] 0-100       │
│ Colorspace: [○ Keep ○ BT.709 ○ BT.2020] │
├─────────────────────────────────────────┤
│ [Start Conversion]                       │
└─────────────────────────────────────────┘
```

### FFmpeg Filter Chain

Filters applied in this order:
1. **Deinterlace** - yadif (motion-adaptive)
2. **Denoise** - nlmeans (quality) or hqdn3d (fast)
3. **Deblock** - deblock (reduce compression artifacts)
4. **Sharpen** - unsharp (enhance edges)
5. **Colorspace** - colorspace conversion

### Filter Parameters

| Filter | Options | FFmpeg |
|--------|---------|--------|
| Deinterlace | Off, Auto, On | `yadif=mode=1` (on), `yadif=mode=2` (auto) |
| Denoise | Off, Light, Medium, Strong | `nlmeans=s=2` (light), `nlmeans=s=6` (med), `nlmeans=s=10` (strong) |
| Sharpen | 0-100 (default 50) | `unsharp=la=0.5` (50%) |
| Deblock | Off, On | `deblock=alpha=0:beta=0` |
| Colorspace | Keep, BT.709, BT.2020, BT.601 | `colorspace=bt709` etc. |

### Implementation for Convert

#### Config Structure Additions
```go
type convertConfig struct {
    // ... existing fields ...

    // Filter settings
    Deinterlace string  // "off", "auto", "on"
    Denoise     string  // "off", "light", "medium", "strong"
    Sharpen     float64 // 0-100
    Deblock     bool
    Colorspace  string  // "keep", "bt709", "bt2020", "bt601"
}
```

#### FFmpeg Build Logic
```go
func buildFilterChain(cfg convertConfig) string {
    var filters []string

    if cfg.Deinterlace == "on" || cfg.Deinterlace == "auto" {
        mode := map[string]string{"on": "1", "auto": "2"}[cfg.Deinterlace]
        filters = append(filters, fmt.Sprintf("yadif=mode=%s", mode))
    }

    if cfg.Denoise != "off" {
        strength := map[string]string{"light": "2", "medium": "6", "strong": "10"}[cfg.Denoise]
        filters = append(filters, fmt.Sprintf("nlmeans=s=%s", strength))
    }

    if cfg.Deblock {
        filters = append(filters, "deblock=alpha=0:beta=0")
    }

    if cfg.Sharpen > 0 {
        amount := cfg.Sharpen / 100.0
        filters = append(filters, fmt.Sprintf("unsharp=la=%.2f", amount))
    }

    if cfg.Colorspace != "keep" {
        filters = append(filters, fmt.Sprintf("colorspace=%s", cfg.Colorspace))
    }

    return strings.Join(filters, ",")
}
```

#### Files to Modify for Convert

1. **`main.go`**
   - Add Filters tab to Convert view
   - Add filter controls (dropdowns, sliders)
   - Update `startConversion` to include filters
   - Update `buildConversionArgs` to add filter chain

2. **`internal/convert/types.go`**
   - Add filter fields to `ConvertConfig`

3. **`internal/app/modules/convert/view.go`**
   - Add `BuildFiltersTab()` function

4. **i18n**
   - Add filter labels (ConvertDeinterlace, ConvertDenoise, etc.)

---

## Part 2: Upscale Module Filter Integration

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
