# Active Work Coordination

This file tracks what each agent is currently working on to prevent conflicts and coordinate changes.

**Last Updated**: 2026-01-06 19:05 UTC

---

## 🔴 Current Blockers

- **Build Status**: ❌ FAILING (main.go syntax errors introduced by unified player changes)
- **Critical Bug**: BUG-005 - CRF quality settings not showing when CRF mode selected (see BUGS.md)

---

## 👥 Active Work by Agent

### 🤖 Claude (thisagent - Claude Code)
**Status**: ✅ SESSION COMPLETE - Handed off to opencode

**Completed This Session** (2026-01-04):

**Morning Session (10:00):**
- ✅ **Quality Widget Deduplication** (main.go:7075-7128)
  - Converted qualitySelectSimple/Adv to ColoredSelect
  - Registered both with state manager for auto-sync
  - Updated updateQualityOptions to use state manager
  - Eliminated manual synchronization code
- ✅ **Enhancement Module Fixes** (internal/enhancement/)
  - Defined SkinToneAnalysis struct
  - Fixed invalid ContentAnalysis field assignments
  - Removed unused imports and variables
- ✅ **Missing Imports Restored** (main.go:3-48)
  - Added all missing stdlib and third-party imports
  - Build now passes

**Evening Session (18:00-21:00):**
- ✅ **Fixed BUG-001**: Quality Preset visibility (showing in wrong modes)
- ✅ **Fixed BUG-002**: Target File Size visibility (showing in wrong modes)
- ✅ **Fixed BUG-003**: AAC audio codec color too similar to OPUS
- ✅ **Fixed BUG-004**: Audio module drag & drop support added
- ✅ **Created BUGS.md**: Bug tracking system for multi-agent coordination
- ⚠️ **Introduced BUG-005**: Over-corrected visibility logic, CRF settings now don't show

**Files Modified**:
- `main.go` - Quality widgets, visibility fixes, audio drag & drop, imports
- `internal/ui/colors.go` - AAC color changed to cyan
- `internal/enhancement/enhancement_module.go` - SkinToneAnalysis struct
- `internal/enhancement/onnx_model.go` - Unused variable cleanup
- `go.mod`, `go.sum` - Added oto/v3 dependency
- `BUGS.md` - NEW: Bug tracking system
- `WORKING_ON.md` - Updated coordination

**Handoff to opencode**:
1. **CRITICAL**: Fix BUG-005 (CRF quality settings not showing)
2. Complete widget deduplication (4 pairs remaining)
3. Complete ColoredSelect expansion (32 widgets)

---

### 🤖 opencode
**Status**: 🧩 IN PROGRESS - Unified player integration + CRF fixes

**🔥 IMMEDIATE TASKS** (from Claude):
1. **FIX BUG-005** (CRITICAL): CRF quality settings not showing
   - **File**: `main.go:8851-8883` (`updateQualityVisibility()` function)
   - **Problem**: When user selects CRF mode, Quality Preset dropdown doesn't appear
   - **Likely Cause**: Over-corrected visibility logic after fixing BUG-001/BUG-002
   - **Investigation**: Check logic flow in `updateQualityVisibility()` and bitrate mode callback
   - **See**: BUGS.md for full details

2. **Widget Deduplication** (4 remaining pairs):
   - resolutionSelectSimple & resolutionSelect (lines ~8009, 8347)
   - targetAspectSelect & targetAspectSelectSimple (lines ~7397, 8016)
   - encoderPresetSelect & simplePresetSelect (lines ~7531, 7543)
   - bitratePresetSelect & simpleBitrateSelect (lines ~7969, 7982)
   - **Pattern**: Follow quality widget example at main.go:7075-7128

3. **ColoredSelect Expansion** (32 remaining widgets):
   - Resolution, aspect, preset, bitrate, frame rate, etc.
   - Use appropriate color maps (BuildGenericColorMap, BuildQualityColorMap, etc.)

**Uncommitted Work** (Defer to later):
- `internal/queue/edit.go` - Job editing logic (keep for future dev24+)
- `internal/ui/command_editor.go` - Fyne UI dialog
- Enhancement module framework

**Coordination**:
- Jake will be using Codex for UI work this week
- Focus on build stability and widget conversions first

---

## 🤝 Coordination Status

**Current Handoff**: Claude → opencode

**Claude's Handoff** (2026-01-04 21:10):
1. ✅ Quality widget deduplication complete (pattern established)
2. ✅ Enhancement module compilation fixed
3. ✅ Missing imports restored - Build passes
4. ✅ Fixed 4 user-reported bugs (BUG-001 through BUG-004)
5. ⚠️ Introduced BUG-005 (Critical): CRF settings visibility broken
6. ✅ Created BUGS.md tracking system
7. 📋 4 widget pairs still need deduplication
8. 📋 32 widgets need ColoredSelect conversion

**For opencode**:
- **Priority 1**: Fix BUG-005 (Critical - CRF quality settings not showing)
- Priority 2: Complete widget deduplication using established pattern
- Priority 3: ColoredSelect expansion for remaining 32 widgets

**Coordination Notes**:
- User will be using Codex for UI work this week - coordinate visual changes
- Build must pass before UI work can continue

---

### 🤖 Codex (UI focus)
**Status**: 🧱 ACTIVE - UI palette separation + state manager scaffolding

**Working On Now** (2026-01-06):
- ✅ Added `internal/state/convert_manager.go` (state manager scaffolding for Convert)
- ✅ Updated codec palette separation to make format/audio/video colors more distinct

**Next for Codex**:
1. Wire `ConvertManager` into convert UI (quality + bitrate mode visibility)
2. Validate CRF visibility paths once build passes
3. Review any cross-category color clashes in dropdown lists

---

## 📝 Shared Files - Coordinate Before Modifying!

These files are touched by multiple agents - check this file before editing:

- **`main.go`** - High conflict risk!
  - Claude: UI fixes, GPU detection, format selectors
  - opencode: Player integration, enhancement module

- **`internal/queue/queue.go`** - Medium risk
  - Claude: JobType constant fixes
  - opencode: Queue system improvements

- **`internal/sysinfo/sysinfo.go`** - Low risk
  - Claude: GPUVendor() method

---

## ✅ Ready to Commit/Push

**Dev24 started** - Build is passing (dev24)

---

## 🎯 Dev23 Status

**Release Status**: ✅ TAGGED - v0.1.0-dev23

---

## 🚀 Next Steps (Dev24 Planning)

### Immediate Actions
1. ✅ Tag v0.1.0-dev23
2. ✅ Bump version to v0.1.0-dev24
3. ⏭️ Plan dev24 UI cleanup and stability fixes

### Potential Dev24 Focus
- Windows dropdown UI parity
- Additional settings panel alignment
- General UI spacing and word-wrapping cleanup
- Revisit opencode job editing integration (WIP)

---

## 💡 Quick Reference

**To update this file**:
1. Mark what you're starting to work on
2. Update "Currently Modifying" section
3. Move completed items to "Completed This Session"
4. Update blocker status if you fix something
5. Save and commit this file with your changes

**Commit message format**:
- `feat(ui): add colored dropdown menus`
- `fix(build): resolve compilation errors`
- `docs: update WORKING_ON coordination file`
