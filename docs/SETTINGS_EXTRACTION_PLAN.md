# Settings Module Extraction Plan

## Overview

Extract `settings_module.go` (2366 lines) into `internal/app/modules/settings/` following the established module extraction pattern.

## Status: In Progress (Phase 1 Complete)

### Completed
- [x] Created module directory: `internal/app/modules/settings/`
- [x] Created `view.go` with Options struct and BuildView function
- [x] Updated showSettingsView to use settings.BuildView
- [x] Build passes
- [x] Tab builders remain in settings_module.go via callback pattern

### Current Pattern
The module uses a callback pattern:
- `settings_module.go` contains tab implementations
- Tabs passed as callbacks: `BuildPreferencesTab: func() { return buildPreferencesTab(s) }`
- This keeps complexity manageable while establishing module structure

### Remaining
- [ ] Move tab builders to module package (requires extensive callback wiring)
- [ ] Move helper functions (loadPrefsConfig, etc.)
- [ ] Move bootstrap helpers (Windows dependency installation)
- [ ] Convert settings_module.go to thin shim

### Note
Partial extraction attempts showed that full extraction requires significant
callback wiring. The current callback pattern is a valid abstraction that
maintains code clarity while establishing module boundaries.

## Current State

**Source:** `settings_module.go` (2366 lines)
**Target:** `internal/app/modules/settings/view.go`

**Pattern to follow:** Similar to `inspect_module.go`:
1. Thin shim in root `settings_module.go`
2. UI logic in `internal/app/modules/settings/view.go`
3. Callbacks for appState access

## Challenges

### 1. Heavy appState Dependencies

`settings_module.go` has many functions that directly access `appState`:

```go
func (s *appState) installWindowsFFmpegFromUI(onSuccess func()) {
func (s *appState) installRealESRGANFromUI(onSuccess func()) {
func (s *appState) installRIFEFromUI(onSuccess func()) {
func (s *appState) installWindowsPythonFromUI(onSuccess func(pythonExe string)) {
func (s *appState) maybePromptWindowsDependencyBootstrap() {
```

**Solution:** Pass these as callbacks in Options struct.

### 2. Large Tab Functions

| Tab | Lines | Functions |
|-----|-------|-----------|
| `buildSettingsView` | 1070-1077 | Main container |
| `buildUpdatesTab` | 1079-1124 | Update info |
| `buildDependenciesTab` | 1655-1980 | Dependency management |
| `buildPreferencesTab` | 1981-2366 | Preferences |

**Solution:** Extract each tab builder to separate files.

### 3. Shared State with appcfg

Functions like `loadPrefsConfig`, `savePrefsConfig` use `appcfg.SaveModuleJSON`.

**Solution:** Keep these in root module, pass as callbacks.

### 4. FFmpeg/Dependency Installation

Windows-specific dependency bootstrapping is tightly coupled.

**Solution:** Create `internal/app/modules/settings/bootstrap.go` for Windows-specific logic.

## Extraction Plan

### Phase 1: Create Module Structure

```
internal/app/modules/settings/
├── bootstrap.go       # Dependency installation helpers
├── view.go           # Main view + Options struct
├── updates_tab.go    # Updates tab builder
├── deps_tab.go       # Dependencies tab builder
└── prefs_tab.go      # Preferences tab builder
```

### Phase 2: Define Options Struct

```go
// internal/app/modules/settings/view.go
type Options struct {
    Window       fyne.Window
    ModuleColor  string
    StatsBar    fyne.CanvasObject
    OnBack     func()
    
    // Preferences callbacks
    OnLoadPrefs    func() (*prefsConfig, error)
    OnSavePrefs    func(*prefsConfig) error
    
    // Dependency callbacks
    OnCheckDep     func(depName string) bool
    OnInstallDep   func(depName string, onDone func())
    OnGetDepStatus func(depName string) (Dependency, bool)
    
    // FFmpeg callbacks
    OnCheckFFmpeg  func() (string, bool)
    OnInstallFFmpeg func(onSuccess func())
    OnGetLatestRelease func() (*updateInfo, error)
    
    // Other deps
    OnCheckPython  func() (string, bool)
    OnInstallPython func(onSuccess func(string))
    OnCheckRealESRGAN func() (string, bool)
    OnInstallRealESRGAN func(onSuccess func())
    OnCheckRIFE func() (string, bool)
    OnInstallRIFE func(onSuccess func())
    
    // Stats bar
    OnGetStatsBar func() fyne.CanvasObject
}
```

### Phase 3: Extract Functions

Order of extraction (least to most dependent):

1. **Tab builders** (lowest coupling)
   - `buildUpdatesTab` → `updates_tab.go`
   - `buildPreferencesTab` → `prefs_tab.go`
   - `buildDependenciesTab` → `deps_tab.go`

2. **Helper functions** (medium coupling)
   - `buildSettingsView` → `view.go`
   - `loadPrefsConfig`, `savePrefsConfig` → keep in root

3. **Installation functions** (highest coupling)
   - `installWindowsFFmpegFromUI` → `bootstrap.go`
   - `installRealESRGANFromUI` → `bootstrap.go`
   - `installRIFEFromUI` → `bootstrap.go`
   - `installWindowsPythonFromUI` → `bootstrap.go`
   - `maybePromptWindowsDependencyBootstrap` → `bootstrap.go`

### Phase 4: Root Shim

```go
// settings_module.go
package main

import "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/settings"

func (s *appState) showSettingsView() {
    s.setContent(settings.BuildView(settings.Options{
        Window: s.window,
        ModuleColor: utils.MustHex("#607D8B"),
        StatsBar: s.statsBar,
        OnBack: s.showMainMenu,
        // ... callbacks
    }))
}
```

## Implementation Steps

### Step 1: Create Module Directory
```bash
mkdir -p internal/app/modules/settings
```

### Step 2: Create view.go with Options
Extract `buildSettingsView` and Options struct.

### Step 3: Extract Tab Files
Extract each tab builder to its own file:
- `updates_tab.go`
- `prefs_tab.go`
- `deps_tab.go`

### Step 4: Create bootstrap.go
Move Windows-specific installation functions.

### Step 5: Update Root Shim
Convert `settings_module.go` to thin wrapper.

### Step 6: Build and Test
Verify build passes and app functions correctly.

## Estimated Effort

| Phase | Complexity | Estimated Lines |
|-------|------------|-----------------|
| Structure + Options | Medium | ~200 |
| Tab extractors | Low | ~900 |
| Bootstrap helpers | High | ~700 |
| Root shim | Low | ~50 |
| **Total** | | ~1850 |

## Dependencies to Update

After extraction, these files will need callbacks added:
- `main.go` (for appState wiring)
- `inspect_module.go` (if any shared deps)

## Files Affected

- `settings_module.go` → `internal/app/modules/settings/*.go`
- `main.go` → Add Options callbacks
- `TODO.md` → Mark extraction complete
- `DONE.md` → Document extraction

## Risks

1. **Callback hell** - Many callbacks may make code harder to follow
2. **Breaking changes** - Any appState field changes require updating callbacks
3. **Testing** - UI testing becomes harder with callbacks

## Mitigation

1. Group callbacks logically by feature
2. Use functional options pattern for optional callbacks
3. Create integration tests for each tab

## Success Criteria

- [ ] `settings_module.go` < 100 lines
- [ ] All tab builders in `internal/app/modules/settings/`
- [ ] Build passes
- [ ] App functions correctly
- [ ] No regression in Settings module functionality
