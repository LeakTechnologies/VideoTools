package i18n

// Strings holds every user-visible string in the application.
// Each field is a translatable key. The en-CA instance is the source
// of truth; all other languages fall back to en-CA for any empty field.
//
// When adding a new UI string:
//   1. Add a field here with a descriptive name.
//   2. Set the English value in en_ca.go.
//   3. Add translations to other language files as available.
type Strings struct {
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle string

	// ── Main Menu ────────────────────────────────────────────────────────
	MenuQueue     string
	MenuBenchmark string
	MenuResults   string
	MenuAbout     string

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
	CategoryConvert   string
	CategoryInspect   string
	CategoryDisc      string
	CategoryPlayback  string

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate   string
	ActionCancel     string
	ActionSave       string
	ActionLoad       string
	ActionReset      string
	ActionBrowse     string
	ActionOpen       string
	ActionClose      string
	ActionBack       string
	ActionAdd        string
	ActionRemove     string
	ActionClear      string
	ActionClearAll   string
	ActionInstall    string
	ActionUninstall  string
	ActionStart      string
	ActionStop       string
	ActionPause      string
	ActionResume     string
	ActionDelete     string
	ActionRefresh    string
	ActionApply      string
	ActionConfirm    string
	ActionEdit       string
	ActionCopy       string
	ActionExport     string
	ActionImport     string

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput       string
	LabelOutput      string
	LabelSource      string
	LabelDestination string
	LabelFormat      string
	LabelQuality     string
	LabelResolution  string
	LabelBitrate     string
	LabelFrameRate   string
	LabelCodec       string
	LabelAudio       string
	LabelVideo       string
	LabelSubtitles   string
	LabelDuration    string
	LabelSize        string
	LabelProgress    string
	LabelStatus      string
	LabelLanguage    string
	LabelVersion     string
	LabelLicense     string

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady      string
	StatusProcessing string
	StatusComplete   string
	StatusFailed     string
	StatusCancelled  string
	StatusPending    string
	StatusRunning    string
	StatusUpToDate   string

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle           string
	SettingsTabGeneral      string
	SettingsTabDependencies string
	SettingsTabUpdates      string
	SettingsTabAbout        string
	SettingsLanguage        string
	SettingsLanguageScript  string // Inuktitut script sub-option label
	SettingsScriptSyllabics string // "Traditional Syllabics"
	SettingsScriptLatin     string // "Latin"
	SettingsTheme           string
	SettingsOutputFolder    string

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton      string
	UpdateUpToDate         string
	UpdateAvailable        string
	UpdateInstall          string
	UpdatePatchesAvailable string
	UpdateInstallPatches   string
	UpdateHashMismatch     string // "Hash mismatch detected:"
	UpdateHashCurrent      string // "Current:"
	UpdateHashLatest       string // "Latest:"

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle       string
	QueueEmpty       string
	QueueInProgress  string
	QueueCompleted   string
	QueueFailed      string
	QueueJobRunning  string
	QueueJobPending  string

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle     string
	HistoryNoEntries string

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt   string
	ConvertOutputFormat string
	ConvertHardwareAccel string

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow    string
	ThumbnailContactSheet   string
	ThumbnailColumns        string
	ThumbnailRows           string
	ThumbnailOutputFolder   string

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle       string
	AboutDescription string
	AboutLicense     string
	AboutSupport     string

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound     string
	ErrNoOutputFolder   string
	ErrFFmpegMissing    string
	ErrProcessFailed    string
	ErrConfigLoad       string
	ErrConfigSave       string
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
