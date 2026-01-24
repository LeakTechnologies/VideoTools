# Convert Panel Modularization Plan (Dev24/25)

## 🎯 Goal
Move Advanced Convert UI logic out of main.go into modular UI components, keeping main.go as glue only.

## 📁 File Structure

```
internal/ui/
├── convert_advanced.go     # Advanced panel UI builder
├── convert_state.go         # State manager + callbacks  
├── convert_simple.go        # Simple panel UI builder (future)
└── convert_types.go          # Shared types and constants

main.go (cleanup)
├── Keep encoding/FFmpeg logic in existing helpers
├── Import internal/ui package only
└── Replace UI blocks with module calls
```

## 🔧 What to Extract from main.go

### 1. UI Builders (buildConvertView -> convert_advanced.go)
- Advanced panel dropdowns, sliders, toggles
- Layout containers and responsive sizing
- Quality presets and format selection
- Hardware acceleration controls
- Two-pass encoding interface
- Progress preview and command display

### 2. State Management (convertUIState -> convert_state.go)
```go
type ConvertState struct {
    // Current settings
    Format         formatOption
    Quality        string
    Preset         string
    TwoPass        bool
    HardwareAccel  bool
    
    // UI bindings
    FormatList     *widget.Select
    QualitySelect  *widget.Select
    // ... etc
}

type ConvertUIBindings struct {
    // Controls accessible to main.go
    StartConversion  func()
    StopConversion   func()
    ShowPreview     func()
    // ... etc
}
```

### 3. Callback Functions (applyQuality, setQuality, updateEncodingControls -> convert_state.go)
- State change management
- Validation and sanitization
- Settings persistence
- Progress update handling

## 🔄 Integration Pattern

### main.go Changes:
```go
import "git.leaktechnologies.dev/stu/VideoTools/internal/ui"

// Replace giant UI block with:
if useAdvanced {
    panel := ui.BuildConvertAdvancedPanel(state, src)
    mainContent := container.NewVBox(panel)
} else {
    panel := ui.BuildConvertSimplePanel(state, src)  
    mainContent := container.NewVBox(panel)
}
```

### Module Interface:
```go
func BuildConvertAdvancedPanel(state *appState, src *videoSource) (fyne.CanvasObject, *ConvertUIBindings)
func BuildConvertSimplePanel(state *appState, src *videoSource) (fyne.CanvasObject, *ConvertUIBindings)
func InitConvertState(state *appState, src *videoSource) *ConvertState
```

## 🎨 Benefits

1. **Maintainable**: UI logic separated from core logic
2. **Testable**: UI components can be unit tested independently
3. **Reusability**: Simple/Advanced panels reused in other modules
4. **Clean Code**: main.go becomes readable and focused
5. **Future-Proof**: Easy to add new UI features without bloating main.go

## 📋 Implementation Order

1. **Phase 1**: Create convert_types.go with shared types
2. **Phase 2**: Extract state management into convert_state.go
3. **Phase 3**: Build convert_advanced.go with UI logic
4. **Phase 4**: Update main.go to use new modules
5. **Phase 5**: Test and iterate on modular interface

## 🎯 Success Metrics

✅ main.go reduced by 2000+ lines  
✅ UI logic properly separated and testable  
✅ Clean module boundaries with no circular deps  
✅ Maintain existing functionality and user experience  
✅ Foundation for future UI improvements in other modules  

This modularization will make the codebase much more maintainable and prepare us for advanced features in dev25.