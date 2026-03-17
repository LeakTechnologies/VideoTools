package i18n

// iu is the Inuktitut translation (Unified Canadian Aboriginal Syllabics).
// The Latin script variant shares these keys — the script toggle in Settings
// switches font rendering only, not the string content.
// Empty fields fall back to enCA automatically.
// Translation progress: see README for current percentage.
var iu = Strings{
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle: "VideoTools",

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue:     "ᓄᐊᑕᐅᓯᒪᔪᑦ",          // Queue
	MenuBenchmark: "ᖃᐅᔨᓴᕐᓂᖅ",            // Benchmark
	MenuResults:   "ᓯᕗᓪᓕᖅᐸᐅᔪᑦ",          // Results
	MenuAbout:     "ᒥᒃᓵᓄᑦ / ᐃᑲᔪᖅᑐᐃᓂᖅ", // About / Support
	MenuLogs:      "ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ",         // Logs

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ",  // Convert
	ModuleMerge:       "ᑲᑎᑕᐅᓗᒍ",      // Merge
	ModuleTrim:        "ᐃᒡᒋᕐᓗᒍ",       // Trim
	ModuleFilters:     "ᐊᓯᒃᑳᕈᑕᐅᔪᑦ",   // Filters
	ModuleUpscale:     "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ", // Upscale
	ModuleEnhancement: "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ", // Enhancement
	ModuleAudio:       "ᓂᐱᒃ",          // Audio
	ModuleAuthor:      "ᓴᓇᓗᒍ",         // Author/Create
	ModuleRip:         "ᓂᕈᓐᓇᓯᓗᒍ",     // Rip/Extract
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "ᑎᑎᖅᑲᒃᓴᑦ",     // Subtitles
	ModuleThumbnail:   "ᐊᑭᖃᓪᓕᐊᑦ",    // Thumbnail
	ModuleCompare:     "ᓇᓗᓇᐃᖅᓯᓗᒋᑦ",  // Compare
	ModuleInspect:     "ᖃᐅᔨᓴᕐᓗᒍ",     // Inspect
	ModulePlayer:      "ᑕᕐᕆᔭᒃᓴᓕᕆᔪᖅ", // Player
	ModuleSettings:    "ᐋᖅᑭᒃᓱᐃᓂᖅ",   // Settings

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:  "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ", // Converting
	CategoryInspect:  "ᖃᐅᔨᓴᕐᓂᖅ",    // Inspecting
	CategoryDisc:     "ᑕᐃᑯᓂᖓ",       // Disc
	CategoryPlayback: "ᐅᖃᐅᓯᖅ",       // Playback

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:   "ᓴᓇᓗᒍ",                    // Generate
	ActionCancel:     "ᐊᑎᖕᓂᖓ",                   // Cancel
	ActionSave:       "ᓴᐳᒻᒥᔪᒍ",                  // Save
	ActionLoad:       "ᐊᒐᒃᓯᓗᒍ",                  // Load
	ActionReset:      "ᐅᑎᕐᕕᒋᓗᒍ ᓯᕗᓪᓕᖅᐸᐅᔪᒧᑦ",  // Reset
	ActionBrowse:     "ᓂᕈᐊᕐᓗᒍ",                  // Browse
	ActionOpen:       "ᐅᒃᐱᕆᓗᒍ",                  // Open
	ActionClose:      "ᒪᑕᐃᓗᒍ",                   // Close
	ActionBack:       "ᐅᑎᕐᓗᒍ",                   // Back
	ActionAdd:        "ᐃᓚᓕᐅᔾᔨᓗᒍ",               // Add
	ActionRemove:     "ᐊᒡᒋᑎᑕᐅᑎᓗᒍ",              // Remove
	ActionClear:      "ᓴᕿᙱᑦᑎᓗᒍ",                // Clear
	ActionInstall:    "ᐃᓇᖏᕐᕕᒋᓗᒍ",               // Install
	ActionUninstall:  "ᓄᖅᑲᕐᕕᒋᓗᒍ",               // Uninstall
	ActionStart:      "ᐱᒋᐊᕐᓗᒍ",                  // Start
	ActionStop:       "ᓄᖅᑲᕐᓗᒍ",                  // Stop
	ActionDelete:     "ᐊᒡᒋᑎᑕᐅᑎᓗᒍ",              // Delete
	ActionRefresh:    "ᓄᑖᕐᕕᒋᓗᒍ",                 // Refresh
	ActionViewQueue:  "ᖃᐅᔨᓴᕐᓗᒍ ᓄᐊᑕᐅᓯᒪᔪᑦ",     // View Queue
	ActionAddToQueue: "ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᔾᔨᓗᒍ",   // Add to Queue
	ActionLoadVideo:  "ᐊᒐᒃᓯᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",        // Load Video
	ActionClearVideo: "ᓴᕿᙱᑦᑎᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",       // Clear Video

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelLanguage:    "ᐅᖃᐅᓯᖅ",   // Language
	LabelVersion:     "ᑎᑎᕋᖅᓯᒪᔪᖅ", // Version
	LabelNoFile:      "ᐃᓚᖏᓐᓂᒃ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ", // No file loaded
	LabelNoVideoLoaded: "ᑕᕐᕆᔭᒐᒃᓴᖅ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ", // No video loaded

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:        "ᓴᐳᒻᒥᔪᖅ",     // Ready
	StatusProcessing:   "ᐱᓕᕆᔭᐅᔪᖅ...", // Processing
	StatusComplete:     "ᐃᓱᓕᓚᐅᖅᑐᖅ",  // Complete
	StatusFailed:       "ᐊᔪᕈᓐᓃᖅᑐᖅ",  // Failed
	StatusCancelled:    "ᐊᑎᖕᓂᖓ",      // Cancelled
	StatusPending:      "ᐊᑦᑐᓂᖅ",      // Pending
	StatusUpToDate:     "ᐊᓄᕌᓂᖅᑐᖅ",   // Up to date
	StatusNoActiveJobs: "○ ᐱᓕᕆᔭᐅᔪᓐᓇᖅᑐᑦ ᓄᖅᑲᖅᑐᑦ", // No active jobs

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:                "ᐋᖅᑭᒃᓱᐃᓂᖅ",                                        // Settings
	SettingsTabPreferences:       "ᐋᖅᑭᒃᓱᐃᓂᖅ",                                        // Preferences
	SettingsTabDependencies:      "ᐱᔭᕆᐊᑐᔪᑦ",                                          // Dependencies
	SettingsTabBenchmark:         "ᖃᐅᔨᓴᕐᓂᖅ",                                          // Benchmark
	SettingsTabUpdates:           "ᓄᑖᓕᐅᕐᓂᖅ",                                          // Updates
	SettingsLanguage:             "ᐅᖃᐅᓯᖅ",                                              // Language
	SettingsLanguageScript:       "ᑎᑎᕋᐅᓯᖅ",                                            // Script
	SettingsScriptSyllabics:      "ᖃᓂᐅᔮᖅᐸᐃᑦ",                                         // Traditional Syllabics
	SettingsScriptLatin:          "ᓚᑎᓐ",                                                 // Latin
	SettingsAppPreferences:       "ᐊᑐᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᓂᖅ",                               // Application Preferences
	SettingsMasterSettings:       "ᐊᒃᓱᕈᕐᓇᖅᑑᔪᑦ ᐋᖅᑭᒃᓱᐃᓂᖅ",                          // Master Settings
	SettingsHardwareAccel:        "ᓴᓇᒐᒃᓴᓂᒃ ᓱᒃᑲᐃᕙᒍᓐᓇᖅᑐᑦ",                          // Hardware Acceleration (Global)
	SettingsDetect:               "ᓇᓗᓇᐃᖅᓯᓗᒍ",                                         // Detect
	SettingsUseAuto:              "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ",                                       // Use Auto
	SettingsDetectedFmt:          "ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ: %s",                               // Detected: %s
	SettingsModuleVisibility:     "ᑕᑯᔭᐅᔪᓐᓇᖅᑐᑦ",                                       // Module Visibility
	SettingsModuleVisibilityHint: "ᑕᑯᔭᐅᔪᓐᓇᖅᑐᑦ ᐃᓯᕐᕕᖕᒧᑦ ᑕᑯᔭᐅᕙᒃᑐᑦ.",                 // Module visibility applies on the main menu.
	SettingsShowUpscale:          "ᓴᖅᑮᓗᒍ ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ",                              // Show Upscale module
	SettingsShowDisc:             "ᓴᖅᑮᓗᒍ ᑕᐃᑯᓂᖓ (ᓴᓇᓗᒍ & ᓂᕈᓐᓇᓯᓗᒍ)",                  // Show Disc category (Author & Rip)
	SettingsFFmpegMissing:        "ᐊᑦᑕᕐᓇᖅᑑᔪᖅ FFmpeg ᐱᔭᕆᐊᑐᔪᖅ ᓂᒡᓕᓯᒪᔪᖅ.\n\nᐃᓇᖏᕐᕕᒋᓗᒍ ᐱᓕᕆᔾᔪᑎᑦ ᐊᑐᕐᓗᒍ.\n\nᐃᓇᖏᕐᕕᒋᓂᖓ:\n%LOCALAPPDATA%\\VideoTools\\bin",
	SettingsInstallFFmpeg:        "ᐃᓇᖏᕐᕕᒋᓗᒍ FFmpeg ᒫᓐᓇ",                            // Install FFmpeg Now
	SettingsOpenSettings:         "ᐅᒃᐱᕆᓗᒍ ᐋᖅᑭᒃᓱᐃᓂᖅ",                               // Open Settings
	SettingsContinueLimited:      "ᑲᔪᓯᓗᒍ ᐊᔪᕈᓐᓃᕈᑕᐅᓂᖓ",                              // Continue Limited Mode
	SettingsOpenReleases:         "ᐅᒃᐱᕆᓗᒍ ᓄᑕᐅᓯᒪᔪᑦ ᒪᒃᐱᒐᖅ",                          // Open Releases Page
	SettingsUpdatesInfo:          "ᖃᐅᔨᓴᕐᓗᒍ ᐊᑐᕐᓗᓪᓗ ᓄᑖᑦ.",                          // Check for updates and manage app updates.
	SettingsUpdatesAutoInfo:      "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ ᖃᐅᔨᓴᕐᓂᖅ ᓄᑖᑦ ᐋᖅᑭᒃᓯᒪᔪᒥᑦ.",            // Checks for updates automatically based on the schedule above.

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:       "ᖃᐅᔨᓴᕐᓗᒍ ᓄᑖᑦ",                     // Check for Updates
	UpdateInstall:           "ᐃᓇᖏᕐᕕᒋᓗᒍ ᓄᑖᖅ",                    // Install Update
	UpdateInstallPatches:    "ᐃᓇᖏᕐᕕᒋᓗᒍ ᒪᑐᐃᓐᓇᐅᔪᑦ",               // Install Patches
	UpdateCurrentVersion:    "ᒫᓐᓇᐅᔪᖅ ᑎᑎᕋᖅᓯᒪᔪᖅ:",                 // Current Version:
	UpdateVersionHash:       "ᑐᑭᓯᒪᑎᑦᑎᔪᖅ:",                        // Version Hash:
	UpdateChecking:          "ᖃᐅᔨᓴᕐᓂᐊᖅᑐᖅ ᓄᑖᑦ...",               // Checking for updates...
	UpdateError:             "ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ:",                      // Error:
	UpdateAvailableFmt:      "⬤ ᓄᑖᖅ ᑐᓂᔭᐅᔪᓐᓇᖅᑐᖅ: %s (%s)",       // Update available: %s (%s)
	UpdateNewBuildAvailable: "⬤ ᓄᑖᖅ ᐱᓕᕆᔾᔪᑕᐅᔪᖅ: %s (%s)",        // New build available: %s (%s)
	UpdateUpToDateFmt:       "✓ ᐊᓄᕌᓂᖅᑐᖅ (%s, %s)",               // Up to date (latest: %s, %s)
	UpdateAutoCheck:         "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ ᖃᐅᔨᓴᖅ:",               // Auto-check:
	UpdateDisabled:          "ᐊᑦᑐᐃᓂᖃᖏᑦᑐᖅ",                       // Disabled
	UpdateEveryHour:         "ᐃᑲᕐᕋᒥᒃ ᑕᒡᒐᓴᒥ",                     // Every hour
	UpdateEvery2Hours:       "2-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                       // Every 2 hours
	UpdateEvery3Hours:       "3-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                       // Every 3 hours
	UpdateEvery4Hours:       "4-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                       // Every 4 hours
	UpdateEvery6Hours:       "6-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                       // Every 6 hours
	UpdateEvery12Hours:      "12-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                      // Every 12 hours
	UpdateDaily:             "ᐅᓪᓗᒥ ᑕᒡᒐᓴᒥ",                       // Daily
	UpdateSemiWeekly:        "3-ᓂᒃ ᐅᓪᓗᓂᒃ ᑕᒡᒐᓴᒥ",                // Semi-weekly (every 3 days)
	UpdateWeekly:            "ᐱᓇᓱᐊᕈᓯᒥ ᑕᒡᒐᓴᒥ",                   // Weekly
	UpdateBiWeekly:          "2-ᓂᒃ ᐱᓇᓱᐊᕈᓯᓂᒃ ᑕᒡᒐᓴᒥ",             // Bi-weekly (every 2 weeks)
	UpdateMonthly:           "ᑕᕐᕿᒥ ᑕᒡᒐᓴᒥ",                       // Monthly
	UpdateBiMonthly:         "2-ᓂᒃ ᑕᕐᕿᓂᒃ ᑕᒡᒐᓴᒥ",                // Bi-monthly (every 2 months)

	// ── Dependencies ─────────────────────────────────────────────────────
	DependenciesTitle:        "ᐱᔭᕆᐊᑐᔪᑦ ᐱᓕᕆᔾᔪᑎᑦ",           // System Dependencies
	DependenciesDesc:         "ᐊᐅᓚᑦᑎᓗᒋᑦ VideoTools ᐱᔭᕆᐊᑐᔪᑦ.", // Manage VideoTools dependencies
	DependenciesInstalled:    "ᐃᓇᖏᕐᓯᒪᔪᖅ",                    // Installed
	DependenciesNotInstalled: "ᐃᓇᖏᕐᓯᒪᙱᓚᖅ",                   // Not Installed
	DependenciesCore:         "ᐊᑦᑕᕐᓇᖅᑑᔪᖅ ᐱᔭᕆᐊᑐᔪᖅ",           // Core dependency
	DependenciesInstall:      "ᐃᓇᖏᕐᕕᒋᓗᒍ",                     // Install
	DependenciesUninstall:    "ᓄᖅᑲᕐᕕᒋᓗᒍ",                     // Uninstall
	DependenciesRefresh:      "ᓄᑖᕐᕕᒋᓗᒍ ᐊᑐᖅᑕᐅᔪᑦ",              // Refresh Status
	DependenciesRequiredBy:   "ᐱᔭᕆᐊᑐᔭᐅᔪᑦ:",                   // Required by:
	DependenciesInstallCmd:   "ᐃᓇᖏᕐᕕᒋᓗᒍ: %s",                 // Install: %s

	// ── Benchmark ────────────────────────────────────────────────────────
	BenchmarkTitle:     "ᖃᐅᔨᓴᕐᓂᖅ ᓴᓇᒐᒃᓴᒥᒃ",                                         // Hardware Benchmark
	BenchmarkDesc:      "ᖃᐅᔨᓴᕐᓗᒍ ᑕᕐᕆᔭᒐᓕᕆᔾᔪᑎᑦ ᐱᓕᕆᓂᖏᑦ ᐊᑐᕐᕕᒋᓂᐊᖅᑕᑦ ᐃᓕᔭᐅᔪᒃᓴᑦ.", // Test video encoding performance
	BenchmarkRunButton: "ᐱᒋᐊᕐᓗᒍ ᖃᐅᔨᓴᕐᓂᖅ ᓴᓇᒐᒃᓴᒥᒃ",                              // Run Hardware Benchmark
	BenchmarkRecent:    "ᓂᕈᓐᓇᓯᔪᓂᒃ ᖃᐅᔨᓴᓚᐅᖅᑕᑦ",                                   // Recent Benchmarks

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailLoadVideo:   "ᐊᒐᒃᓯᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ", // Load Video
	ThumbnailNoFile:      "ᐃᓚᖏᓐᓂᒃ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ", // No file loaded
	ThumbnailGenerateNow: "ᐱᒋᐊᕈᑎᒋᓗᒍ",          // Generate Now

	// ── About ─────────────────────────────────────────────────────────────
	AboutLogsFolder: "ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ ᓄᓇᓕᖕᓂ", // Logs Folder
	AboutClose:      "ᒪᑕᐃᓗᒍ",               // Close
}

func init() {
	register("iu", iu)
}
