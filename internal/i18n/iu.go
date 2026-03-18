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
	MenuQueue:     "ᓄᐊᑕᐅᓯᒪᔪᑦ",         // Queue
	MenuBenchmark: "ᖃᐅᔨᓴᕐᓂᖅ",          // Benchmark
	MenuResults:   "ᓯᕗᓪᓕᖅᐸᐅᔪᑦ",        // Results
	MenuAbout:     "ᒥᒃᓵᓄᑦ / ᐃᑲᔪᖅᑐᐃᓂᖅ", // About / Support
	MenuLogs:      "ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ",       // Logs

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ",  // Convert
	ModuleMerge:       "ᑲᑎᑕᐅᓗᒍ",     // Merge
	ModuleTrim:        "ᐃᒡᒋᕐᓗᒍ",     // Trim
	ModuleFilters:     "ᐊᓯᒃᑳᕈᑕᐅᔪᑦ",  // Filters
	ModuleUpscale:     "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ", // Upscale
	ModuleEnhancement: "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ", // Enhancement
	ModuleAudio:       "ᓂᐱᒃ",        // Audio
	ModuleAuthor:      "ᓴᓇᓗᒍ",       // Author/Create
	ModuleRip:         "ᓂᕈᓐᓇᓯᓗᒍ",    // Rip/Extract
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "ᑎᑎᖅᑲᒃᓴᑦ",    // Subtitles
	ModuleThumbnail:   "ᐊᑭᖃᓪᓕᐊᑦ",    // Thumbnail
	ModuleCompare:     "ᓇᓗᓇᐃᖅᓯᓗᒋᑦ",  // Compare
	ModuleInspect:     "ᖃᐅᔨᓴᕐᓗᒍ",    // Inspect
	ModulePlayer:      "ᑕᕐᕆᔭᒃᓴᓕᕆᔪᖅ", // Player
	ModuleSettings:    "ᐋᖅᑭᒃᓱᐃᓂᖅ",   // Settings

	// ── Dialog Titles ───────────────────────────────────────────────────────
	DialogCompare:     "ᓇᓗᓇᐃᖅᓯᓗᒋᑦ ᕿᓂᖅᓴᖨᖏᓐᓂᕐᓗᒍ", // Compare Videos
	DialogInspect:     "ᖃᐅᔨᓴᕐᓗᒍ ᕿᓂᖅᓴᖨᖏᓐᓂᕐᓗᒍ",   // Inspect Video
	DialogThumbnail:   "ᐊᑭᖃᓪᓕᐊᑦ ᓴᓇᓗᒍ",          // Thumbnail Generation
	DialogSnippet:     "ᐃᓱᒃᑕᐅᔪᑦ",               // Snippet
	DialogInterlacing: "ᓱᓕᒐᓪᓕᓕᐅᔪᑦ ᐊᓯᔾᔨᕆᐊᖅᑐᖅ",   // Interlacing Analysis
	DialogAutoCrop:    "ᐊᐅᑎᒃ ᑕᐅᔪᓐᓇᖅᑐᑦ",         // Auto-Crop
	DialogNoVideo:     "ᕿᓂᖅᓴᖨᖏᓐᓂᖅ ᓵᕝᕙᔪᖅ",       // No Video
	DialogNoFile:      "ᓴᓗᐃᔪᖅ ᓵᕝᕙᔪᖅ",           // No File
	DialogNoConfig:    "ᐃᓄᖁᔮᖃᕐᓂᖅ ᓵᕝᕙᔪᖅ",        // No Config
	DialogConfigSaved: "ᐃᓄᖁᔮᖃᕐᓂᖅ ᓴᐳᒻᒥᔪᒍ",       // Config Saved
	DialogCopied:      "ᐊᒐᒃᓯᔪᒍ",                // Copied
	DialogNoLog:       "ᓴᓗᐃᔪᖅ ᓵᕝᕙᔪᖅ",           // No Log
	DialogQueued:      "ᐃᓚᓕᐅᔾᔨᓗᒍ",              // Queued
	DialogCancelled:   "ᐊᑎᖕᓂᖓ",                 // Cancelled
	DialogBatchAdd:    "ᐃᓚᓕᐅᔾᔨᓗᒍ ᐊᒋᒃᓯᔪᖅ",       // Batch Add
	DialogRecovery:    "ᐃᓱᒃᑕᐅᔪᑦ ᐱᔪᓐᓇᖅᑐᑦ",       // Conversion Recovery
	DialogSnippets:    "ᐃᓱᒃᑕᐅᔪᑦ",               // Snippets
	DialogPreview:     "ᐊᑭᖃᓪᓕᐊᑦ ᓴᓇᓗᒍ",          // Generating Preview
	DialogPlayback:    "ᑕᕐᕆᔭᒃᓴᓕᕆᔪᑦ ᐱᔪᓐᓇᖅᑐᑦ",    // Playback Unavailable

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:     "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ",  // Converting
	CategoryInspect:     "ᖃᐅᔨᓴᕐᓂᖅ",    // Inspecting
	CategoryDisc:        "ᑕᐃᑯᓂᖓ",      // Disc
	CategoryPlayback:    "ᐅᖃᐅᓯᖅ",      // Playback
	CategoryAdvanced:    "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ", // Advanced
	CategoryScreenshots: "ᐊᑭᖃᓪᓕᐊᑦ",    // Screenshots
	CategorySettings:    "ᐋᖅᑭᒃᓱᐃᓂᖅ",   // Settings

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:     "ᓴᓇᓗᒍ",                 // Generate
	ActionCancel:       "ᐊᑎᖕᓂᖓ",                // Cancel
	ActionSave:         "ᓴᐳᒻᒥᔪᒍ",               // Save
	ActionLoad:         "ᐊᒐᒃᓯᓗᒍ",               // Load
	ActionReset:        "ᐅᑎᕐᕕᒋᓗᒍ ᓯᕗᓪᓕᖅᐸᐅᔪᒧᑦ",   // Reset
	ActionBrowse:       "ᓂᕈᐊᕐᓗᒍ",               // Browse
	ActionOpen:         "ᐅᒃᐱᕆᓗᒍ",               // Open
	ActionClose:        "ᒪᑕᐃᓗᒍ",                // Close
	ActionBack:         "ᐅᑎᕐᓗᒍ",                // Back
	ActionAdd:          "ᐃᓚᓕᐅᔾᔨᓗᒍ",             // Add
	ActionRemove:       "ᐊᒡᒋᑎᑕᐅᑎᓗᒍ",            // Remove
	ActionClear:        "ᓴᕿᙱᑦᑎᓗᒍ",              // Clear
	ActionClearAll:     "ᓴᕿᙱᑦᑎᓗᒍ ᑕᒪᐃᓐᓂ",        // Clear All
	ActionInstall:      "ᐃᓇᖏᕐᕕᒋᓗᒍ",             // Install
	ActionUninstall:    "ᓄᖅᑲᕐᕕᒋᓗᒍ",             // Uninstall
	ActionStart:        "ᐱᒋᐊᕐᓗᒍ",               // Start
	ActionStop:         "ᓄᖅᑲᕐᓗᒍ",               // Stop
	ActionPause:        "ᓄᖅᑲᕐᓗᒍ ᐊᑕᐅᓯᕐᒥᒃ",       // Pause
	ActionResume:       "ᐅᑎᕐᓗᒍ ᐱᓕᕆᓂᕐᒧᑦ",        // Resume
	ActionDelete:       "ᐊᒡᒋᑎᑕᐅᑎᓗᒍ",            // Delete
	ActionRefresh:      "ᓄᑖᕐᕕᒋᓗᒍ",              // Refresh
	ActionApply:        "ᐊᑐᓕᖅᑎᑦᑎᓗᒍ",            // Apply
	ActionConfirm:      "ᐃᓱᒪᒋᔭᖃᕐᓗᒍ",            // Confirm
	ActionEdit:         "ᐊᓯᔾᔨᓗᒍ",               // Edit
	ActionCopy:         "ᑐᒃᓯᑕᐅᓗᒍ",              // Copy
	ActionExport:       "ᐅᐸᒃᑎᑦᑎᓗᒍ",             // Export
	ActionImport:       "ᐃᓯᕐᓗᒍ ᓴᓇᔭᐅᓯᒪᔪᖅ",       // Import
	ActionCopyLog:      "ᑐᒃᓯᑕᐅᓗᒍ ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ",   // Copy Log
	ActionCopyMetadata: "ᑐᒃᓯᑕᐅᓗᒍ ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ", // Copy Metadata
	ActionLoadConfig:   "ᐊᒐᒃᓯᓗᒍ ᐋᖅᑭᒃᓱᐃᔾᔪᑎᒃ",    // Load Config
	ActionSaveConfig:   "ᓴᐳᒻᒥᔪᒍ ᐋᖅᑭᒃᓱᐃᔾᔪᑎᒃ",    // Save Config
	ActionViewQueue:    "ᖃᐅᔨᓴᕐᓗᒍ ᓄᐊᑕᐅᓯᒪᔪᑦ",     // View Queue
	ActionAddToQueue:   "ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᔾᔨᓗᒍ",   // Add to Queue
	ActionLoadVideo:    "ᐊᒐᒃᓯᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",      // Load Video
	ActionClearVideo:   "ᓴᕿᙱᑦᑎᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",     // Clear Video

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput:         "ᐃᓯᕐᕕᒃ",              // Input
	LabelOutput:        "ᐅᐸᒃᑎᕕᒃ",             // Output
	LabelSource:        "ᓇᑭᑐᐃᓐᓇᖓ",            // Source
	LabelDestination:   "ᑎᑭᕕᒃ",               // Destination
	LabelFormat:        "ᐊᕕᒃᑕᐅᓯᒪᓂᖓ",          // Format
	LabelQuality:       "ᐱᕚᓪᓕᖅᓯᒪᓂᖓ",          // Quality
	LabelResolution:    "ᓇᓗᓇᐃᕈᑕᐅᓂᖓ",          // Resolution
	LabelBitrate:       "ᐅᖃᐅᓯᕐᒧᑦ ᐊᑭᖏᑦ",       // Bitrate
	LabelFrameRate:     "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ", // Frame Rate
	LabelCodec:         "ᓴᓇᔾᔪᑎ",              // Codec
	LabelAudio:         "ᓂᐱᒃ",                // Audio
	LabelVideo:         "ᑕᕐᕆᔭᒐᒃᓴᖅ",           // Video
	LabelSubtitles:     "ᑎᑎᖅᑲᒃᓴᑦ",            // Subtitles
	LabelDuration:      "ᐊᑭᓕᒐᒃᓴᖓ",            // Duration
	LabelSize:          "ᐊᖏᕐᓂᖓ",              // Size
	LabelProgress:      "ᐱᓕᕆᔭᐅᓂᖓ",            // Progress
	LabelStatus:        "ᐊᑐᖅᑕᐅᓂᖓ",            // Status
	LabelLanguage:      "ᐅᖃᐅᓯᖅ",              // Language
	LabelVersion:       "ᑎᑎᕋᖅᓯᒪᔪᖅ",           // Version
	LabelLicense:       "ᐊᑐᕐᓂᒧᑦ ᐊᓂᕐᕋᖅ",       // License
	LabelNoFile:        "ᐃᓚᖏᓐᓂᒃ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ",   // No file loaded
	LabelNoVideoLoaded: "ᑕᕐᕆᔭᒐᒃᓴᖅ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ", // No video loaded
	LabelFileFmt:       "ᐃᓚᖓ: %s",            // File: %s

	// ── Inspect Module ────────────────────────────────────────────────────────
	InspectInstructions:       "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᒐᒃᓯᓗᒍ ᖃᐅᔨᒋᐊᕐᓗᒍ ᐊᑦᑕᓇᖏᑦᑐᒃᑯᑦ. ᐅᐸᒃᓯᓗᒍ ᅎᕝᕙᓘᓐᓃᑦ ᐸᓯᔭᒃᓴᒥᒃ ᐊᑐᕐᓗᒍ.", // Load a video to inspect
	InspectLoadingPreview:     "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᐊᒐᒃᑕᐅᔪᖅ...",                                               // Loading preview...
	InspectNoPreviewAvailable: "ᑕᑯᒃᓴᑦᑎᐊᕈᓐᓇᙱᓚᖅ",                                                       // No preview available

	// ── Audio Module ───────────────────────────────────────────────────────────
	AudioInstructions:    "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐅᐸᒃᓯᓗᒍ ᅎᕝᕙᓘᓐᓃᑦ ᓂᕈᐊᕐᓗᒍ", // Drop video file or click to browse
	AudioOutputFormat:    "ᐅᐸᒃᑎᕕᒃ ᐊᕕᒃᑕᐅᓯᒪᓂᖓ:",               // Output Format:
	AudioQualityPreset:   "ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐊᑐᖅᑕᐅᔪᒃᓴᖅ:",            // Quality Preset:
	AudioBitrate:         "ᐅᖃᐅᓯᕐᒧᑦ ᐊᑭᖏᑦ:",                   // Bitrate:
	AudioOutputDirectory: "ᐅᐸᒃᑎᕕᒃ ᓄᓇᓕᖕᒥ:",                   // Output Directory:

	// ── Filters Module ─────────────────────────────────────────────────────────
	FiltersInstructions:      "ᐊᓯᒃᑳᕈᑕᐅᔪᓂᒃ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᒧᑦ. ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐅᖃᐅᓯᕐᒧᑦ ᓄᑖᕐᕕᒋᓗᒍ.", // Apply filters and color corrections
	FiltersVideoPreview:      "ᑕᕐᕆᔭᒐᒃᓴᖅ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᖓ",                                         // Video preview
	FiltersSectionBrightness: "ᕿᓚᒥᒃ, ᐊᑦᑐᐃᓂᖏᑦ, ᐊᒻᒪ ᑎᑭᒃᑯᑦ ᐋᖅᑭᒃᓱᐃᓗᒋᑦ",                          // Adjust brightness, contrast, and saturation
	FiltersBrightness:        "ᕿᓚᒥᒃ:",                                                       // Brightness:
	FiltersContrast:          "ᐊᑦᑐᐃᓂᖏᑦ:",                                                    // Contrast:
	FiltersSaturation:        "ᑕᕐᕆᔭᒐᒃᓴᖅ ᑎᑭᒃᑯᑦ:",                                             // Saturation:
	FiltersSectionSharpen:    "ᓇᓗᓇᐃᕈᓐᓇᖅᑎᑦᑎᓗᒍ, ᑐᓂᓯᓗᒍ, ᐊᒻᒪ ᓱᓕᔪᒃᑎᑦᑎᓗᒍ",                         // Sharpen, blur, and denoise
	FiltersSharpness:         "ᓇᓗᓇᐃᕈᓐᓇᖅᑎᑦᑎᓂᖓ:",                                              // Sharpness:
	FiltersDenoise:           "ᓱᓕᔪᒃᑎᑦᑎᓂᖅ:",                                                  // Denoise:
	FiltersSectionRotate:     "ᐅᑎᕐᓗᒍ ᐊᒻᒪ ᑐᓴᕐᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",                                    // Rotate and flip video
	FiltersRotation:          "ᐅᑎᕈᑎᒃ:",                                                      // Rotation:
	FiltersFlipHorizontal:    "ᐊᑭᒋᔭᖓ ᐊᒃᓱᒋᔭᖓᓗ:",                                              // Flip Horizontal:
	FiltersFlipVertical:      "ᖁᓕᖓ ᑕᖅᑭᖓᓗ:",                                                  // Flip Vertical:
	FiltersSectionArtistic:   "ᓴᓇᙳᐊᕈᑎᑦ ᐊᑐᓕᖅᑎᑦᑎᓗᒋᑦ",                                          // Apply artistic effects
	FiltersSectionEra:        "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓯᕗᓪᓕᐅᔾᔨᓯᒪᔪᑦ ᐊᑐᓕᖅᑎᑦᑎᓗᒋᑦ",                             // Authentic decade-based video effects
	FiltersEraMode:           "ᑕᕐᕆᔭᒐᒃᓴᖅ ᐊᕙᑎ:",                                               // Era Mode:
	FiltersInterlacing:       "ᐊᑭᒋᔭᖓ ᑎᑭᒃᑯᑦ:",                                                // Interlacing:
	FiltersChromaNoise:       "ᑎᑭᒃᑯᑦ ᐃᓕᓐᓂᐊᕐᓂᖏᑦ:",                                            // Chroma Noise:
	FiltersTapeNoise:         "ᑕᐃᑯᓂᖓ ᐃᓕᓐᓂᐊᕐᓂᖏᑦ:",                                            // Tape Noise:
	FiltersTrackingError:     "ᐱᒋᐊᖅᑎᑕᐅᓂᖓ ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ:",                                       // Tracking Error:
	FiltersTapeDropout:       "ᑕᐃᑯᓂᖓ ᓄᖅᑲᕐᓂᖅ:",                                               // Tape Dropout:
	FiltersInterpHint:        "ᐊᔾᔨᒌᙱᑦᑐᑦ ᐊᑐᖅᑕᐅᔪᑦ ᐊᑐᕐᓂᒧᑦ ᐃᑲᔪᖅᑐᐃᔪᑦ.",                           // Balanced preset is recommended
	FiltersSectionMotion:     "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓄᑦᑎᓂᐊᖅᑐᑦ ᓄᑖᑦ ᐱᓕᕆᒐᒃᓴᑦ ᓴᓇᓗᒋᑦ",                         // Generate smoother motion
	FiltersPreset:            "ᐊᑐᖅᑕᐅᔪᒃᓴᖅ:",                                                  // Preset:
	FiltersTargetFPS:         "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ:",                                         // Target FPS:

	// ── Subtitles Module ─────────────────────────────────────────────────
	SubtitlesOfflineHint:  "ᐊᑕᐅᑦᑎᒃᑰᕈᑎᖃᙱᑦᑐᒥ STT ggml-small.bin ᐊᑐᖅᑐᖅ - API ᐊᑐᖅᑕᐅᑎᑦᑎᑦᑕᐃᓕᒪᔪᖅ", // Offline STT uses bundled ggml-small.bin - no API needed
	SubtitlesEmpty:        "ᑎᑎᖅᑲᒃᓴᓂᒃ ᖃᕆᑕᐅᔭᒃᑯᑦ ᑐᓂᓯᒋᑦ",                                       // Drag and drop subtitle files here
	SubtitlesExtractEmbed: "ᑎᑎᖅᑲᒃᓴᑦ ᐊᒡᒐᖅᑐᖅᑕᐅᓯᒪᔪᑦ ᓇᒡᒋᔭᒃᓴᐃᑦ ᐱᔭᕆᐊᖅᑕᑦ",                         // Extract embedded subtitle tracks
	SubtitlesOCROutput:    "OCR ᐊᒡᒐᖅᑐᕆᔭᐅᔪᖅ (ᐊᔾᔨᖅᑕᐅᓯᒪᔪᑦ ᑎᑎᖅᑲᒃᓴᑦ ᐱᒐᓱᐊᖅᑐᑦ):",                  // OCR output (image-based subtitles only):
	SubtitlesOCRLanguage:  "OCR ᐅᖃᐅᓯᖅ (tesseract, ᑕᑯᓐᓇᖅᑕᐃᓕᒪᔪᖅ: eng, fra, iku):",            // OCR language (tesseract, e.g. eng, fra, iku):
	SubtitlesShiftOffset:  "ᑎᑎᖅᑲᒃᓴᑦ ᑭᓯᐊᓂ ᐊᑯᓂᐅᔪᒃᑯᑦ ᐅᔾᔨᕆᓗᒋᑦ (ᐊᑦᑕᕐᓇᖅᑐᒃᑯᑦ):",                   // Shift all subtitle times by offset (seconds):
	SubtitlesStart:        "ᐱᒋᐊᖅᑐᖅ",                                                        // Start
	SubtitlesEnd:          "ᐃᓱᒪᒋᔭᒃᓴᖅ",                                                      // End

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:        "ᓴᐳᒻᒥᔪᖅ",               // Ready
	StatusProcessing:   "ᐱᓕᕆᔭᐅᔪᖅ...",           // Processing
	StatusComplete:     "ᐃᓱᓕᓚᐅᖅᑐᖅ",             // Complete
	StatusFailed:       "ᐊᔪᕈᓐᓃᖅᑐᖅ",             // Failed
	StatusCancelled:    "ᐊᑎᖕᓂᖓ",                // Cancelled
	StatusPending:      "ᐊᑦᑐᓂᖅ",                // Pending
	StatusRunning:      "ᐱᓕᕆᔭᐅᔪᖅ...",           // Running...
	StatusUpToDate:     "ᐊᓄᕌᓂᖅᑐᖅ",              // Up to date
	StatusNoActiveJobs: "○ ᐱᓕᕆᔭᐅᔪᓐᓇᖅᑐᑦ ᓄᖅᑲᖅᑐᑦ", // No active jobs

	// ── Trim ──────────────────────────────────────────────────────────────
	TrimInstructions:     "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᒐᒃᓯᓗᒍ ᐃᒡᒋᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᓗᒍ. ᓂᕈᐊᕐᓗᒍ ᐊᑭᓕᒐᒃᓴᖓᓂᒃ.", // Load video to set trim points
	TrimInPoint:          "ᐃᓯᕐᕕᒃ",                                                // In Point
	TrimOutPoint:         "ᐊᓂᒍᕐᕕᒃ",                                               // Out Point
	TrimSetIn:            "ᐃᓯᕐᕕᒃ ᐋᖅᑭᒃᓱᐃᓗᒍ",                                       // Set In
	TrimSetOut:           "ᐊᓂᒍᕐᕕᒃ ᐋᖅᑭᒃᓱᐃᓗᒍ",                                      // Set Out
	TrimClear:            "ᓴᕿᙱᑦᑎᓗᒋᑦ ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ",                                  // Clear Points
	TrimMode:             "ᐃᒡᒋᕐᓂᒧᑦ ᑐᕌᖓᓂᖓ:",                                       // Trim Mode:
	TrimModeKeep:         "ᓴᐳᒻᒥᓗᒍ ᐊᓐᓄᕌᒃᓯᒪᔪᖅ",                                     // Keep Region
	TrimModeCut:          "ᐃᒡᒋᕐᓗᒍ ᐊᓐᓄᕌᒃᓯᒪᔪᖅ",                                     // Cut Region
	TrimPreview:          "ᑕᑯᒃᓴᑦᑎᐊᕐᓗᒍ ᐃᒡᒋᕐᓂᖅ",                                    // Preview Trim
	TrimSmartCopy:        "ᐃᓕᓴᕆᔭᐅᓗᒍ ᑐᒃᓯᓗᒍ (ᓱᒃᑲᐃᑦᑑᓪᓗᓂ)",                           // Smart Copy (Fast)
	TrimRecode:           "ᓄᑖᕐᕕᒋᓗᒍ ᑎᑎᕋᖅᑕᐅᓂᖓ (ᐊᖏᕐᓂᖓᓂᒃ ᐊᑐᖅᑎᑕᐅᓗᒍ)",                  // Re-encode
	TrimSegment:          "ᐊᕕᒃᑕᐅᔪᖅ",                                              // Segment
	TrimAddSegment:       "ᐊᕕᒃᑕᐅᔪᒥᒃ ᐃᓚᓕᐅᔾᔨᓗᒍ",                                    // Add Segment
	TrimExportSegments:   "ᐊᕙᓗᒋᑦ ᐱᔾᔪᑕᐅᑉᓗᑎᒃ ᐊᓯᖏᑦ ᐃᓚᖏᑦ",                            // Export as separate files
	TrimInvalidSelection: "ᐱᔾᔪᑕᐅᓂᖅ ᐊᑐᖅᑎᑕᐅᓗᖅᓂᖅ",                                   // Invalid Selection
	TrimOutput:           "ᐅᐸᒃᑎᕕᒃ",                                               // Output

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:                "ᐋᖅᑭᒃᓱᐃᓂᖅ",                     // Settings
	SettingsTabGeneral:           "ᑕᒪᐃᓐᓄᑦ ᐊᑐᕐᓂᒧᑦ",                // General
	SettingsTabPreferences:       "ᐋᖅᑭᒃᓱᐃᓂᖅ",                     // Preferences
	SettingsTabDependencies:      "ᐱᔭᕆᐊᑐᔪᑦ",                      // Dependencies
	SettingsTabBenchmark:         "ᖃᐅᔨᓴᕐᓂᖅ",                      // Benchmark
	SettingsTabUpdates:           "ᓄᑖᓕᐅᕐᓂᖅ",                      // Updates
	SettingsTabAbout:             "ᒥᒃᓵᓄᑦ",                        // About
	SettingsTheme:                "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᒃ",                   // Theme
	SettingsOutputFolder:         "ᐅᐸᒃᑎᕕᒃ ᓄᓇᓕᖕᒥ",                 // Output Folder
	SettingsLanguage:             "ᐅᖃᐅᓯᖅ",                        // Language
	SettingsLanguageScript:       "ᑎᑎᕋᐅᓯᖅ",                       // Script
	SettingsScriptSyllabics:      "ᖃᓂᐅᔮᖅᐸᐃᑦ",                     // Traditional Syllabics
	SettingsScriptLatin:          "ᓚᑎᓐ",                          // Latin
	SettingsAppPreferences:       "ᐊᑐᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᓂᖅ",              // Application Preferences
	SettingsMasterSettings:       "ᐊᒃᓱᕈᕐᓇᖅᑑᔪᑦ ᐋᖅᑭᒃᓱᐃᓂᖅ",          // Master Settings
	SettingsHardwareAccel:        "ᓴᓇᒐᒃᓴᓂᒃ ᓱᒃᑲᐃᕙᒍᓐᓇᖅᑐᑦ",          // Hardware Acceleration (Global)
	SettingsDetect:               "ᓇᓗᓇᐃᖅᓯᓗᒍ",                     // Detect
	SettingsUseAuto:              "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ",                   // Use Auto
	SettingsDetectedFmt:          "ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ: %s",             // Detected: %s
	SettingsModuleVisibility:     "ᑕᑯᔭᐅᔪᓐᓇᖅᑐᑦ",                   // Module Visibility
	SettingsModuleVisibilityHint: "ᑕᑯᔭᐅᔪᓐᓇᖅᑐᑦ ᐃᓯᕐᕕᖕᒧᑦ ᑕᑯᔭᐅᕙᒃᑐᑦ.", // Module visibility applies on the main menu.
	SettingsShowUpscale:          "ᓴᖅᑮᓗᒍ ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ",             // Show Upscale module
	SettingsShowDisc:             "ᓴᖅᑮᓗᒍ ᑕᐃᑯᓂᖓ (ᓴᓇᓗᒍ & ᓂᕈᓐᓇᓯᓗᒍ)", // Show Disc category (Author & Rip)
	SettingsFFmpegMissing:        "ᐊᑦᑕᕐᓇᖅᑑᔪᖅ FFmpeg ᐱᔭᕆᐊᑐᔪᖅ ᓂᒡᓕᓯᒪᔪᖅ.\n\nᐃᓇᖏᕐᕕᒋᓗᒍ ᐱᓕᕆᔾᔪᑎᑦ ᐊᑐᕐᓗᒍ.\n\nᐃᓇᖏᕐᕕᒋᓂᖓ:\n%LOCALAPPDATA%\\VideoTools\\bin",
	SettingsInstallFFmpeg:        "ᐃᓇᖏᕐᕕᒋᓗᒍ FFmpeg ᒫᓐᓇ",               // Install FFmpeg Now
	SettingsOpenSettings:         "ᐅᒃᐱᕆᓗᒍ ᐋᖅᑭᒃᓱᐃᓂᖅ",                   // Open Settings
	SettingsContinueLimited:      "ᑲᔪᓯᓗᒍ ᐊᔪᕈᓐᓃᕈᑕᐅᓂᖓ",                  // Continue Limited Mode
	SettingsOpenReleases:         "ᐅᒃᐱᕆᓗᒍ ᓄᑕᐅᓯᒪᔪᑦ ᒪᒃᐱᒐᖅ",              // Open Releases Page
	SettingsUpdatesInfo:          "ᖃᐅᔨᓴᕐᓗᒍ ᐊᑐᕐᓗᓪᓗ ᓄᑖᑦ.",               // Check for updates and manage app updates.
	SettingsUpdatesAutoInfo:      "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ ᖃᐅᔨᓴᕐᓂᖅ ᓄᑖᑦ ᐋᖅᑭᒃᓯᒪᔪᒥᑦ.", // Checks for updates automatically based on the schedule above.

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:       "ᖃᐅᔨᓴᕐᓗᒍ ᓄᑖᑦ",                                                                                         // Check for Updates
	UpdateUpToDate:          "ᐊᓄᕌᓂᖅᑐᒥᒃ ᐊᑐᖅᑎᑕᐅᔪᖅ (%s).",                                                                             // You are running the latest version
	UpdateAvailable:         "ᓄᑖᖅ ᑐᓂᔭᐅᔪᓐᓇᖅᑐᖅ: %s\n\nᒫᓐᓇ %s ᐊᑐᖅᑎᑕᐅᔪᖅ.\n\nᐃᓇᖏᕐᕕᒋᓗᒍ ᓄᑖᖅ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ.",                                   // A new version is available
	UpdateInstall:           "ᐃᓇᖏᕐᕕᒋᓗᒍ ᓄᑖᖅ",                                                                                        // Install Update
	UpdatePatchesAvailable:  "ᒫᓐᓇ %s ᐊᑐᖅᑎᑕᐅᔪᖅ.\n\nᐊᓯᔾᔨᖅᑕᐅᓯᒪᔪᖅ ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ:\n  ᒫᓐᓇ: %s\n  ᓄᑖᖅ: %s\n\nᐃᓇᖏᕐᕕᒋᓗᒍ ᒪᑐᐃᓐᓇᐅᔪᑦ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ.", // Patches available
	UpdateInstallPatches:    "ᐃᓇᖏᕐᕕᒋᓗᒍ ᒪᑐᐃᓐᓇᐅᔪᑦ",                                                                                   // Install Patches
	UpdateHashMismatch:      "ᐊᓯᔾᔨᖅᑕᐅᓯᒪᔪᖅ ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ:",                                                                           // Hash mismatch detected:
	UpdateHashCurrent:       "ᒫᓐᓇ:",                                                                                                // Current:
	UpdateHashLatest:        "ᓄᑖᖅ:",                                                                                                // Latest:
	UpdateCurrentVersion:    "ᒫᓐᓇᐅᔪᖅ ᑎᑎᕋᖅᓯᒪᔪᖅ:",                                                                                    // Current Version:
	UpdateVersionHash:       "ᑐᑭᓯᒪᑎᑦᑎᔪᖅ:",                                                                                          // Version Hash:
	UpdateChecking:          "ᖃᐅᔨᓴᕐᓂᐊᖅᑐᖅ ᓄᑖᑦ...",                                                                                   // Checking for updates...
	UpdateError:             "ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ:",                                                                                         // Error:
	UpdateAvailableFmt:      "⬤ ᓄᑖᖅ ᑐᓂᔭᐅᔪᓐᓇᖅᑐᖅ: %s (%s)",                                                                           // Update available: %s (%s)
	UpdateNewBuildAvailable: "⬤ ᓄᑖᖅ ᐱᓕᕆᔾᔪᑕᐅᔪᖅ: %s (%s)",                                                                            // New build available: %s (%s)
	UpdateUpToDateFmt:       "✓ ᐊᓄᕌᓂᖅᑐᖅ (%s, %s)",                                                                                  // Up to date (latest: %s, %s)
	UpdateAutoCheck:         "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ ᖃᐅᔨᓴᖅ:",                                                                                   // Auto-check:
	UpdateDisabled:          "ᐊᑦᑐᐃᓂᖃᖏᑦᑐᖅ",                                                                                          // Disabled
	UpdateEveryHour:         "ᐃᑲᕐᕋᒥᒃ ᑕᒡᒐᓴᒥ",                                                                                        // Every hour
	UpdateEvery2Hours:       "2-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                                                                                         // Every 2 hours
	UpdateEvery3Hours:       "3-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                                                                                         // Every 3 hours
	UpdateEvery4Hours:       "4-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                                                                                         // Every 4 hours
	UpdateEvery6Hours:       "6-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                                                                                         // Every 6 hours
	UpdateEvery12Hours:      "12-ᓂᒃ ᐃᑲᕐᕋᓂᒃ",                                                                                        // Every 12 hours
	UpdateDaily:             "ᐅᓪᓗᒥ ᑕᒡᒐᓴᒥ",                                                                                          // Daily
	UpdateSemiWeekly:        "3-ᓂᒃ ᐅᓪᓗᓂᒃ ᑕᒡᒐᓴᒥ",                                                                                    // Semi-weekly (every 3 days)
	UpdateWeekly:            "ᐱᓇᓱᐊᕈᓯᒥ ᑕᒡᒐᓴᒥ",                                                                                       // Weekly
	UpdateBiWeekly:          "2-ᓂᒃ ᐱᓇᓱᐊᕈᓯᓂᒃ ᑕᒡᒐᓴᒥ",                                                                                 // Bi-weekly (every 2 weeks)
	UpdateMonthly:           "ᑕᕐᕿᒥ ᑕᒡᒐᓴᒥ",                                                                                          // Monthly
	UpdateBiMonthly:         "2-ᓂᒃ ᑕᕐᕿᓂᒃ ᑕᒡᒐᓴᒥ",                                                                                    // Bi-monthly (every 2 months)
	UpdateAutoCheckDesc:     "ᐊᐅᓚᑕᐅᑎᑕᐅᓗᒍ ᖃᐅᔨᓴᖅ ᓄᑖᑦ ᐋᖅᑭᒃᓯᒪᔪᒥᑦ.",                                                                     // Auto-checks based on schedule

	// ── Dependencies ─────────────────────────────────────────────────────
	DependenciesTitle:        "ᐱᔭᕆᐊᑐᔪᑦ ᐱᓕᕆᔾᔪᑎᑦ",              // System Dependencies
	DependenciesDesc:         "ᐊᐅᓚᑦᑎᓗᒋᑦ VideoTools ᐱᔭᕆᐊᑐᔪᑦ.", // Manage VideoTools dependencies
	DependenciesInstalled:    "ᐃᓇᖏᕐᓯᒪᔪᖅ",                     // Installed
	DependenciesNotInstalled: "ᐃᓇᖏᕐᓯᒪᙱᓚᖅ",                    // Not Installed
	DependenciesCore:         "ᐊᑦᑕᕐᓇᖅᑑᔪᖅ ᐱᔭᕆᐊᑐᔪᖅ",            // Core dependency
	DependenciesInstall:      "ᐃᓇᖏᕐᕕᒋᓗᒍ",                     // Install
	DependenciesUninstall:    "ᓄᖅᑲᕐᕕᒋᓗᒍ",                     // Uninstall
	DependenciesRefresh:      "ᓄᑖᕐᕕᒋᓗᒍ ᐊᑐᖅᑕᐅᔪᑦ",              // Refresh Status
	DependenciesRequiredBy:   "ᐱᔭᕆᐊᑐᔭᐅᔪᑦ:",                   // Required by:
	DependenciesInstallCmd:   "ᐃᓇᖏᕐᕕᒋᓗᒍ: %s",                 // Install: %s

	// ── Benchmark ────────────────────────────────────────────────────────
	BenchmarkTitle:     "ᖃᐅᔨᓴᕐᓂᖅ ᓴᓇᒐᒃᓴᒥᒃ",                                 // Hardware Benchmark
	BenchmarkDesc:      "ᖃᐅᔨᓴᕐᓗᒍ ᑕᕐᕆᔭᒐᓕᕆᔾᔪᑎᑦ ᐱᓕᕆᓂᖏᑦ ᐊᑐᕐᕕᒋᓂᐊᖅᑕᑦ ᐃᓕᔭᐅᔪᒃᓴᑦ.", // Test video encoding performance
	BenchmarkRunButton: "ᐱᒋᐊᕐᓗᒍ ᖃᐅᔨᓴᕐᓂᖅ ᓴᓇᒐᒃᓴᒥᒃ",                          // Run Hardware Benchmark
	BenchmarkRecent:    "ᓂᕈᓐᓇᓯᔪᓂᒃ ᖃᐅᔨᓴᓚᐅᖅᑕᑦ",                              // Recent Benchmarks

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle:      "ᓄᐊᑕᐅᓯᒪᔪᑦ",                // Queue
	QueueEmpty:      "ᐱᓕᕆᒐᒃᓴᑦ ᓄᐊᑕᐅᓯᒪᔪᑦ ᓄᖅᑲᖅᑐᑦ", // No jobs in queue
	QueueInProgress: "ᐱᓕᕆᔭᐅᔪᖅ",                 // In Progress
	QueueCompleted:  "ᐃᓱᓕᓚᐅᖅᑐᖅ",                // Completed
	QueueFailed:     "ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ",              // Failed
	QueueJobRunning: "ᐱᓕᕆᔭᐅᔪᖅ...",              // Running...
	QueueJobPending: "ᐊᑦᑐᓂᖅ",                   // Pending

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle:     "ᐊᑐᓚᐅᖅᑕᑦ",     // HISTORY
	HistoryNoEntries: "ᐃᓚᖏᑦ ᓄᖅᑲᖅᑐᑦ", // No entries

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt:    "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐅᐸᒃᓯᓗᒍ",    // Drop a video file here
	ConvertOutputFormat:  "ᐅᐸᒃᑎᕕᒃ ᐊᕕᒃᑕᐅᓯᒪᓂᖓ",    // Output Format
	ConvertHardwareAccel: "ᓴᓇᒐᒃᓴᓂᒃ ᓱᒃᑲᐃᕙᒍᓐᓇᖅᑐᑦ", // Hardware Acceleration

	// ── Compare ──────────────────────────────────────────────────────────────────
	CompareInstructions:    "ᑕᕐᕆᔭᒐᒃᓴᑦ ᒪᕐᕉᒃ ᐊᒐᒃᓯᓗᒋᒃ ᓇᓗᓇᐃᖅᓯᓗᒋᑦ. ᐅᐸᒃᓯᓗᒋᒃ ᅎᕝᕙᓘᓐᓃᑦ ᐸᓯᔭᒃᓴᒥᒃ ᐊᑐᕐᓗᒍ.", // Load two videos to compare
	CompareFullscreen:      "ᐊᖏᕐᓂᖓᓂᒃ ᑕᑯᒃᓴᑦᑎᐊᕐᓗᒍ",                                              // Fullscreen Compare
	CompareCopyReport:      "ᑐᒃᓯᓗᒍ ᓇᓗᓇᐃᕈᑕᐅᔪᖅ",                                                 // Copy Comparison
	CompareHidePlayer:      "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᓄᖅᑲᕐᓗᒍ",                                               // Hide Player
	CompareShowPlayer:      "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᓴᖅᑮᓗᒍ",                                                // Show Player
	CompareLoadFile1:       "ᐊᒐᒃᓯᓗᒍ ᐃᓚᖓ 1",                                                    // Load File 1
	CompareLoadFile2:       "ᐊᒐᒃᓯᓗᒍ ᐃᓚᖓ 2",                                                    // Load File 2
	CompareFile1NotLoaded:  "ᐃᓚᖓ 1: ᐊᒐᒃᓯᓯᒪᙱᓚᖅ",                                                // File 1: Not loaded
	CompareFile2NotLoaded:  "ᐃᓚᖓ 2: ᐊᒐᒃᓯᓯᒪᙱᓚᖅ",                                                // File 2: Not loaded
	CompareFile1Fmt:        "ᐃᓚᖓ 1: %s",                                                       // File 1: %s
	CompareFile2Fmt:        "ᐃᓚᖓ 2: %s",                                                       // File 2: %s
	CompareFile1Info:       "ᐃᓚᖓ 1 ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ",                                              // File 1 Info
	CompareFile2Info:       "ᐃᓚᖓ 2 ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ",                                              // File 2 Info
	CompareBackToView:      "< ᐅᑎᕐᓗᒍ ᓇᓗᓇᐃᖅᓯᕕᒻᒧᑦ",                                              // < BACK TO COMPARE
	ComparePlayBoth:        "▶ ᑕᕐᕆᔭᒐᒃᓴᑦ ᒪᕐᕉᒃ ᐱᒋᐊᕐᓗᒋᒃ",                                         // ▶ Play Both
	ComparePauseBoth:       "⏸ ᑕᕐᕆᔭᒐᒃᓴᑦ ᒪᕐᕉᒃ ᓄᖅᑲᕐᓗᒋᒃ",                                         // ⏸ Pause Both
	CompareSyncTitle:       "ᐊᑦᑐᖅᑕᐅᓂᑯ ᑕᕐᕆᔭᒐᓕᕆᔪᖅ",                                              // Synchronized Playback
	CompareSyncMsg:         "ᐊᑦᑐᖅᑕᐅᓂᑯ ᑕᕐᕆᔭᒐᓕᕆᔪᖅ ᑐᓂᔭᐅᓂᐊᖅᑐᖅ.\n\nᒫᓐᓇ, ᐊᑐᓕᖅᑎᑦᑎᓗᒍ ᐊᑐᕐᓂᒧᑦ ᐱᓕᕆᔾᔪᑎᑦ.", // Sync msg
	CompareSyncMsgShort:    "ᐊᑦᑐᖅᑕᐅᓂᑯ ᑕᕐᕆᔭᒐᓕᕆᔪᖅ ᑐᓂᔭᐅᓂᐊᖅᑐᖅ.",                                   // Sync msg short
	CompareSideInfo:        "ᑕᑯᒃᓴᑦᑎᐊᕐᓂᒧᑦ ᐊᕙᒋᓗᒍ.",                                              // Side-by-side info
	CompareCopied:          "ᑐᒃᓯᑕᐅᓚᐅᖅᑐᖅ",                                                      // Copied
	CompareCopiedMsg:       "ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ ᑐᒃᓯᑕᐅᓚᐅᖅᑐᑦ",                                         // Comparison metadata copied
	CompareCopiedFileMsg:   "ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ ᑐᒃᓯᑕᐅᓚᐅᖅᑐᑦ",                                         // Metadata copied
	CompareNoVideosTitle:   "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓄᖅᑲᖅᑐᑦ",                                                 // No Videos
	CompareNoVideosFSMsg:   "ᑕᕐᕆᔭᒐᒃᓴᑦ ᒪᕐᕉᒃ ᐊᒐᒃᓯᓗᒋᒃ ᑕᑯᒃᓴᑦᑎᐊᕐᓂᒧᑦ.",                              // Load two videos for fullscreen
	CompareNoVideosCopyMsg: "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᒐᒃᓯᓗᒍ ᑐᒃᓯᓗᒍ ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᑦ.",                            // Load at least one video to copy

	// ── Player ───────────────────────────────────────────────────────────────────
	PlayerInstructions: "VT_Player - ᑕᕐᕆᔭᒐᒃᓴᓂᒃ ᑕᕐᕆᔭᒐᓕᕆᔪᖅ ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᓂᒃ ᐊᑐᖅᑎᑦᑎᓪᓗᓂ.", // VT_Player

	// ── Rip ──────────────────────────────────────────────────────────────────────
	RipDropPrompt:       "DVD/ISO/VIDEO_TS ᐅᐸᒃᓯᓗᒍ",                                     // Drop DVD/ISO/VIDEO_TS path here
	RipOutputPath:       "ᐅᐸᒃᑎᕕᒃ ᐊᑭᓕᒐᒃᓴᖓ",                                              // Output path
	RipSource:           "ᓇᑭᑐᐃᓐᓇᖓ",                                                     // Source
	RipFormatLabel:      "ᐊᕕᒃᑕᐅᓯᒪᓂᖓ",                                                   // Format
	RipLog:              "ᓂᕈᓐᓇᓯᓂᕐᒧᑦ ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ",                                        // Rip Log
	RipAddToQueue:       "ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᔾᔨᓗᒍ",                                          // Add Rip to Queue
	RipNow:              "ᒫᓐᓇ ᓂᕈᓐᓇᓯᓗᒍ",                                                 // Rip Now
	RipClearISO:         "ᓴᕿᙱᑦᑎᓗᒍ ISO",                                                 // Clear ISO
	RipJobQueuedTitle:   "ᓄᐊᑕᐅᓯᒪᔪᑦ",                                                    // Queue
	RipJobQueuedMsg:     "ᓂᕈᓐᓇᓯᓂᕐᒧᑦ ᐱᓕᕆᒐᒃᓴᖅ ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᖅᑕᐅᓚᐅᖅᑐᖅ.",                   // Rip job added to queue
	RipStartTitle:       "ᓂᕈᓐᓇᓯᓂᖅ",                                                     // Rip
	RipStartMsg:         "ᓂᕈᓐᓇᓯᓂᖅ ᐱᒋᐊᖅᑐᖅ! ᓄᐊᑕᐅᓯᒪᔪᑦ ᑕᑯᓗᒍ.",                              // Rip started
	RipNoConfigTitle:    "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ ᓄᖅᑲᖅᑐᑦ",                                           // No Config
	RipNoConfigMsg:      "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ ᓴᐳᒻᒥᔭᐅᓯᒪᔪᑦ ᓇᓗᓇᐃᖅᑕᐅᓯᒪᙱᓚᑦ. ᐊᓯᔾᔨᓯᒪᔪᑎᑦ ᐊᑐᓕᕐᕕᒋᓛᓚᐅᖅᑐᖅ.", // No saved config
	RipConfigSavedTitle: "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ ᓴᐳᒻᒥᔭᐅᓚᐅᖅᑐᑦ",                                      // Config Saved
	RipConfigSavedFmt:   "ᓴᐳᒻᒥᔭᐅᓚᐅᖅᑐᖅ %s-ᒧᑦ",                                           // Saved to %s
	RipErrNoSource:      "DVD/ISO/VIDEO_TS ᓇᑭᑐᐃᓐᓇᖓ ᐋᖅᑭᒃᓱᐃᓗᒍ",                           // set a source path

	// ── Upscale ──────────────────────────────────────────────────────────────────
	UpscaleNow:             "ᒫᓐᓇ ᐱᕚᓪᓕᖅᑎᑦᑎᓗᒍ",                      // UPSCALE NOW
	UpscaleStartedTitle:    "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ ᐱᒋᐊᖅᑐᖅ",                   // Upscale Started
	UpscaleStartedFmt:      "ᐱᕚᓪᓕᖅᑎᑦᑎᔭᐅᔪᖅ %s-ᒧᑦ.\nᓄᐊᑕᐅᓯᒪᔪᑦ ᑕᑯᓗᒍ.", // Upscaling to %s
	UpscaleAddedTitle:      "ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᖅᑕᐅᓚᐅᖅᑐᖅ",              // Added to Queue
	UpscaleAddedFmt:        "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᕐᒧᑦ ᐱᓕᕆᒐᒃᓴᖅ ᐃᓚᓕᐅᖅᑕᐅᓚᐅᖅᑐᖅ.\nᑎᑭᕕᒃ: %s, ᐊᑐᖅᑕᐅᔾᔪᑎᒃ: %s",
	UpscaleSourceFmt:       "ᓇᑭᑐᐃᓐᓇᖓ: %dx%d",                                              // Source: %dx%d
	UpscaleSourceNA:        "ᓇᑭᑐᐃᓐᓇᖓ: ᓴᕿᙱᑦᑐᖅ",                                             // Source: N/A
	UpscaleMethodFmt:       "ᐊᑐᖅᑕᐅᔾᔪᑎᒃ: %s",                                               // Method: %s
	UpscaleTargetFmt:       "ᑎᑭᕕᒃ: %s",                                                    // Target: %s
	UpscaleBlurFmt:         "ᐃᒻᒪᖄ ᐊᑦᑐᐃᓂᖓ: %.2f",                                           // Blur Strength: %.2f
	UpscaleEnableBlur:      "ᐊᑦᑐᐃᓂᖓ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ",                                            // Enable Blur
	UpscaleAdjustFilters:   " ᐊᓯᒃᑳᕈᑕᐅᔪᑦ ᐋᖅᑭᒃᓱᐃᓗᒋᑦ",                                        // Adjust Filters
	UpscaleAdjustFmt:       "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᒃ: %.2fx",                                           // Adjustment: %.2fx
	UpscaleDenoiseFmt:      "ᓱᓕᔪᒃᑎᑦᑎᓂᖅ: %.2f",                                             // Denoise: %.2f
	UpscaleDenoiseAvail:    "ᓱᓕᔪᒃᑎᑦᑎᓂᖅ ᑐᓂᔭᐅᔪᓐᓇᖅᑐᖅ ᐊᑐᕆᐊᖅ ᑎᓕᕋᑦᑎᒍᑦ",                          // Denoise available
	UpscaleDenoiseUnavail:  "ᓱᓕᔪᒃᑎᑦᑎᓂᖅ ᐊᑐᕆᐊᖅ ᑎᓕᕋᑦᑎᒍᑦ ᐊᑐᕈᓐᓇᖅᑐᖅ",                            // Denoise unavailable
	UpscaleFrameRateFmt:    "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ: %s",                                      // Frame Rate: %s
	UpscaleMotionInterp:    "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓄᑦᑎᓂᐊᖅᑐᑦ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ",                                 // Use Motion Interpolation
	UpscaleMotionHint:      "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓄᑦᑎᓂᐊᖅᑐᑦ ᓄᑖᑦ ᐱᓕᕆᒐᒃᓴᑦ ᓴᓇᓗᑎᒃ",                         // Motion hint
	UpscaleAIEnabled:       "AI ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ",                                     // Use AI Upscaling
	UpscaleAIDetected:      "Real-ESRGAN ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ - ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐊᑐᕈᓐᓇᖅᑐᖅ",               // AI detected
	UpscaleAINotDetected:   "Real-ESRGAN ᓇᓗᓇᐃᖅᑕᐅᓯᒪᙱᓚᖅ. ᐃᓇᖏᕐᕕᒋᓗᒍ ᐱᕚᓪᓕᖅᓯᒪᓂᒧᑦ:",              // AI not detected
	UpscaleAIPython:        "Python Real-ESRGAN ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ, ᑭᓯᐊᓂ ncnn ᐊᑐᕆᐊᖅᑐᖅ.",         // Python detected
	UpscaleAIFallback:      "ᐊᑐᕐᓂᒧᑦ ᓴᓇᔾᔪᑎᑦ ᐊᑐᖅᑕᐅᓂᐊᖅᑐᑦ.",                                   // Fallback methods
	UpscaleAINote:          "ᑐᑭᓯᒋᐊᒃᑲᓐᓂᕈᑎᒃ: AI ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ ᓱᒃᑲᐃᑦᑑᙱᓚᖅ ᑭᓯᐊᓂ ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐊᖏᓂᖅᓴᖅ", // AI note
	UpscaleAIAdvanced:      "ᐊᒃᓱᕈᕐᓇᖅᑑᔪᑦ (ncnn ᐊᑐᖅᑕᐅᔾᔪᑎ)",                                  // Advanced
	UpscaleFaceEnhance:     "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐱᕚᓪᓕᖅᑎᑦᑎᓗᒍ (Python/GFPGAN ᐱᔭᕆᐊᑐᔪᖅ)",                // Face Enhancement
	UpscaleTTACheck:        "TTA ᐊᑐᓕᖅᑎᑦᑎᓗᒍ (ᓱᒃᑲᐃᑦᑑᙱᓚᖅ, ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐊᖏᓂᖅᓴᖅ)",                 // Enable TTA
	UpscaleBitrateHint:     "CRF ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐋᖅᑭᒃᓱᐃᓗᒍ; ᐊᑭᓕᒐᒃᓴᖓ ᑕᑯᕕᒋᓗᒍ ᐊᖏᕐᓂᒧᑦ.",              // Bitrate hint
	UpscaleClassicDesc:     "ᐊᑐᕐᓂᒧᑦ ᓴᓇᔾᔪᑎᑦ - ᑕᒪᐃᓐᓂ ᑐᓂᔭᐅᔪᓐᓇᖅᑐᑦ",                            // Classic desc
	UpscaleOptionalBlur:    "ᐊᑦᑐᐃᓂᖓ ᐊᑐᖅᑕᐅᔪᒃᓴᖅ",                                            // Optional blur
	UpscaleFilterIntHint:   "ᐊᓯᒃᑳᕈᑕᐅᔪᑦ ᐱᕚᓪᓕᖅᑎᑦᑎᓂᕐᒧᑦ ᐃᓗᐊᓂ ᐊᑐᓕᖅᑎᑦᑎᔭᐅᔪᑦ.",                    // Filter hint
	UpscaleVideoBox:        "ᑕᕐᕆᔭᒐᒃᓴᖅ",                                                    // Video
	UpscaleTargetResBox:    "ᑎᑭᕕᒃ ᓇᓗᓇᐃᕈᑕᐅᓂᖓ",                                              // Target Resolution
	UpscaleEncodingBox:     "ᑕᕐᕆᔭᒐᒃᓴᖅ ᑎᑎᕋᖅᓯᒪᔭᖓ",                                           // Video Encoding
	UpscaleScalingBox:      "ᐊᖏᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᓂᖅ",                                             // Scaling
	UpscaleFrameRateBox:    "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ",                                          // Frame Rate
	UpscaleFilterIntBox:    "ᐊᓯᒃᑳᕈᑕᐅᔪᑦ ᐃᓗᖅᑯᓯᖓ",                                            // Filter Integration
	UpscaleAIBox:           "AI ᐱᕚᓪᓕᖅᑎᑦᑎᓂᖅ",                                               // AI Upscaling
	UpscaleResLabel:        "ᓇᓗᓇᐃᕈᑕᐅᓂᖓ:",                                                  // Resolution:
	UpscaleVideoCodecLabel: "ᑕᕐᕆᔭᒐᒃᓴᖅ ᓴᓇᔾᔪᑎᒃ:",                                            // Video Codec:
	UpscaleEncoderLabel:    "ᑎᑎᕋᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᔾᔪᑎᒃ:",                                         // Encoder Preset:
	UpscaleQualityLabel:    "ᐱᕚᓪᓕᖅᓯᒪᓂᖓ ᐊᑐᖅᑕᐅᔪᒃᓴᖅ:",                                        // Quality Preset:
	UpscaleBitrateLabel:    "ᐅᖃᐅᓯᕐᒧᑦ ᐊᑭᖏᑦ ᑐᕌᖓᓂᖓ:",                                         // Bitrate Mode:
	UpscaleTargetFPSLabel:  "ᑎᑭᕕᒃ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ:",                                    // Target FPS:
	UpscaleAIModelLabel:    "AI ᐊᑐᖅᑕᐅᔾᔪᑎᒃ:",                                               // AI Model:
	UpscaleAIPresetLabel:   "ᐱᓕᕆᔾᔪᑎᒃ ᐊᑐᖅᑕᐅᔪᒃᓴᖅ:",                                          // Processing Preset:
	UpscaleAIScaleLabel:    "ᐱᕚᓪᓕᖅᑎᑦᑎᓂᒧᑦ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᒃ:",                                     // Upscale Factor:
	UpscaleAITileLabel:     "ᐊᕕᒃᓯᒪᔪᖅ ᐊᖏᕐᓂᖓ:",                                              // Tile Size:
	UpscaleAIOutputLabel:   "ᐅᐸᒃᑎᕕᒃ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ:",                                          // Output Frames:
	UpscaleGPULabel:        "GPU:",                                                        // GPU:
	UpscaleThreadsLabel:    "ᐱᓕᕆᒐᒃᓴᑦ (ᐊᒐᒃᓯᓗᒋᑦ/ᐱᓕᕆᓗᒋᑦ/ᓴᐳᒻᒥᓗᒋᑦ):",                           // Threads:
	UpscaleFilterIntLabel:  "ᐊᓯᒃᑳᕈᑕᐅᔪᑦ ᐃᓗᖅᑯᓯᖓ:",                                           // Filter Integration:
	UpscaleScalingLabel:    "ᐊᖏᕐᓂᒧᑦ ᐋᖅᑭᒃᓱᐃᓂᒧᑦ ᓴᓇᔾᔪᑎᒃ:",                                    // Scaling Algorithm:

	// ── Frame Interpolation (RIFE) ────────────────────────────────────────────
	RIFEBoxTitle:        "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᓄᑖᑦ ᓴᓇᓂᖅ (RIFE)",                 // Frame Interpolation (RIFE)
	RIFEDetected:        "rife-ncnn-vulkan ᓇᓗᓇᐃᖅᑕᐅᓚᐅᖅᑐᖅ",              // rife detected
	RIFENotDetected:     "rife-ncnn-vulkan ᓇᓗᓇᐃᖅᑕᐅᓯᒪᙱᓚᖅ. ᐃᓇᖏᕐᕕᒋᓗᒍ:",   // rife not detected
	RIFEEnabled:         "RIFE ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᓄᑖᑦ ᓴᓇᓂᖅ ᐊᑐᓕᖅᑎᑦᑎᓗᒍ",         // Enable RIFE
	RIFEMultiplierLabel: "ᐊᒥᓱᑦ ᒥᒃᓵᓄᑦ:",                                // Multiplier:
	RIFEModelLabel:      "ᐊᑐᖅᑕᐅᔾᔪᑎᒃ:",                                 // Model:
	RIFEEstFPSFmt:       "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᑭᓕᒐᒃᓴᖓ: %.0f fps",               // Estimated output: %.0f fps
	RIFENote:            "RIFE ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᓄᑖᑦ ᓴᓇᔪᖅ ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᑭᓕᒃᑯᑦ.", // RIFE note

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow:        "ᐱᒋᐊᕈᑎᒋᓗᒍ",                                                // GENERATE NOW
	ThumbnailContactSheet:       "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᑎᑎᖅᑲᖅ",                                        // Contact Sheet
	ThumbnailColumns:            "ᐊᑕᐅᓯᕐᒥᒃ ᑲᑎᑕᐅᓯᒪᔪᑦ",                                        // Columns
	ThumbnailRows:               "ᐊᑕᐅᓯᕐᒥᒃ ᑲᑎᑕᐅᓯᒪᔪᑦ ᐊᑭᓕᒐᒃᓴᖓ",                                // Rows
	ThumbnailOutputFolder:       "ᐅᐸᒃᑎᕕᒃ ᓄᓇᓕᖕᒥ",                                            // Output Folder
	ThumbnailIndividual:         "ᐊᑕᐅᓯᐅᔪᑦ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ",                                      // Individual Thumbnails
	ThumbnailLoadVideo:          "ᐊᒐᒃᓯᓗᒍ ᑕᕐᕆᔭᒐᒃᓴᖅ",                                         // Load Video
	ThumbnailNoFile:             "ᐃᓚᖏᓐᓂᒃ ᐊᒐᒃᓯᓯᒪᙱᓚᖅ",                                        // No file loaded
	ThumbnailFileLoaded:         "ᐃᓚᖓ: ᑕᕐᕆᔭᒐᒃᓴᖅ ᐊᒐᒃᓯᑕᐅᓚᐅᖅᑐᖅ",                               // File: video loaded
	ThumbnailInstructions:       "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᓴᓇᓗᒍ. ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᒐᒃᓯᓗᒍ ᐋᖅᑭᒃᓱᐃᓗᓪᓗᒍ.", // Instructions
	ThumbnailContactSheetToggle: "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᑎᑎᖅᑲᖅ ᓴᓇᓗᒍ (ᐊᑕᐅᓯᕐᒥᒃ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᒃ)",              // Contact Sheet toggle
	ThumbnailShowTimestamps:     "ᐊᑭᓕᒐᒃᓴᑦ ᑕᑯᒃᓴᑦᑎᐊᕈᑎᓂ ᓴᖅᑮᓗᒋᑦ",                               // Show timestamps
	ThumbnailContactSheetGrid:   "ᑕᑯᒃᓴᑦᑎᐊᕐᕕᒃ ᑎᑎᖅᑲᖅ ᑲᑎᑦᑎᓯᒪᔪᖅ",                               // Contact Sheet Grid
	ThumbnailSize:               "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᒃ ᐊᖏᕐᓂᖓ:",                                       // Thumbnail Size:
	ThumbnailCountFmt:           "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐊᒥᓱᑦ: %d",                                     // Thumbnail Count: %d
	ThumbnailWidthFmt:           "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᒃ ᐊᖏᔪᑦ: %d px",                                  // Thumbnail Width: %d px
	ThumbnailColumnsFmt:         "ᐊᑕᐅᓯᕐᒥᒃ ᑲᑎᑕᐅᓯᒪᔪᑦ: %d",                                    // Columns: %d
	ThumbnailRowsFmt:            "ᐊᑕᐅᓯᕐᒥᒃ ᑲᑎᑕᐅᓯᒪᔪᑦ ᐊᑭᓕᒐᒃᓴᖓ: %d",                            // Rows: %d
	ThumbnailTotalFmt:           "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᑕᒪᐃᓐᓂ: %d",                                    // Total thumbnails: %d
	ThumbnailAddToQueue:         "ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᔾᔨᓗᒍ",                                      // Add to Queue
	ThumbnailAddAllToQueue:      "ᑕᒪᐃᓐᓂᒃ ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᔾᔨᓗᒋᑦ",                              // Add All to Queue
	ThumbnailLoadedVideos:       "ᐊᒐᒃᓯᑕᐅᓯᒪᔪᑦ ᑕᕐᕆᔭᒐᒃᓴᑦ:",                                    // Loaded Videos:
	ThumbnailVideoFmt:           "ᑕᕐᕆᔭᒐᒃᓴᖅ %d",                                             // Video %d
	ThumbnailNoVideoTitle:       "ᑕᕐᕆᔭᒐᒃᓴᖅ ᓄᖅᑲᖅᑐᖅ",                                         // No Video
	ThumbnailNoVideoMsg:         "ᑕᕐᕆᔭᒐᒃᓴᒥᒃ ᐊᒐᒃᓯᓗᒍ ᓯᕗᓪᓕᐅᔾᔨᓗᒍ.",                             // Please load a video file first
	ThumbnailStartedTitle:       "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ",                                              // Thumbnails
	ThumbnailStartedMsg:         "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᓴᓇᓂᖅ ᐱᒋᐊᖅᑐᖅ! ᓄᐊᑕᐅᓯᒪᔪᑦ ᑕᑯᓗᒍ.",                  // Generation started
	ThumbnailJobQueuedTitle:     "ᓄᐊᑕᐅᓯᒪᔪᑦ",                                                // Queue
	ThumbnailJobQueuedMsg:       "ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐱᓕᕆᒐᒃᓴᖅ ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᖅᑕᐅᓚᐅᖅᑐᖅ!",              // Job added to queue
	ThumbnailNoVideosTitle:      "ᑕᕐᕆᔭᒐᒃᓴᑦ ᓄᖅᑲᖅᑐᑦ",                                         // No Videos
	ThumbnailNoVideosMsg:        "ᑕᕐᕆᔭᒐᒃᓴᑦ ᐊᒐᒃᓯᓗᒋᑦ ᓯᕗᓪᓕᐅᔾᔨᓗᒋᑦ ᐃᓚᓕᐅᔾᔨᓗᒋᑦ.",                  // Load videos first
	ThumbnailJobsQueuedFmt:      "%d ᑕᑯᒃᓴᑦᑎᐊᕈᑎᑦ ᐱᓕᕆᒐᒃᓴᑦ ᓄᐊᑕᐅᓯᒪᔪᓄᑦ ᐃᓚᓕᐅᖅᑕᐅᓚᐅᖅᑐᑦ.",           // Queued %d jobs

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle:       "ᒥᒃᓵᓄᑦ VideoTools",                                                            // About VideoTools
	AboutDescription: "ᑕᕐᕆᔭᒐᒃᓴᓂᒃ ᐱᓕᕆᔾᔪᑎᒃ ᓱᒃᑲᐃᑦᑑᒍᑎ ᐊᒻᒪ ᓴᓇᔭᐅᔪᒃᑯᑦ.",                                    // A native video processing toolkit
	AboutLicense:     "ᐊᑐᕐᓂᒧᑦ ᐊᓂᕐᕋᖅ",                                                                // License
	AboutSupport:     "ᐃᑲᔪᖅᑐᐃᓂᖅ",                                                                    // Support
	AboutLogsFolder:  "ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ ᓄᓇᓕᖕᓂ",                                                            // Logs Folder
	AboutScanForDocs: "ᑎᑎᕋᖅᑕᐅᓯᒪᔪᑦ ᓂᕈᐊᕐᓗᒋᑦ",                                                          // Scan for docs
	AboutFeedback:    "ᑐᓴᕈᑕᐅᔪᒃᓴᑦ: ᑎᑎᕋᖅᑕᐅᓯᒪᔪᓂᒃ ᑕᑯᓗᒋᑦ ᑕᕐᕆᔭᒐᓕᕆᔾᔪᑎᒃ ᑕᑯᕕᒋᓗᒍ; ᐱᒻᒪᕆᐅᔪᑦ ᑎᑎᕋᖅᑕᐅᓯᒪᔪᓂᒃ ᑐᓂᓯᓗᒍ.", // Feedback
	AboutClose:       "ᒪᑕᐃᓗᒍ",                                                                       // Close

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound:   "ᐃᓚᖓ ᓇᓗᓇᐃᖅᑕᐅᙱᓚᖅ: %s",                      // File not found: %s
	ErrNoOutputFolder: "ᐅᐸᒃᑎᕕᒃ ᓄᓇᓕᖕᒥ ᓂᕈᐊᖅᑕᐅᓯᒪᙱᓚᖅ.",               // No output folder selected
	ErrFFmpegMissing:  "FFmpeg ᐃᓇᖏᕐᓯᒪᙱᓚᖅ ᅎᕝᕙᓘᓐᓃᑦ ᓇᓗᓇᐃᖅᑕᐅᔪᓐᓇᙱᓚᖅ.", // FFmpeg not installed
	ErrProcessFailed:  "ᐱᓕᕆᓂᖅ ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ: %s",                    // Process failed: %s
	ErrConfigLoad:     "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ ᐊᒐᒃᓯᓗᒍ ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ: %s",        // Failed to load config: %s
	ErrConfigSave:     "ᐋᖅᑭᒃᓱᐃᔾᔪᑎᑦ ᓴᐳᒻᒥᔭᐅᓗᒍ ᐊᔪᕈᓐᓃᓚᐅᖅᑐᖅ: %s",      // Failed to save config: %s
}

func init() {
	register("iu", iu)
}
