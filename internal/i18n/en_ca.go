package i18n

// enCA is the English (Canada) source of truth.
// Every field must be populated — this is the fallback for all other languages.
var enCA = Strings{
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle: "VideoTools",

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue:     "Queue",
	MenuBenchmark: "Benchmark",
	MenuResults:   "Results",
	MenuAbout:     "About / Support",

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "Convert",
	ModuleMerge:       "Merge",
	ModuleTrim:        "Trim",
	ModuleFilters:     "Filters",
	ModuleUpscale:     "Upscale",
	ModuleEnhancement: "Enhancement",
	ModuleAudio:       "Audio",
	ModuleAuthor:      "Author",
	ModuleRip:         "Rip",
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "Subtitles",
	ModuleThumbnail:   "Thumbnail",
	ModuleCompare:     "Compare",
	ModuleInspect:     "Inspect",
	ModulePlayer:      "Player",
	ModuleSettings:    "Settings",

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:  "Convert",
	CategoryInspect:  "Inspect",
	CategoryDisc:     "Disc",
	CategoryPlayback: "Playback",

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:  "Generate",
	ActionCancel:    "Cancel",
	ActionSave:      "Save",
	ActionLoad:      "Load",
	ActionReset:     "Reset",
	ActionBrowse:    "Browse",
	ActionOpen:      "Open",
	ActionClose:     "Close",
	ActionBack:      "Back",
	ActionAdd:       "Add",
	ActionRemove:    "Remove",
	ActionClear:     "Clear",
	ActionClearAll:  "Clear All",
	ActionInstall:   "Install",
	ActionUninstall: "Uninstall",
	ActionStart:     "Start",
	ActionStop:      "Stop",
	ActionPause:     "Pause",
	ActionResume:    "Resume",
	ActionDelete:    "Delete",
	ActionRefresh:   "Refresh",
	ActionApply:     "Apply",
	ActionConfirm:   "Confirm",
	ActionEdit:      "Edit",
	ActionCopy:      "Copy",
	ActionExport:    "Export",
	ActionImport:    "Import",

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput:       "Input",
	LabelOutput:      "Output",
	LabelSource:      "Source",
	LabelDestination: "Destination",
	LabelFormat:      "Format",
	LabelQuality:     "Quality",
	LabelResolution:  "Resolution",
	LabelBitrate:     "Bitrate",
	LabelFrameRate:   "Frame Rate",
	LabelCodec:       "Codec",
	LabelAudio:       "Audio",
	LabelVideo:       "Video",
	LabelSubtitles:   "Subtitles",
	LabelDuration:    "Duration",
	LabelSize:        "Size",
	LabelProgress:    "Progress",
	LabelStatus:      "Status",
	LabelLanguage:    "Language",
	LabelVersion:     "Version",
	LabelLicense:     "License",

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:      "Ready",
	StatusProcessing: "Processing...",
	StatusComplete:   "Complete",
	StatusFailed:     "Failed",
	StatusCancelled:  "Cancelled",
	StatusPending:    "Pending",
	StatusRunning:    "Running...",
	StatusUpToDate:   "Up to date",

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:           "Settings",
	SettingsTabGeneral:      "General",
	SettingsTabDependencies: "Dependencies",
	SettingsTabUpdates:      "Updates",
	SettingsTabAbout:        "About",
	SettingsLanguage:        "Language",
	SettingsLanguageScript:  "Script",
	SettingsScriptSyllabics: "Traditional Syllabics",
	SettingsScriptLatin:     "Latin",
	SettingsTheme:           "Theme",
	SettingsOutputFolder:    "Output Folder",

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:      "Check for Updates",
	UpdateUpToDate:         "You are running the latest version (%s).",
	UpdateAvailable:        "A new version is available: %s\n\nYou are currently on %s.\n\nClick 'Install Update' to download and restart automatically.",
	UpdateInstall:          "Install Update",
	UpdatePatchesAvailable: "You are on %s (the latest release tag).\n\nHash mismatch detected:\n  Current: %s\n  Latest:  %s\n\nClick 'Install Patches' to download the latest build and restart automatically.",
	UpdateInstallPatches:   "Install Patches",
	UpdateHashMismatch:     "Hash mismatch detected:",
	UpdateHashCurrent:      "Current:",
	UpdateHashLatest:       "Latest:",

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle:      "Queue",
	QueueEmpty:      "No jobs in queue",
	QueueInProgress: "In Progress",
	QueueCompleted:  "Completed",
	QueueFailed:     "Failed",
	QueueJobRunning: "Running...",
	QueueJobPending: "Pending",

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle:     "HISTORY",
	HistoryNoEntries: "No entries",

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt:    "Drop a video file here",
	ConvertOutputFormat:  "Output Format",
	ConvertHardwareAccel: "Hardware Acceleration",

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow:        "GENERATE NOW",
	ThumbnailContactSheet:       "Contact Sheet",
	ThumbnailColumns:            "Columns",
	ThumbnailRows:               "Rows",
	ThumbnailOutputFolder:       "Output Folder",
	ThumbnailIndividual:         "Individual Thumbnails",
	ThumbnailLoadVideo:          "Load Video",
	ThumbnailNoFile:             "No file loaded",
	ThumbnailFileLoaded:         "File: video loaded",
	ThumbnailInstructions:       "Generate thumbnails from a video file. Load a video and configure settings.",
	ThumbnailContactSheetToggle: "Generate Contact Sheet (single image)",
	ThumbnailShowTimestamps:     "Show timestamps on thumbnails",
	ThumbnailContactSheetGrid:   "Contact Sheet Grid",
	ThumbnailSize:               "Thumbnail Size:",
	ThumbnailCountFmt:           "Thumbnail Count: %d",
	ThumbnailWidthFmt:           "Thumbnail Width: %d px",
	ThumbnailColumnsFmt:         "Columns: %d",
	ThumbnailRowsFmt:            "Rows: %d",
	ThumbnailTotalFmt:           "Total thumbnails: %d",
	ThumbnailAddToQueue:         "Add to Queue",
	ThumbnailAddAllToQueue:      "Add All to Queue",
	ThumbnailLoadedVideos:       "Loaded Videos:",
	ThumbnailVideoFmt:           "Video %d",
	ThumbnailNoVideoTitle:       "No Video",
	ThumbnailNoVideoMsg:         "Please load a video file first.",
	ThumbnailStartedTitle:       "Thumbnails",
	ThumbnailStartedMsg:         "Thumbnail generation started! View progress in Job Queue.",
	ThumbnailJobQueuedTitle:     "Queue",
	ThumbnailJobQueuedMsg:       "Thumbnail job added to queue!",
	ThumbnailNoVideosTitle:      "No Videos",
	ThumbnailNoVideosMsg:        "Load videos first to add to queue.",
	ThumbnailJobsQueuedFmt:      "Queued %d thumbnail jobs.",

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle:       "About VideoTools",
	AboutDescription: "A native video processing toolkit built for speed and simplicity.",
	AboutLicense:     "License",
	AboutSupport:     "Support",
	AboutLogsFolder:  "Logs Folder",
	AboutScanForDocs: "Scan for docs",
	AboutFeedback:    "Feedback: use the Logs button on the main menu to view logs; send issues with attached logs.",
	AboutClose:       "Close",

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound:   "File not found: %s",
	ErrNoOutputFolder: "No output folder selected.",
	ErrFFmpegMissing:  "FFmpeg is not installed or could not be found.",
	ErrProcessFailed:  "Process failed: %s",
	ErrConfigLoad:     "Failed to load config: %s",
	ErrConfigSave:     "Failed to save config: %s",
}

func init() {
	register("en-CA", enCA)
}
