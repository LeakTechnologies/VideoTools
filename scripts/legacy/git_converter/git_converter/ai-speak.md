# 🤖 AI-Speak: Development Collaboration Guide

**🎯 PURPOSE:** This file serves as the central collaboration hub between the AI assistant and development team for the lt-convert.sh cross-platform video converter project.

**👥 Partners:** 
- AI Assistant (development partner)
- Human Developer & Tester
- User (Windows testing & feedback)

**📅 Project:** lt-convert.sh - Cross-platform Video Converter  
**🔄 Last Updated:** 2025-12-14

---

## 🧭 **Project Overview (VideoTools & lt-convert)**
- VideoTools is a professional GUI suite focused on DVD-compliant output (MPEG-2 NTSC/PAL), AC-3 audio, and DVDStyler/PS2 compatibility.
- Core app: Go-based modular architecture (convert/queue/ui/player), batch queue with pause/resume/history, smart framerate/audio conversion, aspect handling, and validation.
- Scripts: `lt-convert.sh` provides cross-platform hardware-accelerated AV1/HEVC conversions with modular bash components (hardware/codec/quality/filters/encode).
- Key docs: `README.md` (overview), `INSTALLATION.md`, `DVD_USER_GUIDE.md`, `DVD_IMPLEMENTATION_SUMMARY.md`, `QUEUE_SYSTEM_GUIDE.md`, `INTEGRATION_GUIDE.md`, `BUILD_AND_RUN.md`.
- Current Linux display focus is Wayland-first while retaining X11 support; VT_Player is moving to GTK and not yet functional.

---

## 📖 **HOW TO USE THIS FILE (For AI Assistant)**

### **🔍 READING THIS FILE:**
1. **Start at the top** - understand project context
2. **Check Current Status** - see what's completed vs pending
3. **Review Priority Tasks** - focus on high-priority items first
4. **Read Conversation Log** - understand past issues and solutions
5. **Check Testing Status** - see what needs validation

### **✏️ UPDATING THIS FILE:**
1. **IMMEDIATELY UPDATE** when making any changes or suggestions
2. **USE CLEAR MARKERS** like [x] for completed, [ ] for pending
3. **ADD TO CONVERSATION LOG** with date and issue description
4. **UPDATE TESTING CHECKLISTS** for new features
5. **PROPOSE IMPROVEMENTS** in the suggested improvements section

### **🎯 YOUR ROLE:**
- **Cross-platform guardian** - ensure Windows/Linux compatibility
- **Proactive improver** - suggest optimizations and new features
- **Documentation keeper** - maintain clear development history
- **Testing coordinator** - track what needs to be validated

---

## 📋 **Current Development Status**

---

## 📋 **Current Development Status**

### ✅ **Completed Features**
- [x] Modular architecture implementation
- [x] Cross-platform compatibility (Windows/Linux/macOS)
- [x] GPU/Encoder selection with auto-detection
- [x] Codec selection (AV1/HEVC)
- [x] Container selection (MKV/MP4)
- [x] Quality modes (Source/CRF/Custom bitrate)
- [x] Color correction presets (8 options)
- [x] Drag-and-drop functionality
- [x] Hardware benchmarking
- [x] Error handling and validation

### 🔄 **Current Issues Being Addressed**
- 🚨 **PERFORMANCE ISSUE:** lt-convert.sh slower (70-100 FPS) vs legacy (200+ FPS)

### 💬 **Conversation Log**

### 2025-12-14 - Performance Regression Investigation
**User:** "The code converts videos slower than previous versions that I created before we went modular. I want to understand why. Please can you look through it code in 'legacy' folder and see if you can understand why FPS on each video convert was in the 200s and above, whereas I only get 70-100 using lt-convert.sh"

**Analysis:** Found key performance differences:

**🔍 LEGACY (Fast - 200+ FPS):**
- Uses **bitrate encoding** (`-b:v 1800k-3500k`)
- **Simple presets** - no complex quality parameters
- **Direct AMD AMF** with minimal overhead
- **Basic scaling** only when needed

