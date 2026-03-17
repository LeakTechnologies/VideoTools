package i18n

// Strings holds every user-visible string in the application.
// Each field is a translatable key. The en-CA instance is the source
// of truth; all other languages fall back to en-CA for any empty field.
//
// When adding a new UI string:
//  1. Add a field here with a descriptive name.
//  2. Set the English value in en_ca.go.
//  3. Add translations to other language files as available.
type Strings struct {
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle string

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue     string
	MenuBenchmark string
	MenuResults   string
	MenuAbout     string
	MenuLogs      string

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert     string
	ModuleMerge       string
	ModuleTrim        string
	ModuleFilters     string
	ModuleUpscale     string
	ModuleEnhancement string
	ModuleAudio       string
	ModuleAuthor      string
	ModuleRip         string
	ModuleBluRay      string
	ModuleSubtitles   string
	ModuleThumbnail   string
	ModuleCompare     string
	ModuleInspect     string
	ModulePlayer      string
	ModuleSettings    string

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert  string
	CategoryInspect  string
	CategoryDisc     string
	CategoryPlayback string

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate     string
	ActionCancel       string
	ActionSave         string
	ActionLoad         string
	ActionReset        string
	ActionBrowse       string
	ActionOpen         string
	ActionClose        string
	ActionBack         string
	ActionAdd          string
	ActionRemove       string
	ActionClear        string
	ActionClearAll     string
	ActionInstall      string
	ActionUninstall    string
	ActionStart        string
	ActionStop         string
	ActionPause        string
	ActionResume       string
	ActionDelete       string
	ActionRefresh      string
	ActionApply        string
	ActionConfirm      string
	ActionEdit         string
	ActionCopy         string
	ActionExport       string
	ActionImport       string
	ActionViewQueue    string // "View Queue"
	ActionAddToQueue   string // "Add to Queue"
	ActionLoadVideo    string // "Load Video"
	ActionClearVideo   string // "Clear Video"
	ActionCopyLog      string // "Copy Log"
	ActionCopyMetadata string // "Copy Metadata"
	ActionLoadConfig   string // "Load Config"
	ActionSaveConfig   string // "Save Config"

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput         string
	LabelOutput        string
	LabelSource        string
	LabelDestination   string
	LabelFormat        string
	LabelQuality       string
	LabelResolution    string
	LabelBitrate       string
	LabelFrameRate     string
	LabelCodec         string
	LabelAudio         string
	LabelVideo         string
	LabelSubtitles     string
	LabelDuration      string
	LabelSize          string
	LabelProgress      string
	LabelStatus        string
	LabelLanguage      string
	LabelVersion       string
	LabelLicense       string
	LabelNoFile        string // "No file loaded"
	LabelNoVideoLoaded string // "No video loaded"
	LabelFileFmt       string // "File: %s"

	// ── Inspect Module ────────────────────────────────────────────────────────
	InspectInstructions       string // "Load a video to inspect..."
	InspectLoadingPreview     string // "Loading preview..."
	InspectNoPreviewAvailable string // "No preview available"

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady        string
	StatusProcessing   string
	StatusComplete     string
	StatusFailed       string
	StatusCancelled    string
	StatusPending      string
	StatusRunning      string
	StatusUpToDate     string
	StatusNoActiveJobs string // "○ No active jobs"

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle                string
	SettingsTabGeneral           string
	SettingsTabDependencies      string
	SettingsTabUpdates           string
	SettingsTabAbout             string
	SettingsTabBenchmark         string
	SettingsTabPreferences       string
	SettingsLanguage             string
	SettingsLanguageScript       string // Inuktitut script sub-option label
	SettingsScriptSyllabics      string // "Traditional Syllabics"
	SettingsScriptLatin          string // "Latin"
	SettingsTheme                string
	SettingsOutputFolder         string
	SettingsAppPreferences       string // "Application Preferences"
	SettingsMasterSettings       string // "Master Settings"
	SettingsHardwareAccel        string // "Hardware Acceleration (Global)"
	SettingsDetect               string // "Detect" (hw accel detect button)
	SettingsUseAuto              string // "Use Auto"
	SettingsDetectedFmt          string // "Detected: %s"
	SettingsModuleVisibility     string // "Module Visibility"
	SettingsModuleVisibilityHint string // "Module visibility applies on the main menu."
	SettingsShowUpscale          string // "Show Upscale module"
	SettingsShowDisc             string // "Show Disc category (Author & Rip)"
	SettingsFFmpegMissing        string // "Core dependency FFmpeg is missing..."
	SettingsInstallFFmpeg        string // "Install FFmpeg Now"
	SettingsOpenSettings         string // "Open Settings"
	SettingsContinueLimited      string // "Continue Limited Mode"
	SettingsOpenReleases         string // "Open Releases Page"
	SettingsUpdatesInfo          string // "Check for updates and manage app updates."
	SettingsUpdatesAutoInfo      string // "Automatic updates info"

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton       string
	UpdateUpToDate          string
	UpdateAvailable         string
	UpdateInstall           string
	UpdatePatchesAvailable  string
	UpdateInstallPatches    string
	UpdateHashMismatch      string // "Hash mismatch detected:"
	UpdateHashCurrent       string // "Current:"
	UpdateHashLatest        string // "Latest:"
	UpdateCurrentVersion    string // "Current Version:"
	UpdateVersionHash       string // "Version Hash:"
	UpdateChecking          string // "Checking for updates..."
	UpdateError             string // "Error:"
	UpdateAvailableFmt      string // "Update available: %s (%s)"
	UpdateNewBuildAvailable string // "New build available: %s (%s)"
	UpdateUpToDateFmt       string // "Up to date (latest: %s, %s)"
	UpdateAutoCheck         string // "Auto-check:"
	UpdateDisabled          string // "Disabled"
	UpdateEveryHour         string // "Every hour"
	UpdateEvery2Hours       string // "Every 2 hours"
	UpdateEvery3Hours       string // "Every 3 hours"
	UpdateEvery4Hours       string // "Every 4 hours"
	UpdateEvery6Hours       string // "Every 6 hours"
	UpdateEvery12Hours      string // "Every 12 hours"
	UpdateDaily             string // "Daily"
	UpdateSemiWeekly        string // "Semi-weekly (every 3 days)"
	UpdateWeekly            string // "Weekly"
	UpdateBiWeekly          string // "Bi-weekly (every 2 weeks)"
	UpdateMonthly           string // "Monthly"
	UpdateBiMonthly         string // "Bi-monthly (every 2 months)"
	UpdateAutoCheckDesc     string // "Checks for updates automatically based on the schedule above."

	// ── Dependencies ─────────────────────────────────────────────────────
	DependenciesTitle        string // "System Dependencies"
	DependenciesDesc         string // "Manage VideoTools dependencies..."
	DependenciesInstalled    string // "Installed"
	DependenciesNotInstalled string // "Not Installed"
	DependenciesCore         string // "Core dependency"
	DependenciesInstall      string // "Install"
	DependenciesUninstall    string // "Uninstall"
	DependenciesRefresh      string // "Refresh Status"
	DependenciesRequiredBy   string // "Required by:"
	DependenciesInstallCmd   string // "Install: %s" (shows install command)

	// ── Benchmark ────────────────────────────────────────────────────────
	BenchmarkTitle     string // "Hardware Benchmark"
	BenchmarkDesc      string // "Test your system's video encoding performance..."
	BenchmarkRunButton string // "Run Hardware Benchmark"
	BenchmarkRecent    string // "Recent Benchmarks"

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle      string
	QueueEmpty      string
	QueueInProgress string
	QueueCompleted  string
	QueueFailed     string
	QueueJobRunning string
	QueueJobPending string

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle     string
	HistoryNoEntries string

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt    string
	ConvertOutputFormat  string
	ConvertHardwareAccel string

	// ── Compare ──────────────────────────────────────────────────────────────────
	CompareInstructions    string
	CompareFullscreen      string
	CompareCopyReport      string
	CompareHidePlayer      string
	CompareShowPlayer      string
	CompareLoadFile1       string
	CompareLoadFile2       string
	CompareFile1NotLoaded  string
	CompareFile2NotLoaded  string
	CompareFile1Fmt        string // "File 1: %s"
	CompareFile2Fmt        string // "File 2: %s"
	CompareFile1Info       string
	CompareFile2Info       string
	CompareBackToView      string
	ComparePlayBoth        string
	ComparePauseBoth       string
	CompareSyncTitle       string
	CompareSyncMsg         string
	CompareSyncMsgShort    string
	CompareSideInfo        string
	CompareCopied          string
	CompareCopiedMsg       string
	CompareCopiedFileMsg   string
	CompareNoVideosTitle   string
	CompareNoVideosFSMsg   string
	CompareNoVideosCopyMsg string

	// ── Player ───────────────────────────────────────────────────────────────────
	PlayerInstructions string

	// ── Rip ──────────────────────────────────────────────────────────────────────
	RipDropPrompt       string
	RipOutputPath       string
	RipSource           string
	RipFormatLabel      string
	RipLog              string
	RipAddToQueue       string
	RipNow              string
	RipClearISO         string
	RipJobQueuedTitle   string
	RipJobQueuedMsg     string
	RipStartTitle       string
	RipStartMsg         string
	RipNoConfigTitle    string
	RipNoConfigMsg      string
	RipConfigSavedTitle string
	RipConfigSavedFmt   string // "Saved to %s"
	RipErrNoSource      string

	// ── Upscale ──────────────────────────────────────────────────────────────────
	UpscaleNow             string
	UpscaleStartedTitle    string
	UpscaleStartedFmt      string // "Upscaling to %s.\nCheck the queue for progress."
	UpscaleAddedTitle      string
	UpscaleAddedFmt        string // "Upscale job added.\nTarget: %s, Method: %s"
	UpscaleSourceFmt       string // "Source: %dx%d"
	UpscaleSourceNA        string
	UpscaleMethodFmt       string // "Method: %s"
	UpscaleTargetFmt       string // "Target: %s"
	UpscaleBlurFmt         string // "Blur Strength: %.2f"
	UpscaleEnableBlur      string
	UpscaleAdjustFilters   string
	UpscaleAdjustFmt       string // "Adjustment: %.2fx"
	UpscaleDenoiseFmt      string // "Denoise: %.2f"
	UpscaleDenoiseAvail    string
	UpscaleDenoiseUnavail  string
	UpscaleFrameRateFmt    string // "Frame Rate: %s"
	UpscaleMotionInterp    string
	UpscaleMotionHint      string
	UpscaleAIEnabled       string
	UpscaleAIDetected      string
	UpscaleAINotDetected   string
	UpscaleAIPython        string
	UpscaleAIFallback      string
	UpscaleAINote          string
	UpscaleAIAdvanced      string
	UpscaleFaceEnhance     string
	UpscaleTTACheck        string
	UpscaleBitrateHint     string
	UpscaleClassicDesc     string
	UpscaleOptionalBlur    string
	UpscaleFilterIntHint   string
	UpscaleVideoBox        string
	UpscaleTargetResBox    string
	UpscaleEncodingBox     string
	UpscaleScalingBox      string
	UpscaleFrameRateBox    string
	UpscaleFilterIntBox    string
	UpscaleAIBox           string
	UpscaleResLabel        string
	UpscaleVideoCodecLabel string
	UpscaleEncoderLabel    string
	UpscaleQualityLabel    string
	UpscaleBitrateLabel    string
	UpscaleTargetFPSLabel  string
	UpscaleAIModelLabel    string
	UpscaleAIPresetLabel   string
	UpscaleAIScaleLabel    string
	UpscaleAITileLabel     string
	UpscaleAIOutputLabel   string
	UpscaleGPULabel        string
	UpscaleThreadsLabel    string
	UpscaleFilterIntLabel  string
	UpscaleScalingLabel    string

	// ── Frame Interpolation (RIFE) ────────────────────────────────────────────
	RIFEBoxTitle        string // "Frame Interpolation (RIFE)"
	RIFEDetected        string // "rife-ncnn-vulkan detected"
	RIFENotDetected     string // "rife-ncnn-vulkan not detected..."
	RIFEEnabled         string // "Enable RIFE Frame Interpolation"
	RIFEMultiplierLabel string // "Multiplier:"
	RIFEModelLabel      string // "Model:"
	RIFEEstFPSFmt       string // "Estimated output: %.0f fps"
	RIFENote            string // descriptive note

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow        string
	ThumbnailContactSheet       string
	ThumbnailColumns            string
	ThumbnailRows               string
	ThumbnailOutputFolder       string
	ThumbnailIndividual         string // "Individual Thumbnails"
	ThumbnailLoadVideo          string // "Load Video"
	ThumbnailNoFile             string // "No file loaded"
	ThumbnailFileLoaded         string // "File: video loaded"
	ThumbnailInstructions       string // instruction text below header
	ThumbnailContactSheetToggle string // "Generate Contact Sheet (single image)"
	ThumbnailShowTimestamps     string // "Show timestamps on thumbnails"
	ThumbnailContactSheetGrid   string // "Contact Sheet Grid" (box title)
	ThumbnailSize               string // "Thumbnail Size:"
	ThumbnailCountFmt           string // "Thumbnail Count: %d"
	ThumbnailWidthFmt           string // "Thumbnail Width: %d px"
	ThumbnailColumnsFmt         string // "Columns: %d"
	ThumbnailRowsFmt            string // "Rows: %d"
	ThumbnailTotalFmt           string // "Total thumbnails: %d"
	ThumbnailAddToQueue         string // "Add to Queue"
	ThumbnailAddAllToQueue      string // "Add All to Queue"
	ThumbnailLoadedVideos       string // "Loaded Videos:"
	ThumbnailVideoFmt           string // "Video %d"
	ThumbnailNoVideoTitle       string // dialog: "No Video"
	ThumbnailNoVideoMsg         string // dialog: "Please load a video file first."
	ThumbnailStartedTitle       string // dialog: "Thumbnails"
	ThumbnailStartedMsg         string // dialog: generation started message
	ThumbnailJobQueuedTitle     string // dialog: "Queue"
	ThumbnailJobQueuedMsg       string // dialog: "Thumbnail job added to queue!"
	ThumbnailNoVideosTitle      string // dialog: "No Videos"
	ThumbnailNoVideosMsg        string // dialog: "Load videos first to add to queue."
	ThumbnailJobsQueuedFmt      string // dialog: "Queued %d thumbnail jobs."

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle       string
	AboutDescription string
	AboutLicense     string
	AboutSupport     string
	AboutLogsFolder  string
	AboutScanForDocs string
	AboutFeedback    string
	AboutClose       string

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound   string
	ErrNoOutputFolder string
	ErrFFmpegMissing  string
	ErrProcessFailed  string
	ErrConfigLoad     string
	ErrConfigSave     string
}

// CompletionPercent returns the fraction of fields populated in s
// compared to the en-CA source of truth. Used for README translation table.
func CompletionPercent(s, reference Strings) float64 {
	total := countFields(reference)
	if total == 0 {
		return 0
	}
	filled := countFields(s)
	return float64(filled) / float64(total) * 100
}
