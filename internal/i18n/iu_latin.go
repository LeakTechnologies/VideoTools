package i18n

// iuLatn is the Inuktitut (Latin / Qaliujaaqpait) translation.
// Registered under "iu-Latn" — not shown in the language picker.
// Selected automatically when the user chooses Latin script in Settings
// while Inuktitut is active.  Uses IBM Plex Mono (mono font mode) since
// the romanization uses standard Latin characters.
// Empty fields fall back to enCA automatically.
var iuLatn = Strings{
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle: "VideoTools",

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue:     "Nuatausimaujut",              // Queue
	MenuBenchmark: "Qaujisarniq",                 // Benchmark
	MenuResults:   "Sivulliqpaujut",              // Results
	MenuAbout:     "Mikssannnut / Ikajuqtuiniq",  // About / Support
	MenuLogs:      "Titiraqtausimaujut",          // Logs

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "Asijjiariaqtuq",  // Convert
	ModuleMerge:       "Katitataulugu",   // Merge
	ModuleTrim:        "Iggirrlugu",      // Trim
	ModuleFilters:     "Asikkaarutaujut", // Filters
	ModuleUpscale:     "Pivaalliqtitiniq", // Upscale
	ModuleEnhancement: "Pivaalliqtitiniq", // Enhancement
	ModuleAudio:       "Nipik",           // Audio
	ModuleAuthor:      "Sanarlugu",       // Author/Create
	ModuleRip:         "Nirunnasilugu",   // Rip/Extract
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "Titiqaksait",     // Subtitles
	ModuleThumbnail:   "Akiqalliat",      // Thumbnail
	ModuleCompare:     "Nalunaiqsiluggit", // Compare
	ModuleInspect:     "Qaujisarlugu",    // Inspect
	ModulePlayer:      "Tarvijaksalirijuq", // Player
	ModuleSettings:    "Aaqiksuiniq",     // Settings

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:  "Asijjiariaqtuq", // Converting
	CategoryInspect:  "Qaujisarniq",    // Inspecting
	CategoryDisc:     "Taikkuninga",    // Disc
	CategoryPlayback: "Uqausiiq",       // Playback

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:   "Sanarlugu",                             // Generate
	ActionCancel:     "Atingninga",                            // Cancel
	ActionSave:       "Sapummijugu",                           // Save
	ActionLoad:       "Agaksillugu",                           // Load
	ActionReset:      "Utirviggilugu Sivulliqpaujumut",        // Reset
	ActionBrowse:     "Niruarlugu",                            // Browse
	ActionOpen:       "Ukpiriilugu",                           // Open
	ActionClose:      "Matailugu",                             // Close
	ActionBack:       "Utirlugu",                              // Back
	ActionAdd:        "Ilalliujjilugu",                        // Add
	ActionRemove:     "Aggitittautilugu",                      // Remove
	ActionClear:      "Saqinngiitilugu",                       // Clear
	ActionInstall:    "Inarngirviiggilugu",                    // Install
	ActionUninstall:  "Nuqkarviggilugu",                       // Uninstall
	ActionStart:      "Pigiarlugu",                            // Start
	ActionStop:       "Nuqkarlugu",                            // Stop
	ActionDelete:     "Aggitittautilugu",                      // Delete
	ActionRefresh:    "Nutaarviggilugu",                       // Refresh
	ActionViewQueue:  "Qaujisarlugu Nuatausimaujut",           // View Queue
	ActionAddToQueue: "Nuatausimaujunut Ilalliujjilugu",       // Add to Queue
	ActionLoadVideo:  "Agaksillugu Tarvijaksaq",               // Load Video
	ActionClearVideo: "Saqinngiitilugu Tarvijaksaq",           // Clear Video

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelLanguage:      "Uqausiiq",                          // Language
	LabelVersion:       "Titiraqsimaujuq",                   // Version
	LabelNoFile:        "Ilanginnik Agaksisimanngillaq",      // No file loaded
	LabelNoVideoLoaded: "Tarvijaksaq Agaksisimanngillaq",     // No video loaded

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:        "Pigiaqtauluni",                         // Ready
	StatusProcessing:   "Pilirijautajuq...",                     // Processing
	StatusComplete:     "Isulilauqtuq",                          // Complete
	StatusFailed:       "Ajurunniiqtuq",                         // Failed
	StatusCancelled:    "Atingninga",                            // Cancelled
	StatusPending:      "Attuniq",                               // Pending
	StatusUpToDate:     "Anulaaniqtuq",                          // Up to date
	StatusNoActiveJobs: "○ Pilirijaujannarqtut Nuqkaqtut",       // No active jobs

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:                "Aaqiksuiniq",
	SettingsTabPreferences:       "Aaqiksuiniq",                  // Preferences
	SettingsTabDependencies:      "Pijariatujut",                 // Dependencies
	SettingsTabBenchmark:         "Qaujisarniq",                  // Benchmark
	SettingsTabUpdates:           "Nutaaliurniq",                 // Updates
	SettingsLanguage:             "Uqausiiq",                     // Language
	SettingsLanguageScript:       "Titirauusiit",                 // Script
	SettingsScriptSyllabics:      "Qaniujaaqpait",                // Traditional Syllabics
	SettingsScriptLatin:          "Qaliujaaqpait",                // Latin
	SettingsAppPreferences:       "Atuqnirmut Aaqiksuiniq",       // Application Preferences
	SettingsMasterSettings:       "Aksuruqnarqtujut Aaqiksuiniq", // Master Settings
	SettingsHardwareAccel:        "Sanajaksanik Sukkaivagunnarqtut", // Hardware Acceleration (Global)
	SettingsDetect:               "Nalunaiqsilugu",               // Detect
	SettingsUseAuto:              "Aulatautitaulugu",             // Use Auto
	SettingsDetectedFmt:          "Nalunaiqtaulauqtuq: %s",       // Detected: %s
	SettingsModuleVisibility:     "Takujaujannarqtut",            // Module Visibility
	SettingsModuleVisibilityHint: "Takujaujannarqtut isirviŋmut takujauvaaqtut.", // Module visibility applies on the main menu.
	SettingsShowUpscale:          "Saqquiilugu Pivaalliqtitiniq", // Show Upscale module
	SettingsShowDisc:             "Saqquiilugu Taikkuninga (Sanarlugu & Nirunnasilugu)", // Show Disc category
	SettingsFFmpegMissing:        "Pijariatituuq FFmpeg nalunngissimajanngilaq.\n\nInarngirviiggilugu pilirijutinik atuqtariaqarmat.\n\nInarngirviininga:\n%LOCALAPPDATA%\\VideoTools\\bin",
	SettingsInstallFFmpeg:        "Inarngirviiggilugu FFmpeg Maanna", // Install FFmpeg Now
	SettingsOpenSettings:         "Ukpiriilugu Aaqiksuiniq",      // Open Settings
	SettingsContinueLimited:      "Kajusiilugu Ajurunniiqutauninnga", // Continue Limited Mode
	SettingsOpenReleases:         "Ukpiriilugu Nutausimaujut Makpigaq", // Open Releases Page
	SettingsUpdatesInfo:          "Qaujisarlugu Atuqlullu Nutaat.", // Check for updates and manage app updates.
	SettingsUpdatesAutoInfo:      "Aulatautitaulugu Qaujisarniq Nutaat Aaqiksimaujumit.", // Checks automatically based on schedule.

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:       "Qaujisarlugu Nutaat",                   // Check for Updates
	UpdateInstall:           "Inarngirviiggilugu Nutaaq",             // Install Update
	UpdateInstallPatches:    "Inarngirviiggilugu Matuinnarautajut",   // Install Patches
	UpdateCurrentVersion:    "Maannaujuq Titiraqsimaujuq:",           // Current Version:
	UpdateVersionHash:       "Tukisimatitijuq:",                      // Version Hash:
	UpdateChecking:          "Qaujisarniaqtuq Nutaat...",             // Checking for updates...
	UpdateError:             "Ajurunniilauqtuq:",                     // Error:
	UpdateAvailableFmt:      "⬤ Nutaaq Tunijaujannarqtuq: %s (%s)",  // Update available: %s (%s)
	UpdateNewBuildAvailable: "⬤ Nutaaq Pilirijijautaujuq: %s (%s)",  // New build available: %s (%s)
	UpdateUpToDateFmt:       "✓ Anulaaniqtuq (%s, %s)",              // Up to date (latest: %s, %s)
	UpdateAutoCheck:         "Aulatautitaulugu Qaujisaq:",            // Auto-check:
	UpdateDisabled:          "Attuiniqanngittuq",                     // Disabled
	UpdateEveryHour:         "Ikarramik Taggasami",                   // Every hour
	UpdateEvery2Hours:       "2-nik Ikarranik",                       // Every 2 hours
	UpdateEvery3Hours:       "3-nik Ikarranik",                       // Every 3 hours
	UpdateEvery4Hours:       "4-nik Ikarranik",                       // Every 4 hours
	UpdateEvery6Hours:       "6-nik Ikarranik",                       // Every 6 hours
	UpdateEvery12Hours:      "12-nik Ikarranik",                      // Every 12 hours
	UpdateDaily:             "Ullumi Taggasami",                      // Daily
	UpdateSemiWeekly:        "3-nik Ullunik Taggasami",               // Semi-weekly (every 3 days)
	UpdateWeekly:            "Pinasuarusimi Taggasami",               // Weekly
	UpdateBiWeekly:          "2-nik Pinasuarusinik Taggasami",        // Bi-weekly (every 2 weeks)
	UpdateMonthly:           "Taqqimi Taggasami",                     // Monthly
	UpdateBiMonthly:         "2-nik Taqqinik Taggasami",              // Bi-monthly (every 2 months)

	// ── Dependencies ─────────────────────────────────────────────────────
	DependenciesTitle:        "Pijariatujut Pilirijjutit",          // System Dependencies
	DependenciesDesc:         "Aulattiluggit VideoTools Pijariatujut.", // Manage dependencies
	DependenciesInstalled:    "Inarngirsimaujuq",                   // Installed
	DependenciesNotInstalled: "Inarngirsimanngillaq",               // Not Installed
	DependenciesCore:         "Attarnarqtujuq Pijariatituuq",       // Core dependency
	DependenciesInstall:      "Inarngirviiggilugu",                  // Install
	DependenciesUninstall:    "Nuqkarviggilugu",                    // Uninstall
	DependenciesRefresh:      "Nutaarviggilugu Atuqtaujut",         // Refresh Status
	DependenciesRequiredBy:   "Pijariatitaujut:",                   // Required by:
	DependenciesInstallCmd:   "Inarngirviiggilugu: %s",             // Install: %s

	// ── Benchmark ────────────────────────────────────────────────────────
	BenchmarkTitle:     "Qaujisarniq Sanajaksarmik",                // Hardware Benchmark
	BenchmarkDesc:      "Qaujisarlugu tarvijagalirijjutit pilirinningit atuqviginiaqtat.", // Test encoding performance
	BenchmarkRunButton: "Pigiarlugu Qaujisarniq Sanajaksarmik",     // Run Hardware Benchmark
	BenchmarkRecent:    "Nirunnasiuunik Qaujisalauqtat",            // Recent Benchmarks

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailLoadVideo:   "Agaksillugu Tarvijaksaq", // Load Video
	ThumbnailNoFile:      "Ilanginnik Agaksisimanngillaq", // No file loaded
	ThumbnailGenerateNow: "Pigiautilugu",            // Generate Now

	// ── About ─────────────────────────────────────────────────────────────
	AboutLogsFolder: "Titiraqtausimaujut Nunalingnni", // Logs Folder
	AboutClose:      "Matailugu",                       // Close
}

func init() {
	register("iu-Latn", iuLatn)
}