**🐌 MODULAR (Slow - 70-100 FPS):**
- Uses **CRF encoding** by default (`-crf 16-20`)
- **Complex quality parameters** (`-preset 6`, `-quality 23-28`)
- **Hardware benchmarking overhead** at startup
- **Modular function call overhead**
- **More complex filter chains**

**🎯 ROOT CAUSE:** CRF encoding is inherently slower than bitrate encoding because it analyzes each frame for optimal quality, while bitrate encoding uses fixed target rates.

**Status:** 🔄 **IMPLEMENTING SOLUTIONS**

### 2025-12-14 - Performance Optimization Implementation
**User Request:** "Add Fast Bitrate mode, but also let's create a menu option for hardware benchmarking at startup, This setting will allow user to select Enter for Hardware Benchmarking first before they can convert. This setting will be presented alongside 'Convert (Skip Benchmark)' which will do as it says, it will just go straight to the conversion options. The hardware benchmarking will need to save and remember users hardware preferences, alongside with encoders and hardware selected which are most appropriate to them and show the user a thumbs up in ascii at the end of the benchmark with the results. Let's be clear about the results in a casual fashion. 'Your AMD Graphics Card was detected', The best encoder for you is (show encoder here)' 'Your Benchmark score was (benchmark score based on a test we could somehow do with ffmpeg). I want to also implement the Simplify Filter Chains solution following the work on the Hardware Encoder. Let's prioritise that, then the Add Fast Bitrate Mode, then Simplify Filter Chains."

**Implementation Plan:**
1. **Priority 1:** Hardware benchmarking menu with caching
2. **Priority 2:** Fast bitrate mode implementation  
3. **Priority 3:** Simplify filter chains

**Status:** 🔄 **STARTING IMPLEMENTATION**

---

## 🎯 **Priority Tasks**

### 🚨 **High Priority**
- [ ] Test on actual Wayland Linux system (primary target) while keeping X11 support
- [ ] Verify drag-and-drop works across different desktop environments
- [ ] Performance testing with various video formats
- [ ] Add progress bars for long conversions
- [ ] Implement batch queue processing

### 📈 **Medium Priority**
- [ ] Add subtitle support (burn-in/soft-subs) once core modules stabilize
- [ ] Create comprehensive test suite

### 🔮 **Future Enhancements**
- [ ] GPU memory usage optimization
- [ ] Multi-threaded processing for multiple files
- [ ] Audio-only/video-only modes
- [ ] Preset system for common configurations
- [ ] Integration with video metadata databases

### 💡 **SUGGESTED IMPROVEMENTS TO CONSIDER:**
- [ ] **Progress bars** using `pv` or custom progress indicators
- [ ] **Batch queue system** for processing multiple files sequentially
- [ ] **Configuration file** support for saving user preferences
- [ ] **Logging system** with timestamps and error details
- [ ] **Automatic backup** of original files before conversion
- [ ] **Smart file naming** with metadata integration
- [ ] **Parallel processing** for multi-core CPU encoding
- [ ] **GPU memory monitoring** and automatic adjustment
- [ ] **Resume capability** for interrupted conversions
- [ ] **Integration with system notifications** for completion alerts

---

## 💬 **Conversation Log**

### 2025-12-14 - Line endings + sync
**AI Assistant:** Added `.gitattributes` to enforce LF endings for all shell scripts, preventing Windows CRLF issues. Pulled/merged the latest `lt-convert.sh` changes and set the script executable bit. Repo is clean and up to date on `master`.

### 2025-12-14 - Initial Cross-Platform Fixes
**AI Assistant:** "A developer using Wayland Linux reported that drag and drop onto 'lt-convert.sh' did not work, and it also did not detect video files within the current folder."

**Response:** Implemented comprehensive cross-platform fixes:
- Fixed file detection using explicit scanning instead of `nullglob`
- Added `lshw` hardware detection for Linux
- Implemented cross-platform timeout handling
- Improved path resolution for different shells
- Added more video format support

**Status:** ✅ **RESOLVED** - Awaiting testing feedback

---

## 🧪 **Testing Checklist**

