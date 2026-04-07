# Module Improvements Tracking

## Completed Modules (dev40)

### Convert Module
- [x] CatConvert logging category
- [x] Error logging for: probe failures, config save failures, preview capture
- [ ] Tooltips: format selector, quality selector (partial)
- [ ] Documentation: update TRIM_MODULE_DESIGN.md-style doc

### Trim Module
- [x] CatTrim logging category  
- [x] Error logging for: loadVideo failures, config save failures
- [ ] Tooltips: timeline controls, step buttons
- [x] Documentation: TRIM_MODULE_DESIGN.md updated

### Merge Module
- [x] CatMerge logging category
- [x] Error logging for: clip probe failures, config load failures
- [ ] Tooltips: add clips button, reorder buttons
- [ ] Documentation: needs MERGE_MODULE_DESIGN.md

### Filters Module
- [x] CatFilters logging category
- [x] Error logging for: probe failures, encode failures
- [ ] Tooltips: filter sliders
- [ ] Documentation: needs FILTER_MODULE_DESIGN.md

### Audio Module
- [x] CatAudio logging category
- [x] Error logging for: file load failures, track probe failures, config save
- [ ] Tooltips: format selectors, track selection
- [ ] Documentation: needs AUDIO_MODULE_DESIGN.md

### Author Module
- [x] CatAuthor logging category
- [x] Error logging for: config save failures
- [ ] Tooltips: various controls
- [ ] Documentation: check existing docs

### Rip Module
- [x] CatRip logging category
- [x] Error logging for: missing config, missing paths
- [ ] Tooltips: drive selection, format options
- [ ] Documentation: check existing docs

### Burn Module
- [x] CatBurn logging category
- [x] Error logging for: ISO not found, burn failures
- [ ] Tooltips: drive select, speed select, verify option
- [x] Documentation: docs/BURN_MODULE_DESIGN.md exists

---

## Remaining Modules (future)

### Inspect Module
- [ ] Add CatInspect
- [ ] Add error logging for probe failures
- [ ] Tooltips
- [ ] Documentation

### Upscale Module  
- [ ] Add CatUpscale
- [ ] Add error logging for AI failures
- [ ] Tooltips
- [ ] Documentation

### Compare Module
- [ ] Add CatCompare
- [ ] Add error logging for comparison failures
- [ ] Tooltips
- [ ] Documentation

### Thumbnail Module
- [ ] Add CatThumbnail
- [ ] Add error logging for generation failures
- [ ] Tooltips
- [ ] Documentation

### Settings Module
- [ ] Add CatSettings
- [ ] Add error logging for preference failures
- [ ] Tooltips
- [ ] Documentation

### File Manager Module
- [ ] Add CatFileManager
- [ ] Add error logging for file operation failures
- [ ] Tooltips
- [ ] Documentation: docs/FILE_MANAGER_DESIGN.md exists

---

## Additional Improvements to Consider (all modules)

1. **Panic recovery** - Ensure all goroutines have defer/recover (dev39: ✅ done)
2. **Config validation** - Validate inputs before processing
3. **Graceful degradation** - Handle missing dependencies
4. **User feedback** - Better error dialogs with actionable info
5. **Progress reporting** - Consistent progress callbacks
6. **Resource cleanup** - Ensure files/temps are cleaned up
7. **State persistence** - Better recovery from crashes

---

## Cross-Cutting Concerns

1. **i18n completeness** - All user-facing strings localized
2. **Accessibility** - ARIA labels, keyboard navigation
3. **Performance** - Lazy loading, caching
4. **Testing** - Unit tests for critical paths

---

## Session Summary (dev40)

### Logging Categories Added
- CatConvert, CatTrim, CatMerge, CatFilters, CatAudio, CatAuthor, CatRip, CatBurn

### Key Fixes
- Queue right-click crash (PopUpMenu Window nil)
- Main menu Files dropdown crash
- Module visibility fix (burn, filemanager)
- Preview frame capture error logging

### Documentation
- TRIM_MODULE_DESIGN.md updated with player architecture
- MODULE_IMPROVEMENTS_TRACKING.md created
- AGENTS.md updated with InlineVideoPlayer exceptions