### **Windows Testing**
- [ ] Git Bash compatibility
- [ ] WSL compatibility  
- [ ] PowerShell compatibility
- [ ] Drag-and-drop from File Explorer

### **Linux Testing**
- [ ] Wayland drag-and-drop (GNOME/KDE) **← NEEDS TESTING**
- [ ] X11 drag-and-drop
- [ ] Terminal execution
- [ ] Hardware detection (NVIDIA/AMD/Intel)
- [ ] File permissions handling

### **🧪 LIVE TESTING - Current Changes**
**Feature:** Cross-platform drag-and-drop and file detection
**Status:** ⏳ **AWAITING TEST RESULTS**
**Testers:** 
- User: Windows ✅ (confirmed working)
- Linux Wayland 🔄 **PENDING** (after pulling latest `.gitattributes` + executable bit)

**Expected Results:**
- Drag files onto script → Should process them
- Double-click script → Should find video files in directory
- Hardware detection → Should detect GPU/encoders
- All menus → Should display properly

---

## 📝 **Development Notes**

### **Known Limitations**
- Windows timeout fallback uses manual process killing
- Some Wayland desktop environments may have different drag-and-drop behavior
- Hardware detection varies by Linux distribution
- Wayland is the primary Linux target; X11 support must be preserved
- VT_Player is moving to GTK; current GTK player support is not yet functional

### **Technical Debt**
- Consider replacing bash arrays for better portability
- Evaluate using Python for complex GUI interactions
- Standardize error message formatting

---

## 🔄 **Next Steps**

1. **Immediate:** Get feedback on Wayland testing
2. **Short-term:** Address any compatibility issues found; add progress bars and batch queue processing
3. **Medium-term:** Build comprehensive test suite; plan subtitle support once core is stable
4. **Long-term:** Consider GUI frontend development

---

## 📞 **Contact & Collaboration**

**For AI Assistant:** When working on this project, please:

### **Workflow Rules**
- Commit after every meaningful change; keep commits small and descriptive.
- Update `TODO.md` and `DONE.md` to reflect new work, completed tasks, and testing results.
- Keep this `ai-speak.md` in sync with priorities, logs, and testing status.

### **🔄 When Changes Are Made:**
1. **IMMEDIATELY UPDATE** this file with any suggestions or changes
2. Mark completed tasks with [x]
3. Add new tasks to appropriate priority section
4. Update conversation log with user feedback

### **💡 PROACTIVE IMPROVEMENTS:**
5. **REGULARLY SUGGEST** improvements and optimizations
6. **IDENTIFY OPPORTUNITIES** for:
   - Performance enhancements
   - User experience improvements
   - Code quality and maintainability
   - New feature ideas
7. **PROPOSE SPECIFIC SOLUTIONS** with code examples when possible
8. **RESEARCH BEST PRACTICES** for video conversion and shell scripting

### **🔍 Cross-Platform Compatibility Review:**
9. **ALWAYS CHECK** for compatibility issues across:
   - Windows (Git Bash, WSL, PowerShell)
   - Linux (Wayland, X11, various distros)
10. **IDENTIFY POTENTIAL BREAKING CHANGES** before implementation
11. **SUGGEST ALTERNATIVES** if a feature won't work cross-platform

### **📋 Documentation Requirements:**
12. **UPDATE TESTING CHECKLISTS** for new features
13. **ADD TECHNICAL NOTES** for any platform-specific workarounds
14. **VERIFY** all module dependencies are cross-platform (Windows/Linux only)

### **⚠️ CRITICAL:**
- **NEVER IMPLEMENT** a feature without cross-platform consideration
- **ALWAYS NOTIFY** in this file when platform-specific code is added
- **COORDINATE TESTING** with developer and user feedback before marking complete

**Goal:** Create the most robust, cross-platform video converter available.

---

## 🚨 **Compatibility Alert System**

### **Recent Changes Requiring Review:**
- *None currently - all changes cross-platform verified*

### **Platform-Specific Code Sections:**
- Windows: `wmic` hardware detection, timeout fallback
- Linux: `lspci`/`lshw` hardware detection, `timeout` command

---
