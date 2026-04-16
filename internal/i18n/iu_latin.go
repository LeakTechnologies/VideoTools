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
	MenuQueue:           "Nuatausimaujut",             // Queue
	MenuBenchmark:       "Qaujisarniq",                // Benchmark
	MenuResults:         "Sivulliqpaujut",             // Results
	MenuAbout:           "Mikssannnut / Ikajuqtuiniq", // About / Support
	MenuLogs:            "Titiraqtausimaujut",         // Logs
	MenuFiles:           "Qinik",                      // Files (machine-generated, needs human review)
	MenuFilesOpen:       "Qinik ikajuqtunik...",       // Open Files... (machine-generated, needs human review)
	MenuFilesRecent:     "Qinik ilaqsimaujut",         // Recent Files (machine-generated, needs human review)
	MenuFilesOpenFolder: "Ilaggit qukiqsuallu",        // Open Output Folder (machine-generated, needs human review)
	MenuFilesAddMore:    "Qinik peqqittunik...",       // Add More Files (machine-generated, needs human review)
	MenuFilesGoTo:       "Aggiqsualuu %s",             // Go to %s (machine-generated, needs human review)

	// ── Module Names ─────────────────────────────────────────────────────
	ModuleConvert:     "Asijjiariaqtuq",   // Convert
	ModuleMerge:       "Katitataulugu",    // Merge
	ModuleTrim:        "Iggirrlugu",       // Trim
	ModuleFilters:     "Asikkaarutaujut",  // Filters
	ModuleUpscale:     "Pivaalliqtitiniq", // Upscale
	ModuleEnhancement: "Pivaalliqtitiniq", // Enhancement
	ModuleAudio:       "Nipik",            // Audio
	ModuleAuthor:      "Sanarlugu",        // Author/Create
	ModuleRip:         "Nirunnasilugu",    // Rip/Extract
	ModuleBurn:        "Qiniq",            // Burn

	// File Manager Module (machine-generated, needs human review)
	ModuleFileManager: "Iluqtuq", // File Manager
	ModuleBluRay:      "Blu-Ray",
	ModuleSubtitles:   "Titiqaksait",       // Subtitles
	ModuleThumbnail:   "Akiqalliat",        // Thumbnail
	ModuleCompare:     "Nalunaiqsiluggit",  // Compare
	ModuleInspect:     "Qaujisarlugu",      // Inspect
	ModulePlayer:      "Tarvijaksalirijuq", // Player
	ModuleSettings:    "Aaqiksuiniq",       // Settings

	// ── Dialog Titles ───────────────────────────────────────────────────────
	DialogCompare:            "Nalunaiqsiluggit Quliaqtuit",                                                                                                                                                                      // Compare Videos
	DialogInspect:            "Qaujisarlugu Quliaqtuit",                                                                                                                                                                          // Inspect Video
	DialogThumbnail:          "Akiqalliat Sanarlugu",                                                                                                                                                                             // Thumbnail Generation
	DialogSnippet:            "Ikajurlugu",                                                                                                                                                                                       // Snippet
	DialogInterlacing:        "Silgillirtilugu Asijjiariaqtuq",                                                                                                                                                                   // Interlacing Analysis
	DialogInterlacingResults: "Silgillirtilugu Asijjiariaqtuq Pijjugut",                                                                                                                                                          // Interlacing Analysis Results
	DialogAutoCrop:           "Aumiqsirtilugu",                                                                                                                                                                                   // Auto-Crop
	DialogAutoCropDetection:  "Aumiqsirtilugu Nalunaiqsi",                                                                                                                                                                        // Auto-Crop Detection
	DialogNoBlackBars:        "Annuraaktilugu Pijjut. Quliaqtuit Anniqsirtilugu.",                                                                                                                                                // No black bars detected
	DialogNoVideo:            "Quliaqtuit Annaganngittuq",                                                                                                                                                                        // No Video
	DialogNoFile:             "Santimi Annaganngittuq",                                                                                                                                                                           // No File
	DialogNoConfig:           "Aanniqsirtilugu Annaganngittuq",                                                                                                                                                                   // No Config
	DialogConfigSaved:        "Aanniqsirtilugu Sapummijugut",                                                                                                                                                                     // Config Saved
	DialogCopied:             "Agagsijugut",                                                                                                                                                                                      // Copied
	DialogNoLog:              "Santimi Annaganngittuq",                                                                                                                                                                           // No Log
	DialogQueued:             "Ilallijjugut",                                                                                                                                                                                     // Queued
	DialogCancelled:          "Atingninga",                                                                                                                                                                                       // Cancelled
	DialogBatchAdd:           "Ilallijjugut Aggiqsijjutilugu",                                                                                                                                                                    // Batch Add
	DialogRecovery:           "Ijunniaqtilugu Piujusirtilugu",                                                                                                                                                                    // Conversion Recovery
	DialogSnippets:           "Ikajurlugit",                                                                                                                                                                                      // Snippets
	DialogPreview:            "Akiqalliat Sanarlugu",                                                                                                                                                                             // Generating Preview
	DialogPlayback:           "Tarvijaksalirijut Piujusirtilugu",                                                                                                                                                                 // Playback Unavailable
	DialogJobQueued:          "Ilallijjugut Aggiqsijjutaujunik",                                                                                                                                                                  // Job Queued
	DialogMerge:              "Nutaaliurniq",                                                                                                                                                                                     // Merge
	DialogCancel:             "Atingninga",                                                                                                                                                                                       // Cancel
	DialogSnippetCreated:     "Ikajurlugu Pijjugut",                                                                                                                                                                              // Snippet Created
	DialogQueueNotInit:       "Ilallijjugut Pijuqsirtilugu",                                                                                                                                                                      // Queue not initialized
	DialogNoRunningJob:       "Atingniaqtilugu Aggiqsijjutaujunik",                                                                                                                                                               // No running job to cancel
	DialogISOCannotOpen:      "ISO Taikhanik",                                                                                                                                                                                    // Open DVD ISO
	DialogISOCannotOpenMsg:   "ISO taikhanik pijaugijumaanangitclugu.\n\nPisuvgit Autor ippa.",                                                                                                                                   // DVD ISO files cannot be loaded directly
	DialogBurnComingSoon:     "DVD Kaggianik",                                                                                                                                                                                    // Burn DVD
	DialogBurnComingSoonMsg:  "DVD kaggianik pipkaignikpaujunik.",                                                                                                                                                                // DVD burning will be available in a future update
	DialogUpdateBlocked:      "Nutaaliqpallaarumasuq Nalinginnaanginnaq",                                                                                                                                                         // machine-generated, needs human review — means: software update is blocked/cannot proceed
	StatusUpdateBlockedByJob: "Suliaq malinngajumiittara. Suliait tamarmik isumalaurtinnagit nutaaliqpallaarumasuq aturlugit.\n\nNutaaliqpallaarumasuq nutaaliurniq pijariiqtitisariaqarpoq sulianik isumalaurtitsimannginnani.", // machine-generated, needs human review — means: a background job is running; wait before installing the update
	LabelSnippet:             "Ikajurlugu:",                                                                                                                                                                                      // Snippet:
	MergeStarted:             "Nutaaliurniq Pijjugut!",                                                                                                                                                                           // Merge started!

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:     "Asijjiariaqtuq",   // Converting
	CategoryInspect:     "Qaujisarniq",      // Inspecting
	CategoryDisc:        "Taikkuninga",      // Disc
	CategoryPlayback:    "Uqausiiq",         // Playback
	CategoryAdvanced:    "Pivaalliqtitiniq", // Advanced
	CategoryScreenshots: "Akiqalliat",       // Screenshots
	CategorySettings:    "Aaqiksuiniq",      // Settings

	// ── Common Actions ────────────────────────────────────────────────────
	ActionGenerate:     "Sanarlugu",                       // Generate
	ActionCancel:       "Atingninga",                      // Cancel
	ActionSave:         "Sapummijugu",                     // Save
	ActionLoad:         "Agaksillugu",                     // Load
	ActionReset:        "Utirviggilugu Sivulliqpaujumut",  // Reset
	ActionBrowse:       "Niruarlugu",                      // Browse
	ActionOpen:         "Ukpiriilugu",                     // Open
	ActionClose:        "Matailugu",                       // Close
	ActionBack:         "Utirlugu",                        // Back
	ActionAdd:          "Ilalliujjilugu",                  // Add
	ActionRemove:       "Aggitittautilugu",                // Remove
	ActionClear:        "Saqinngiitilugu",                 // Clear
	ActionClearAll:     "Saqinngiitilugu Tamarmik",        // Clear All
	ActionInstall:      "Inarngirviiggilugu",              // Install
	ActionUninstall:    "Nuqkarviggilugu",                 // Uninstall
	ActionStart:        "Pigiarlugu",                      // Start
	ActionStop:         "Nuqkarlugu",                      // Stop
	ActionPause:        "Nuqkarlugu Atausiinnarmik",       // Pause
	ActionResume:       "Utirlugu Pilirininnganut",        // Resume
	ActionDelete:       "Aggitittautilugu",                // Delete
	ActionRefresh:      "Nutaarviggilugu",                 // Refresh
	ActionApply:        "Atuliqlugu",                      // Apply
	ActionConfirm:      "Isumajaqarlugu",                  // Confirm
	ActionEdit:         "Asijjiilugu",                     // Edit
	ActionCopy:         "Tuktillugu",                      // Copy
	ActionExport:       "Upaktittilugu",                   // Export
	ActionImport:       "Isirrlugu Sanatausimaujuq",       // Import
	ActionCopyLog:      "Tuktillugu Titiraqtausimaujut",   // Copy Log
	ActionCopyMetadata: "Tuktillugu Tukisimatitijutit",    // Copy Metadata
	ActionLoadConfig:   "Agaksillugu Aaqiksijautit",       // Load Config
	ActionSaveConfig:   "Sapummijugu Aaqiksijautit",       // Save Config
	ActionViewQueue:    "Qaujisarlugu Nuatausimaujut",     // View Queue
	ActionAddToQueue:   "Nuatausimaujunut Ilalliujjilugu", // Add to Queue
	ActionLoadVideo:    "Agaksillugu Tarvijaksaq",         // Load Video
	ActionClearVideo:   "Saqinngiitilugu Tarvijaksaq",     // Clear Video

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelInput:         "Isirvik",                        // Input
	LabelOutput:        "Upaktiivik",                     // Output
	LabelSource:        "Naqituinanga",                   // Source
	LabelDestination:   "Tikiivik",                       // Destination
	LabelFormat:        "Aviktausimaninga",               // Format
	LabelQuality:       "Pivaalliqsimaninga",             // Quality
	LabelResolution:    "Nalunairrutauninnga",            // Resolution
	LabelBitrate:       "Uqausiirmut Akingit",            // Bitrate
	LabelFrameRate:     "Takujautiit Akiligaaksangit",    // Frame Rate
	LabelCodec:         "Sanajjuti",                      // Codec
	LabelAudio:         "Nipik",                          // Audio
	LabelVideo:         "Tarvijaksaq",                    // Video
	LabelSubtitles:     "Titiqaksait",                    // Subtitles
	LabelDuration:      "Akiligaaksanga",                 // Duration
	LabelSize:          "Angirninnga",                    // Size
	LabelProgress:      "Pilirijauninnga",                // Progress
	LabelStatus:        "Atuqtauninnga",                  // Status
	LabelLanguage:      "Uqausiiq",                       // Language
	LabelVersion:       "Titiraqsimaujuq",                // Version
	LabelLicense:       "Atuqniimut Anirraaq",            // License
	LabelNoFile:        "Ilanginnik Agaksisimanngillaq",  // No file loaded
	LabelNoVideoLoaded: "Tarvijaksaq Agaksisimanngillaq", // No video loaded
	LabelFileFmt:       "Ilanga: %s",                     // File: %s

	// ── Inspect Module ────────────────────────────────────────────────────────
	InspectInstructions:       "Tarvijaksaqmik Agaksillugu qaujigiarrlugu attanangittumikkullu. Upaksillugu uqqaatijaksaqmik atuqlugu.", // Load a video to inspect
	InspectLoadingPreview:     "Takujautiit agaktaujuq...",                                                                              // Loading preview...
	InspectNoPreviewAvailable: "Takujautiit takulaunnginnaq",                                                                            // No preview available

	// ── Audio Module ───────────────────────────────────────────────────────────
	AudioInstructions:    "Tarvijaksaqmik upaksillugu uqqaatijaksaqmik niruarlugu", // Drop video or click to browse
	AudioOutputFormat:    "Upaktiivik Aviktausimaninga:",                           // Output Format:
	AudioQualityPreset:   "Pivaalliqsimaninga Atuqtaujuksaq:",                      // Quality Preset:
	AudioBitrate:         "Uqausiirmut Akingit:",                                   // Bitrate:
	AudioOutputDirectory: "Upaktiivik Nunalingni:",                                 // Output Directory:
	AudioExtractNow:      "Apiriuti Atujujuq",                                      // Extract Now
	AudioAddToQueue:      "Qanikkarlugu nutaqqivikkut",                             // Add to Queue
	AudioBrowseForVideo:  "Tarvijaksaqmik niruarlugu",                              // Browse for Video
	AudioSelectAll:       "Nunaliqijaujuq",                                         // Select All
	AudioDeselectAll:     "Nunaliqijaujuq unik",                                    // Deselect All
	AudioBatchMode:       "Sarvimut upaktiivik (miksikup ilanginnik)",              // Batch mode
	AudioAddFiles:        "Ilanginnik niruarlugu",                                  // Add Files
	AudioClearFiles:      "Ukaivik",                                                // Clear
	AudioNormalization:   "EBU R128 pivaalliqsimaningaq pivalaviuqtijuq",           // Apply EBU R128 Normalization
	AudioTargetLUFS:      "Pivaalliqsimaninga LUFS:",                               // Target LUFS:
	AudioTruePeak:        "Pirrvinnirmut pivaalliqsimaninga:",                      // True Peak:
	AudioNoFilesAdded:    "Ilanginnik agaksisimanngillaq",                          // No files added
	AudioFormat:          "Upaktiivik",                                             // Format
	AudioQuality:         "Pivaalliqsimaninga",                                     // Quality
	AudioNormSection:     "Pivaalliqsimaningaq",                                    // Normalization
	AudioOutput:          "Upaktiiviknuttaqut",                                     // Output
	AudioNoTracksFound:   "Upaktiivik agaksisimanngillaq",                          // machine-generated, needs human review
	AudioErrLoadFile:     "Ilak utirviqsaaq agaksisimanngillaq",                    // machine-generated, needs human review
	AudioErrDetectTracks: "Upaktiivik aviktausimaninga agaksisimanngillaq",         // machine-generated, needs human review
	AudioErrNoFile:       "Ilanginnik agaksisimanngillaq",                          // machine-generated, needs human review
	AudioErrNoTracks:     "Upaktiivik agaksisimanngillaq niruarlugu",               // machine-generated, needs human review
	AudioErrNoBatchFiles: "Ilanginnik sarvinik agaksisimanngillaq",                 // machine-generated, needs human review
	AudioErrOutputDir:    "Upaktiivik nunalingni agaksisimanngillaq",               // machine-generated, needs human review

	// ── Filters Module ─────────────────────────────────────────────────────────
	FiltersInstructions:      "Asikkaarutaujunik atuliqlugu tarvijaksaqmut. Takujautiit uqausiirmut nutaarviggilugu.", // Apply filters
	FiltersVideoPreview:      "Tarvijaksaq Takujautiit",                                                               // Video preview
	FiltersSectionBrightness: "Qalamik, attunginnginnik, timmiujuniklu aaqiksuiluggit",                                // Adjust brightness etc.
	FiltersBrightness:        "Qalamik:",                                                                              // Brightness:
	FiltersContrast:          "Attunginnginnik:",                                                                      // Contrast:
	FiltersSaturation:        "Tarvijaksaq Timiujunik:",                                                               // Saturation:
	FiltersSectionSharpen:    "Nalunaiqunnarqtittilugu, tunisillugu, sulijuktittilugulu",                              // Sharpen, blur, denoise
	FiltersSharpness:         "Nalunaiqunnarqtittininga:",                                                             // Sharpness:
	FiltersDenoise:           "Sulijuktittiniq:",                                                                      // Denoise:
	FiltersSectionRotate:     "Utirlugu tasarlugulu Tarvijaksaq",                                                      // Rotate and flip video
	FiltersRotation:          "Utiruti:",                                                                              // Rotation:
	FiltersFlipHorizontal:    "Akigijaanga Aksugijangalu:",                                                            // Flip Horizontal:
	FiltersFlipVertical:      "Qulinga Taqqigangalu:",                                                                 // Flip Vertical:
	FiltersSectionArtistic:   "Sanangualutiit Atuliqlugu",                                                             // Apply artistic effects
	FiltersSectionEra:        "Tarvijaksait Sivullipaujisimaujut Atuliqlugu",                                          // Era-based effects
	FiltersEraMode:           "Tarvijaksaq Avata:",                                                                    // Era Mode:
	FiltersInterlacing:       "Akigijaanga Timiujunik:",                                                               // Interlacing:
	FiltersChromaNoise:       "Timiujunik Ilinniarningit:",                                                            // Chroma Noise:
	FiltersTapeNoise:         "Taikkuninga Ilinniarningit:",                                                           // Tape Noise:
	FiltersTrackingError:     "Pigiarluttauninnga Ajurunniilauqtuq:",                                                  // Tracking Error:
	FiltersTapeDropout:       "Taikkuninga Nuqkarniq:",                                                                // Tape Dropout:
	FiltersInterpHint:        "Ajigiinngittut atuqtaujut atuqniimut ikajuqtuijut.",                                    // Balanced preset recommended
	FiltersSectionMotion:     "Tarvijaksait nutinnianatit nutaat pilirijaksait sanarlugit",                            // Generate smoother motion
	FiltersPreset:            "Atuqtaujuksaq:",                                                                        // Preset:
	FiltersTargetFPS:         "Takujautiit Akiligaaksanga:",                                                           // Target FPS:

	// ── Subtitles Module ─────────────────────────────────────────────────
	SubtitlesOfflineHint:  "Silarsuarmit pissitsiviksaunngittuq STT ggml-small.bin atorlugu - API atorunnainnarsimanngittuq", // Offline STT uses bundled ggml-small.bin - no API needed
	SubtitlesEmpty:        "Qaliujaaqpait niruarnikkut upalungaijautigilugit",                                                // Drag and drop subtitle files here
	SubtitlesExtractEmbed: "Qaliujaaqpait ilitsirijauqataulutik nunarjuarmit",                                                // Extract embedded subtitle tracks
	SubtitlesOCROutput:    "OCR nalinginnaanngitsumik (ujarusimanngitsumik qaliujaaqpait angiqqanngitsorialik):",             // OCR output (image-based subtitles only):
	SubtitlesOCRLanguage:  "OCR uqausiq (tesseract, taima: eng, fra, iku):",                                                  // OCR language (tesseract, e.g. eng, fra, iku):
	SubtitlesShiftOffset:  "Qaliujaaqpait katimajariit asijjiqsimajunik uqariannginnagu (sikuniit):",                         // Shift all subtitle times by offset (seconds):
	SubtitlesStart:        "Pigiarmat",                                                                                       // Start
	SubtitlesEnd:          "Isumaqtaat",
	// Subtitles UI sections
	SubtitlesSources:       "Atuqtaujariit",                                                     // Sources
	SubtitlesRipSection:    "Qaliujaaqpait ilitsijauqataujariit nunallaangajariit",              // Rip Embedded Subtitles
	SubtitlesTimingSection: "Inuusuttumik Asijjiqsimajunik Akiligiinngikkaluarlugu",             // Timing Adjustment
	SubtitlesSTTSection:    "Silarsuarmit pissitsiviksaunngittuq Uqariannginnagu (whisper.cpp)", // Offline Speech-to-Text
	SubtitlesOutputSection: "Nalinginnaanngitsumik",                                             // Output
	SubtitlesStatusSection: "Ilitsirijauqataujariit",                                            // Status
	SubtitlesCuesSection:   "Qaliujaaqpait Uqausivisa",                                          // Subtitle Cues
	// Subtitles actions
	SubtitlesCopyStatus:      "Ilitsirijauqataujariit Nalinginnaanngitsumik",                                    // Copy Status
	SubtitlesAddCue:          "Uqausiq Atausiaq Ilitsirijauqataujariit",                                         // Add Cue
	SubtitlesLoadSubtitles:   "Qaliujaaqpait Nalunaarusiariit",                                                  // Load Subtitles
	SubtitlesSaveSubtitles:   "Qaliujaaqpait Ilinniaqtitaujariit",                                               // Save Subtitles
	SubtitlesGenerateSpeech:  "Uqariangajariit Piginnaaninngikkanginnagu (Silarsuarmit pissitsiviksaunngittuq)", // Generate From Speech
	SubtitlesDetectStreams:   "Isumalirijariit Nalunaarusiariit",                                                // Detect Streams
	SubtitlesExtractSelected: "Niruarnirmik Nalunaarusiariit",                                                   // Extract Selected
	SubtitlesApplyOffset:     "Sivuniksalirijariit Asijjiqsimajunik",                                            // Apply Offset
	SubtitlesCreateOutput:    "Nalinginnaanngitsumik Piginnaasimagaluarlugu",                                    // Create Output
	// Subtitles placeholders
	SubtitlesVideoPlaceholder:   "Tarvijaksaq isuma",                              // Video file path
	SubtitlesFilePlaceholder:    "Qaliujaaqpait (.srt, .vtt, .mks)",               // Subtitle file
	SubtitlesModelPlaceholder:   "Whisper atuqtaujariit isuma (ggml-*.bin)",       // Whisper model path
	SubtitlesBackendPlaceholder: "Whisper atuqtaujariit isuma (whisper.cpp/main)", // Whisper backend path
	SubtitlesOutputPlaceholder:  "Nalinginnaanngitsumik tarvijaksaq isuma",        // Output video path
	// Subtitles status/dynamic labels
	SubtitlesWhisperBackendFmt: "Whisper atuqtaujariit: %s",                                                // Whisper backend: %s
	SubtitlesWhisperModelFmt:   "Whisper atuqtaujariit: %s",                                                // Whisper model: %s
	SubtitlesOfflineModelHint:  "Silarsuarmit pissitsiviksaunngittuq STT niruarjariit ggml atuqtaujariit.", // Offline STT uses selected ggml model                                                                                      // End

	// ── Common Status ─────────────────────────────────────────────────────
	StatusReady:        "Pigiaqtauluni",                   // Ready
	StatusProcessing:   "Pilirijautajuq...",               // Processing
	StatusComplete:     "Isulilauqtuq",                    // Complete
	StatusFailed:       "Ajurunniiqtuq",                   // Failed
	StatusCancelled:    "Atingninga",                      // Cancelled
	StatusPending:      "Attuniq",                         // Pending
	StatusRunning:      "Pilirijautajuq...",               // Running...
	StatusUpToDate:     "Anulaaniqtuq",                    // Up to date
	StatusNoActiveJobs: "○ Pilirijaujannarqtut Nuqkaqtut", // No active jobs

	// ── Trim ──────────────────────────────────────────────────────────────
	TrimInstructions:          "Tarvijaksaqmik agaksillugu iggirnirnnut aaqiksuilugu. Akiligaaksangit niruarlugit.",                                                                                                                                                                                                                               // Load video for trimming
	TrimInPoint:               "Isirvik",                                                                                                                                                                                                                                                                                                          // In Point
	TrimOutPoint:              "Anigiurvik",                                                                                                                                                                                                                                                                                                       // Out Point
	TrimSetIn:                 "Isirvik Aaqiksuilugu",                                                                                                                                                                                                                                                                                             // Set In
	TrimSetOut:                "Anigiurvik Aaqiksuilugu",                                                                                                                                                                                                                                                                                          // Set Out
	TrimClear:                 "Saqinngiitilugit Aaqiksijautit",                                                                                                                                                                                                                                                                                   // Clear Points
	TrimMode:                  "Iggirnirnnut Turanganga:",                                                                                                                                                                                                                                                                                         // Trim Mode:
	TrimModeKeep:              "Sapummiilugu Annuraaksimajuq",                                                                                                                                                                                                                                                                                     // Keep Region
	TrimModeCut:               "Iggirrlugu Annuraaksimajuq",                                                                                                                                                                                                                                                                                       // Cut Region
	TrimPreview:               "Takujautiit Iggirrniq",                                                                                                                                                                                                                                                                                            // Preview Trim
	TrimSmartCopy:             "Ilisarijaullugu Tuktillugu (Sukkaittuumik)",                                                                                                                                                                                                                                                                       // Smart Copy (Fast)
	TrimRecode:                "Nutaarviggilugu Titiraqtauninnga (Angirningani Atuqtitaulugu)",                                                                                                                                                                                                                                                    // Re-encode
	TrimSegment:               "Aviktaujuq",                                                                                                                                                                                                                                                                                                       // Segment
	TrimAddSegment:            "Aviktaujumik Ilalliujjilugu",                                                                                                                                                                                                                                                                                      // Add Segment
	TrimExportSegments:        "Avilugit Pijjutaulutik Asinngiit Ilangit",                                                                                                                                                                                                                                                                         // Export as separate files
	TrimInvalidSelection:      "Pijjutaulutik Asinngilirit",                                                                                                                                                                                                                                                                                       // Invalid Selection
	TrimOutput:                "Upaktiivik",                                                                                                                                                                                                                                                                                                       // Output
	TrimDuration:              "Tunngeq",                                                                                                                                                                                                                                                                                                          // Duration
	TrimSmartCopyWarningTitle: "Ilisarijaullugu Tuktillugu Aaqsuilugu",                                                                                                                                                                                                                                                                            // Smart Copy Warning
	TrimSmartCopyWarning:      "Ilisarijaullugu Tuktillugu (sukkaittuumik, titiraqtauninnga) aqtarvigi unnummata aviktaujumut.\n\nUva uvattaulutik upaktiivik inniaqtuit aviktaujunnut aqtarvigi unnummata, tikinnialaurat tikinnialaurat.\n\nAglingnirtuq unnummata tikinniagviannut, aqtarvigi titiraqtaulutik.\n\nIlisarijaullugu Tuktillugu?", // Smart Copy Warning
	TrimJobAdded:              "Ilallijjugut Ikajurlugu Aggiqsijjutaujunik",                                                                                                                                                                                                                                                                       // Trim job added to queue

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:                "Aaqiksuiniq",
	SettingsTabGeneral:           "Tamainnut Atuqnirmut",                                  // General
	SettingsTabPreferences:       "Aaqiksuiniq",                                           // Preferences
	SettingsTabDependencies:      "Pijariatujut",                                          // Dependencies
	SettingsTabBenchmark:         "Qaujisarniq",                                           // Benchmark
	SettingsTabUpdates:           "Nutaaliurniq",                                          // Updates
	SettingsTabAbout:             "Mikssannnut",                                           // About
	SettingsTheme:                "Takujautiit",                                           // Theme
	SettingsOutputFolder:         "Upaktiivik Nunalingni",                                 // Output Folder
	SettingsLanguage:             "Uqausiiq",                                              // Language
	SettingsLanguageScript:       "Titirauusiit",                                          // Script
	SettingsScriptSyllabics:      "Qaniujaaqpait",                                         // Traditional Syllabics
	SettingsScriptLatin:          "Qaliujaaqpait",                                         // Latin
	SettingsAppPreferences:       "Atuqnirmut Aaqiksuiniq",                                // Application Preferences
	SettingsHWDecode:             "Sanajaksanik Takujautiit Niruqtillugu",                 // Hardware Video Decode
	SettingsHWDecodeHint:         "GPU niruqtilliqaugaqtuq. Saqqutuqtuinnatillugu.",       // machine-generated, needs human review
	SettingsMasterSettings:       "Aksuruqnarqtujut Aaqiksuiniq",                          // Master Settings
	SettingsHardwareAccel:        "Sanajaksanik Sukkaivagunnarqtut",                       // Hardware Acceleration (Global)
	SettingsDetect:               "Nalunaiqsilugu",                                        // Detect
	SettingsUseAuto:              "Aulatautitaulugu",                                      // Use Auto
	SettingsDetectedFmt:          "Nalunaiqtaulauqtuq: %s",                                // Detected: %s
	SettingsModuleVisibility:     "Takujaujannarqtut",                                     // Module Visibility
	SettingsModuleVisibilityHint: "Takujaujannarqtut isirviŋmut takujauvaaqtut.",          // Module visibility applies on the main menu.
	SettingsShowUpscale:          "Saqquiilugu Pivaalliqtitiniq",                          // Show Upscale module
	SettingsShowDisc:             "Saqquiilugu Taikkuninga (Sanarlugu & Nirunnasilugu)",   // Show Disc category
	SettingsQueueSection:         "Qanuriliurniq Nalinginnaavaa",                          // Queue Behaviour
	SettingsQueuePlayLabel:       "Takujautiit Atuqtariaqarmat:",                          // Play Video opens in:
	SettingsQueuePlayInspect:     "Qaimanngissitsivik",                                    // Inspect Module
	SettingsQueuePlaySystem:      "Takujautiit Manirajaq",                                 // Player Module
	SettingsQueuePlayHint:        "Niruarlugu taikkuninga 'Takujautiit' atuqtariaqarmat.", // Choose which module Play Video navigates to.

	// ── Settings — Output Defaults ────────────────────────────────────────────
	SettingsDefaultOutputDir:     "Upaktivik Nalunaiqsimanngitsoq",                                                                   // machine-generated, needs human review — Default Output Directory
	SettingsDefaultOutputDirHint: "Allatgutilikkunnik atuqtauvoq upaktivik ililiriittinnagu. Tarvijaksaq ililiriitinnaago atuqlugu.", // machine-generated, needs human review

	SettingsFFmpegMissing:   "Pijariatituuq FFmpeg nalunngissimajanngilaq.\n\nInarngirviiggilugu pilirijutinik atuqtariaqarmat.\n\nInarngirviininga:\n%LOCALAPPDATA%\\VideoTools\\bin",
	SettingsInstallFFmpeg:   "Inarngirviiggilugu FFmpeg Maanna",                     // Install FFmpeg Now
	SettingsOpenSettings:    "Ukpiriilugu Aaqiksuiniq",                              // Open Settings
	SettingsContinueLimited: "Kajusiilugu Ajurunniiqutauninnga",                     // Continue Limited Mode
	SettingsOpenReleases:    "Ukpiriilugu Nutausimaujut Makpigaq",                   // Open Releases Page
	SettingsUpdatesInfo:     "Qaujisarlugu Atuqlullu Nutaat.",                       // Check for updates and manage app updates.
	SettingsUpdatesAutoInfo: "Aulatautitaulugu Qaujisarniq Nutaat Aaqiksimaujumit.", // Checks automatically based on schedule.

	// ── Updates ───────────────────────────────────────────────────────────
	UpdateCheckButton:       "Qaujisarlugu Nutaat",                                                                                                                                // Check for Updates
	UpdateUpToDate:          "Anulaaniqtuq atuqtitaujuq (%s).",                                                                                                                    // Latest version
	UpdateAvailable:         "Nutaaq tunijaujannarqtuq: %s\n\nMaanna %s atuqtitaujuq.\n\nInarngirviiggilugu nutaaq atuliqlugu.",                                                   // New version available
	UpdateInstall:           "Inarngirviiggilugu Nutaaq",                                                                                                                          // Install Update
	UpdatePatchesAvailable:  "Maanna %s atuqtitaujuq.\n\nAsijjiiqtausimaujuq nalunnaiqtaulauqtuq:\n  Maanna: %s\n  Nutaaq: %s\n\nInarngirviiggilugu matuinnarautajut atuliqlugu.", // Patches available
	UpdateInstallPatches:    "Inarngirviiggilugu Matuinnarautajut",                                                                                                                // Install Patches
	UpdateHashMismatch:      "Asijjiiqtausimaujuq nalunnaiqtaulauqtuq:",                                                                                                           // Hash mismatch:
	UpdateHashCurrent:       "Maanna:",                                                                                                                                            // Current:
	UpdateHashLatest:        "Nutaaq:",                                                                                                                                            // Latest:
	UpdateCurrentVersion:    "Maannaujuq Titiraqsimaujuq:",                                                                                                                        // Current Version:
	UpdateVersionHash:       "Tukisimatitijuq:",                                                                                                                                   // Version Hash:
	UpdateChecking:          "Qaujisarniaqtuq Nutaat...",                                                                                                                          // Checking for updates...
	UpdateError:             "Tunisijunnailaursimajuq nutaat qaujisarniaqtaunirmut.",                                                                                              // Could not reach update server.
	UpdateAvailableFmt:      "● Nutaaq Tunijaujannarqtuq: %s (%s)",                                                                                                                // Update available: %s (%s)
	UpdateNewBuildAvailable: "● Nutaaq Pilirijijautaujuq: %s (%s)",                                                                                                                // New build available: %s (%s)
	UpdateUpToDateFmt:       "✓ Anulaaniqtuq (%s, %s)",                                                                                                                            // Up to date (latest: %s, %s)
	UpdateAutoCheck:         "Aulatautitaulugu Qaujisaq:",                                                                                                                         // Auto-check:
	UpdateDisabled:          "Attuiniqanngittuq",                                                                                                                                  // Disabled
	UpdateEveryHour:         "Ikarramik Taggasami",                                                                                                                                // Every hour
	UpdateEvery2Hours:       "2-nik Ikarranik",                                                                                                                                    // Every 2 hours
	UpdateEvery3Hours:       "3-nik Ikarranik",                                                                                                                                    // Every 3 hours
	UpdateEvery4Hours:       "4-nik Ikarranik",                                                                                                                                    // Every 4 hours
	UpdateEvery6Hours:       "6-nik Ikarranik",                                                                                                                                    // Every 6 hours
	UpdateEvery12Hours:      "12-nik Ikarranik",                                                                                                                                   // Every 12 hours
	UpdateDaily:             "Ullumi Taggasami",                                                                                                                                   // Daily
	UpdateSemiWeekly:        "3-nik Ullunik Taggasami",                                                                                                                            // Semi-weekly (every 3 days)
	UpdateWeekly:            "Pinasuarusimi Taggasami",                                                                                                                            // Weekly
	UpdateBiWeekly:          "2-nik Pinasuarusinik Taggasami",                                                                                                                     // Bi-weekly (every 2 weeks)
	UpdateMonthly:           "Taqqimi Taggasami",                                                                                                                                  // Monthly
	UpdateBiMonthly:         "2-nik Taqqinik Taggasami",                                                                                                                           // Bi-monthly (every 2 months)
	UpdateAutoCheckDesc:     "Aulatautitaulugu qaujisaq nutaat aaqiksimaujumit.",                                                                                                  // Auto-checks based on schedule

	// ── Dependencies ─────────────────────────────────────────────────────
	DependenciesTitle:        "Pijariatujut Pilirijjutit",              // System Dependencies
	DependenciesDesc:         "Aulattiluggit VideoTools Pijariatujut.", // Manage dependencies
	DependenciesInstalled:    "Inarngirsimaujuq",                       // Installed
	DependenciesNotInstalled: "Inarngirsimanngillaq",                   // Not Installed
	DependenciesCore:         "Attarnarqtujuq Pijariatituuq",           // Core dependency
	DependenciesInstall:      "Inarngirviiggilugu",                     // Install
	DependenciesUninstall:    "Nuqkarviggilugu",                        // Uninstall
	DependenciesRefresh:      "Nutaarviggilugu Atuqtaujut",             // Refresh Status
	DependenciesRequiredBy:   "Pijariatitaujut:",                       // Required by:
	DependenciesInstallCmd:   "Inarngirviiggilugu: %s",                 // Install: %s

	// ── Benchmark ────────────────────────────────────────────────────────
	BenchmarkTitle:     "Qaujisarniq Sanajaksarmik",                                       // Hardware Benchmark
	BenchmarkDesc:      "Qaujisarlugu tarvijagalirijjutit pilirinningit atuqviginiaqtat.", // Test encoding performance
	BenchmarkRunButton: "Pigiarlugu Qaujisarniq Sanajaksarmik",                            // Run Hardware Benchmark
	BenchmarkRecent:    "Nirunnasiuunik Qaujisalauqtat",                                   // Recent Benchmarks

	// ── Queue ─────────────────────────────────────────────────────────────
	QueueTitle:                "Nuatausimaujut",                         // Queue
	QueueEmpty:                "Pilirijaksait Nuatausimaujut Nuqkaqtut", // No jobs in queue
	QueueInProgress:           "Pilirijautajuq",                         // In Progress
	QueueCompleted:            "Isulilauqtuq",                           // Completed
	QueueFailed:               "Ajurunniiqtauq",                         // Failed
	QueueJobRunning:           "Pilirijautajuq...",                      // Running...
	QueueJobPending:           "Attuniq",                                // Pending
	ActionQueueStart:          "Nuatausimaviuksaulauq",                  // Start Queue
	ActionQueuePauseAll:       "Attuniq Puttinnik",                      // Pause All
	ActionQueueResumeAll:      "Puttiqsaujungnik",                       // Resume All
	ActionQueueClearCompleted: "Isulilauqtuit Pivautaulauq",             // Clear Completed
	ActionQueueCancelAll:      "Kivfautait Tamarmik Anniqsijiit",        // Cancel All Jobs

	// ── History Sidebar ───────────────────────────────────────────────────
	HistoryTitle:     "Atulaaqtait",       // HISTORY
	HistoryNoEntries: "Ilangit Nuqkaqtut", // No entries

	// ── Convert ───────────────────────────────────────────────────────────
	ConvertDropPrompt:    "Tarvijaksaqmik Upaksillugu",      // Drop a video file here
	ConvertOutputFormat:  "Upaktiivik Aviktausimaninga",     // Output Format
	ConvertHardwareAccel: "Sanajaksanik Sukkaivagunnarqtut", // Hardware Acceleration

	ConvertReset:                  "Aqiitchauliruti",                                // Reset
	ConvertResetDefaults:          "Aqiitchauliruti Agguutaiqqanut",                 // Reset to Defaults
	ConvertSnippetLengthFmt:       "Igiqsiurniq Nalunaiqsirnirniq: %d sek",          // Snippet Length: %d seconds
	ConvertSnippetOutput:          "Igiqsiurniq Upaktiivik:",                        // Snippet Output:
	ConvertMatchSourceFormat:      "Anngatamaqsiurniq",                              // Match Source Format
	ConvertUseConvSettings:        "Ilaqtunik = Agguutaiqqanut Ukiuqsi",             // Unchecked = Use Conversion Settings
	ConvertSnippetHint:            "Igiqsiurniq sukkavinit nuliaqsailiniq.",         // Creates a clip centred on the timeline midpoint.
	ConvertSnippetOptions:         "Igiqsiurniq Agguutaiqqanut",                     // Snippet Options
	ConvertSnippetPosition:        "Igiqsiurniq Wiqditjunniq:",                      // Snippet Position:
	ConvertSnippetFromCurrent:     "Wiqditjunnik Anngatamaqsikjunniq",               // From Current Position
	ConvertSnippetFromCurrentHint: "Wiqditjunniq aulappianniak anngatamaqsikjunniq", // Uses playback position instead of midpoint
	ConvertGenerateSnippet:        "Igiqsiurniq Sakkutilik",                         // Generate Snippet
	ConvertGenerateAllSnippets:    "Igiqsiurniq Sakkutilik Allatgun",                // Generate All Snippets

	// ── Convert Box Titles & Actions (Issue #5) ───────────────────────────────
	ConvertSectionVideoEncoding:   "Tarvijaksaq Nutaqqutigijauniq",                                                            // machine-generated, needs human review — Video Encoding
	ConvertSectionAudioEncoding:   "Nipingit Nutaqqutigijauniq",                                                               // machine-generated, needs human review — Audio Encoding
	ConvertSectionOutput:          "Upaktivik",                                                                                // machine-generated, needs human review — Output
	ConvertSectionResolutionFPS:   "Piusilik Ammalu Tarvijaksait Naglingniqsaujait",                                           // machine-generated, needs human review — Resolution & Frame Rate
	ConvertSectionAspectRatio:     "Piusilik Katimajinga",                                                                     // machine-generated, needs human review — Aspect Ratio
	ConvertSectionAutoCrop:        "Aulappianniarniit Alianait Nanngiarlugu",                                                  // machine-generated, needs human review — Auto-Crop
	ConvertSectionTransformations: "Tarvijaksaq Allatgutilik",                                                                 // machine-generated, needs human review — Video Transformations
	ConvertSectionDeinterlacing:   "Tungavikkut Allatgutilik",                                                                 // machine-generated, needs human review — Deinterlacing
	ConvertActionStart:            "Allatgutilik",                                                                             // machine-generated, needs human review — Convert
	ConvertActionCancelJob:        "Piliriaksaq Atuqtauniit Nungutissavaa",                                                    // machine-generated, needs human review — Cancel Active Job
	ConvertCommandPreview:         "Aulajariarvingit Takujaujariaqanginnagu",                                                  // machine-generated, needs human review — Command Preview
	ConvertCoverArtLabel:          "Nipilik Nunaliarvingit",                                                                   // machine-generated, needs human review — Cover Art
	ConvertOutputFileFmt:          "Upaktivik ililiriit: %s",                                                                  // machine-generated, needs human review — Output file: %s
	ConvertDefaultPathFmt:         "Nalunaiqsimanngitsoq: %s",                                                                 // machine-generated, needs human review — Default: %s
	ConvertAnalyzeInterlacing:     "Tungavikkut Qaujimajaujariaqanginnagu",                                                    // machine-generated, needs human review — Analyze Interlacing
	ConvertAnalyzing:              "Qaujimajaujariaqanginnagu...",                                                             // machine-generated, needs human review — Analyzing...
	ConvertDetectCrop:             "Alianait Nanngiarlugu Nalunaiqsilugu",                                                     // machine-generated, needs human review — Detect Crop
	ConvertDetecting:              "Nalunaiqsimajarluni...",                                                                   // machine-generated, needs human review — Detecting...
	ConvertAspectRatioEntry:       "Katimajinga titirarlugu tamarmik 1.90 uvvalu 256:135.",                                    // machine-generated, needs human review — Enter a ratio like 1.90 or 256:135.
	ConvertRemuxHint:              "Tungavikkut allatgutilik: silattuqsimavoq. Nutaqqutigijauniq aulajariit atuqtaunngillat.", // machine-generated, needs human review — Remux mode: stream copy. Encoding controls are disabled.
	ConvertUseSystemTemp:          "Tamarmik Aulajariit Atuqlugit",                                                            // machine-generated, needs human review — Use System Temp
	ConvertUseDefault:             "Nalunaiqsimanngitsoq Atuqlugu",                                                            // machine-generated, needs human review — Use Default

	// Convert (machine-generated)
	ConvertAddAllToQueue:             "All to Queue",                                                                         // machine-generated
	ConvertAutoCropHint:              "Removes black bars to reduce file size",                                               // machine-generated
	ConvertAutoNameHint:              "Tokens: <actress>, <studio>, <scene>, <title>, <series>, <date>, <filename>",          // machine-generated
	ConvertBackgroundHint:            "Crop removes edges, Letterbox/Pillarbox adds black bars to fit.",                      // machine-generated
	ConvertCacheDirHint:              "Use an SSD for best performance. Leave blank to use system temp.",                     // machine-generated
	ConvertCompareAfter:              "Compare After",                                                                        // machine-generated
	ConvertCustomAspectHint:          "Custom aspect ratio in use.",                                                          // machine-generated
	ConvertEncoderPresetHint:         "Choose slower for better compression, faster for speed",                               // machine-generated
	ConvertFFmpegCommand:             "FFmpeg Command:",                                                                      // machine-generated
	ConvertFormat:                    "Format",                                                                               // machine-generated
	ConvertFrameLabel:                "Frame: %d",                                                                            // machine-generated
	ConvertFrameRate:                 "Frame Rate",                                                                           // machine-generated
	ConvertHideBatchSettings:         "Hide Batch Settings",                                                                  // machine-generated
	ConvertHwAccelHint:               "Auto picks the best available GPU path; if encode fails, switch to none (software).",  // machine-generated
	ConvertInspectHint:               "Load a clip to inspect its technical details.",                                        // machine-generated
	ConvertInterlaceAnalyzing:        "Analyzing interlacing... (first 500 frames)",                                          // machine-generated
	ConvertInterlaceInfo:             "Left: Original | Right: Deinterlaced",                                                 // machine-generated
	ConvertLoadVideoForCommand:       "Load a video to see the FFmpeg command.",                                              // machine-generated
	ConvertLoadVideoToConvert:        "Load a video to convert",                                                              // machine-generated
	ConvertNavNext:                   "Next",                                                                                 // machine-generated
	ConvertNavPrev:                   "Prev",                                                                                 // machine-generated
	ConvertOutputFilename:            "Output Filename",                                                                      // machine-generated
	ConvertOutputFolder:              "Output Folder",                                                                        // machine-generated
	ConvertReadyToConvert:            "Ready to convert",                                                                     // machine-generated
	ConvertSectionAspectHandling:     "Aspect Handling",                                                                      // machine-generated
	ConvertSectionAudioBitrate:       "Audio Bitrate",                                                                        // machine-generated
	ConvertSectionAudioChannels:      "Audio Channels",                                                                       // machine-generated
	ConvertSectionAudioCodec:         "Audio Codec",                                                                          // machine-generated
	ConvertSectionBitrateMode:        "Bitrate Mode",                                                                         // machine-generated
	ConvertSectionBitratePreset:      "Bitrate Preset",                                                                       // machine-generated
	ConvertSectionBitrateSimple:      "Bitrate (simple presets)",                                                             // machine-generated
	ConvertSectionCacheDir:           "Cache/Temp Directory",                                                                 // machine-generated
	ConvertSectionClipsToMerge:       "Clips to Merge",                                                                       // machine-generated
	ConvertSectionCustomAspect:       "Custom Aspect Ratio",                                                                  // machine-generated
	ConvertSectionDVDApect:           "DVD Aspect Ratio",                                                                     // machine-generated
	ConvertSectionEncoderPreset:      "Encoder Preset",                                                                       // machine-generated
	ConvertSectionEncoderSpeed:       "Encoder Speed/Quality",                                                                // machine-generated
	ConvertSectionFormat:             "Format",                                                                               // machine-generated
	ConvertSectionFrameRate:          "Frame Rate",                                                                           // machine-generated
	ConvertSectionHardwareAccel:      "Hardware Acceleration",                                                                // machine-generated
	ConvertSectionLogsDir:            "Logs Directory",                                                                       // machine-generated
	ConvertSectionManualBitrate:      "Manual Bitrate",                                                                       // machine-generated
	ConvertSectionManualCRF:          "Manual CRF",                                                                           // machine-generated
	ConvertSectionMetadata:           "Metadata",                                                                             // machine-generated
	ConvertSectionOutputFilename:     "Output Filename",                                                                      // machine-generated
	ConvertSectionOutputFolder:       "Output Folder",                                                                        // machine-generated
	ConvertSectionOutputOptions:      "Output Options",                                                                       // machine-generated
	ConvertSectionPixelFormat:        "Pixel Format",                                                                         // machine-generated
	ConvertSectionQualityPreset:      "Quality Preset",                                                                       // machine-generated
	ConvertSectionRotation:           "Rotation",                                                                             // machine-generated
	ConvertSectionTargetAspect:       "Target Aspect Ratio",                                                                  // machine-generated
	ConvertSectionTargetFileSize:     "Target File Size",                                                                     // machine-generated
	ConvertSectionTargetResolution:   "Target Resolution",                                                                    // machine-generated
	ConvertSectionVideoCodec:         "Video Codec",                                                                          // machine-generated
	ConvertSettingsInfo:              "Settings persist across videos. Change them anytime to affect all subsequent videos.", // machine-generated
	ConvertShowBatchSettings:         "Show Batch Settings",                                                                  // machine-generated
	ConvertSnippetGenerating:         "Generating %d-second snippet...",                                                      // machine-generated
	ConvertTabAdvanced:               "Advanced",                                                                             // machine-generated
	ConvertTabSimple:                 "Simple",                                                                               // machine-generated
	ConvertTargetAspectHint:          "Pick desired output aspect (default Source).",                                         // machine-generated
	ConvertTransformHint:             "Apply flips and rotation to correct video orientation",                                // machine-generated
	ConvertTwoPassNote:               "Two-pass encoding is ignored in CRF mode.",                                            // machine-generated
	ConvertVideoFormatsHint:          "MP4, MOV, MKV and more",                                                               // machine-generated
	ConvertVideoOfFmt:                "Video %d of %d",                                                                       // machine-generated
	ConvertViewLog:                   "View Log",                                                                             // machine-generated
	FileManagerOpenConvert:           "Open in Convert",                                                                      // machine-generated
	TooltipConvertAudio:              "Audio codec and settings",                                                             // machine-generated
	TooltipConvertCodec:              "Video codec (H.264, H.265, etc.)",                                                     // machine-generated
	TooltipConvertFormat:             "Output video format (MP4, MKV, etc.)",                                                 // machine-generated
	TooltipConvertFormatDevicePreset: "Device-optimized formats ensure compatibility",                                        // machine-generated
	TooltipConvertOutput:             "Output file location",                                                                 // machine-generated
	TooltipConvertQuality:            "Video quality preset (CRF value)",                                                     // machine-generated
	TooltipConvertStart:              "Start conversion",                                                                     // machine-generated

	// ── Compare ──────────────────────────────────────────────────────────────────
	CompareInstructions:    "Tarvijaksait Marruuk agaksillugik nalunaiqsilugit. Upaksillugik uqqaatijaksaqmik atuqlugu.",    // Load two videos
	CompareFullscreen:      "Angirningani Takujautiit",                                                                      // Fullscreen Compare
	CompareCopyReport:      "Tuktillugu Nalunaiqrutaujuq",                                                                   // Copy Comparison
	CompareHidePlayer:      "Takujautiit Nuqkarlugu",                                                                        // Hide Player
	CompareShowPlayer:      "Takujautiit Saqquiilugu",                                                                       // Show Player
	CompareLoadFile1:       "Agaksillugu Ilanga 1",                                                                          // Load File 1
	CompareLoadFile2:       "Agaksillugu Ilanga 2",                                                                          // Load File 2
	CompareFile1NotLoaded:  "Ilanga 1: Agaksisimanngillaq",                                                                  // File 1: Not loaded
	CompareFile2NotLoaded:  "Ilanga 2: Agaksisimanngillaq",                                                                  // File 2: Not loaded
	CompareFile1Fmt:        "Ilanga 1: %s",                                                                                  // File 1: %s
	CompareFile2Fmt:        "Ilanga 2: %s",                                                                                  // File 2: %s
	CompareFile1Info:       "Ilanga 1 Tukisimatitijutit",                                                                    // File 1 Info
	CompareFile2Info:       "Ilanga 2 Tukisimatitijutit",                                                                    // File 2 Info
	CompareBackToView:      "< Utirlugu Nalunaiqsivimmut",                                                                   // < BACK TO COMPARE
	ComparePlayBoth:        "▶ Tarvijaksait Marruuk Pigiarlugik",                                                            // ▶ Play Both
	ComparePauseBoth:       "⏸ Tarvijaksait Marruuk Nuqkarlugik",                                                            // ⏸ Pause Both
	CompareSyncTitle:       "Attungittumik Tarvijagalirijuq",                                                                // Synchronized Playback
	CompareSyncMsg:         "Attungittumik tarvijagalirijuq tunijauniaqtuq.\n\nMaanna, atuliqlugu atuqniimut pilirijjutit.", // Sync msg
	CompareSyncMsgShort:    "Attungittumik tarvijagalirijuq tunijauniaqtuq.",                                                // Sync msg short
	CompareSideInfo:        "Takujautiit avatilugit.",                                                                       // Side-by-side info
	CompareCopied:          "Tuktitaulauqtuq",                                                                               // Copied
	CompareCopiedMsg:       "Tukisimatitijutit tuktitaulauqtut",                                                             // Metadata copied
	CompareCopiedFileMsg:   "Tukisimatitijutit tuktitaulauqtut",                                                             // Metadata copied
	CompareNoVideosTitle:   "Tarvijaksait Nuqkaqtut",                                                                        // No Videos
	CompareNoVideosFSMsg:   "Tarvijaksait marruuk agaksillugik takujautiinnarmut.",                                          // Load two videos
	CompareNoVideosCopyMsg: "Tarvijaksaqmik agaksillugu tuktillugu tukisimatitijutit.",                                      // Load one video to copy

	// ── Player ───────────────────────────────────────────────────────────────────
	PlayerInstructions: "VT_Player - Tarvijaksanik tarvijagalirijuq tukisimatitijutinik atuqtittiluni.", // VT_Player

	// ── Rip ──────────────────────────────────────────────────────────────────────
	RipDropPrompt:       "DVD/ISO/VIDEO_TS Upaksillugu",                                                                     // Drop DVD/ISO/VIDEO_TS path
	RipOutputPath:       "Upaktiivik Akiligaaksanga",                                                                        // Output path
	RipSource:           "Naqituinanga",                                                                                     // Source
	RipFormatLabel:      "Aviktausimaninga",                                                                                 // Format
	RipLog:              "Nirunnasilunniq Titiraqtausimaujut",                                                               // Rip Log
	RipAddToQueue:       "Nuatausimaujunut Ilalliujjilugu",                                                                  // Add Rip to Queue
	RipNow:              "Maanna Nirunnasilugu",                                                                             // Rip Now
	RipClearISO:         "Saqinngiitilugu ISO",                                                                              // Clear ISO
	RipJobQueuedTitle:   "Nuatausimaujut",                                                                                   // Queue
	RipJobQueuedMsg:     "Nirunnasilunnirmut pilirijaksaq nuatausimaujunut ilalliujjitaulauqtuq.",                           // Rip job added
	RipStartTitle:       "Nirunnasilunniq",                                                                                  // Rip
	RipStartMsg:         "Nirunnasilunniq pigiaqtuq! Nuatausimaujut takulugu.",                                              // Rip started
	RipNoConfigTitle:    "Aaqiksijautit Nuqkaqtut",                                                                          // No Config
	RipNoConfigMsg:      "Aaqiksijautit sapummijausimaujut nalunnaiqtausimanngillat. Asijjiiqsimatitinninngi atulilaaqtuq.", // No saved config
	RipConfigSavedTitle: "Aaqiksijautit Sapummijausimaujut",                                                                 // Config Saved
	RipConfigSavedFmt:   "Sapummijausimaujuq %s-mut",                                                                        // Saved to %s
	RipErrNoSource:      "DVD/ISO/VIDEO_TS naqituinanga aaqiksuilugu",                                                       // set a source path
	RipCSSEncrypted:     "CSS Siuulliqpaujuq",                                                                               // CSS Encrypted
	RipCSSDecrypting:    "CSS Siuulliqpaujuq Ilusimajunga...",                                                               // Decrypting CSS...
	RipCSSNotEncrypted:  "Siuulliqpaugannuqqaqtut",                                                                          // Not Encrypted
	RipCSSStatusFmt:     "Siuulliqpauganngit: %s",                                                                           // Encryption: %s

	// ── Upscale ──────────────────────────────────────────────────────────────────
	UpscaleNow:             "Maanna Pivaalliqtitilugu",                                // UPSCALE NOW
	UpscaleStartedTitle:    "Pivaalliqtitiniq Pigiaqtuq",                              // Upscale Started
	UpscaleStartedFmt:      "Pivaalliqtitijautajuq %s-mut.\nNuatausimaujut takulugu.", // Upscaling to %s
	UpscaleAddedTitle:      "Nuatausimaujunut Ilalliujjitaulauqtuq",                   // Added to Queue
	UpscaleAddedFmt:        "Pivaalliqtitinirmut pilirijaksaq ilalliujjitaulauqtuq.\nTikiivik: %s, Atuqtaujutiit: %s",
	UpscaleSourceFmt:       "Naqituinanga: %dx%d",                                                                         // Source: %dx%d
	UpscaleSourceNA:        "Naqituinanga: Saqinngiittut",                                                                 // Source: N/A
	UpscaleMethodFmt:       "Atuqtaujutiit: %s",                                                                           // Method: %s
	UpscaleTargetFmt:       "Tikiivik: %s",                                                                                // Target: %s
	UpscaleBlurFmt:         "Immaqaa Attuninga: %.2f",                                                                     // Blur Strength: %.2f
	UpscaleEnableBlur:      "Attuninga Atuliqlugu",                                                                        // Enable Blur
	UpscaleAdjustFilters:   " Asikkaarutaujut Aaqiksuilugit",                                                              // Adjust Filters
	UpscaleAdjustFmt:       "Aaqiksijautit: %.2fx",                                                                        // Adjustment: %.2fx
	UpscaleDenoiseFmt:      "Sulijuktittiniq: %.2f",                                                                       // Denoise: %.2f
	UpscaleDenoiseAvail:    "Sulijuktittiniq tunijaujannarqtuq tiliramut",                                                 // Denoise available
	UpscaleDenoiseUnavail:  "Sulijuktittiniq tiliramut atuqunnarqtuq",                                                     // Denoise unavailable
	UpscaleFrameRateFmt:    "Takujautiit Akiligaaksanga: %s",                                                              // Frame Rate: %s
	UpscaleMotionInterp:    "Tarvijaksait Nutinniananat Atuliqlugit",                                                      // Use Motion Interpolation
	UpscaleMotionHint:      "Tarvijaksait nutinniananat nutaat pilirijaksait sanarlutik",                                  // Motion hint
	UpscaleAIEnabled:       "AI Pivaalliqtitiniq Atuliqlugu",                                                              // Use AI Upscaling
	UpscaleAIDetected:      "Real-ESRGAN nalunnaiqtaulauqtuq - pivaalliqsimaninga atuqunnarqtuq",                          // AI detected
	UpscaleAINotDetected:   "Real-ESRGAN nalunnaiqtausimanngillaq. Inarngirviiggilugu pivaalliqsimaninnganut:",            // AI not detected
	UpscaleAIPython:        "Python Real-ESRGAN nalunnaiqtaulauqtuq, kisiani ncnn atuqariaqartuq.",                        // Python detected
	UpscaleAIFallback:      "Atuqniimut sanajjutit atuqtauniaqtut.",                                                       // Fallback methods
	UpscaleAINote:          "Nalunaarutit: AI pivaalliqtitiniq sukkaittuunngillaq kisiani pivaalliqsimaninga angijuqsauq", // AI note
	UpscaleAIAdvanced:      "Aaqqiksauniarniq (ncnn atuqtaujutiit)",                                                       // Advanced
	UpscaleFaceEnhance:     "Tarvijaksaqmik Pivaalliqtitilugu (Python/GFPGAN Pijariatituuq)",                              // Face Enhancement
	UpscaleTTACheck:        "TTA Atuliqlugu (Sukkaittuunngillaq, Pivaalliqsimaninga Angijuqsauq)",                         // Enable TTA
	UpscaleBitrateHint:     "CRF pivaalliqsimaninga aaqiksuilugu; akiligaaksanga tarviggilugu angirningani.",              // Bitrate hint
	UpscaleClassicDesc:     "Atuqniimut sanajjutit - tamainni tunijaujannarqtut",                                          // Classic desc
	UpscaleOptionalBlur:    "Attuninga Atuqtaujuksaq",                                                                     // Optional blur
	UpscaleFilterIntHint:   "Asikkaarutaujut pivaalliqtitininnganut ilaanik atuliqlugit.",                                 // Filter hint
	UpscaleVideoBox:        "Tarvijaksaq",                                                                                 // Video
	UpscaleTargetResBox:    "Tikiivik Nalunairrutauninnga",                                                                // Target Resolution
	UpscaleEncodingBox:     "Tarvijaksaq Titiraqsimaninga",                                                                // Video Encoding
	UpscaleScalingBox:      "Angirningani Aaqiksuiluniq",                                                                  // Scaling
	UpscaleFrameRateBox:    "Takujautiit Akiligaaksanga",                                                                  // Frame Rate
	UpscaleFilterIntBox:    "Asikkaarutaujut Iliqusiqqaninga",                                                             // Filter Integration
	UpscaleAIBox:           "AI Pivaalliqtitiniq",                                                                         // AI Upscaling
	UpscaleResLabel:        "Nalunairrutauninnga:",                                                                        // Resolution:
	UpscaleVideoCodecLabel: "Tarvijaksaq Sanajjutit:",                                                                     // Video Codec:
	UpscaleEncoderLabel:    "Titiraqnirnnut Aaqiksijautit:",                                                               // Encoder Preset:
	UpscaleQualityLabel:    "Pivaalliqsimaninga Atuqtaujuksaq:",                                                           // Quality Preset:
	UpscaleBitrateLabel:    "Uqausiirmut Akingit Turanganga:",                                                             // Bitrate Mode:
	UpscaleTargetFPSLabel:  "Tikiivik Takujautiit Akiligaaksanga:",                                                        // Target FPS:
	UpscaleAIModelLabel:    "AI Atuqtaujutiit:",                                                                           // AI Model:
	UpscaleAIPresetLabel:   "Pilirijjutiit Atuqtaujuksaq:",                                                                // Processing Preset:
	UpscaleAIScaleLabel:    "Pivaalliqtitininnganut Takujautiit:",                                                         // Upscale Factor:
	UpscaleAITileLabel:     "Aviksimaujuq Angirninnga:",                                                                   // Tile Size:
	UpscaleAIOutputLabel:   "Upaktiivik Takujautiit:",                                                                     // Output Frames:
	UpscaleGPULabel:        "GPU:",                                                                                        // GPU:
	UpscaleThreadsLabel:    "Pilirijaksait (Agaksillugit/Pilirlugit/Sapummiilugit):",                                      // Threads:
	UpscaleFilterIntLabel:  "Asikkaarutaujut Iliqusiqqaninga:",                                                            // Filter Integration:
	UpscaleScalingLabel:    "Angirningani Aaqiksuininnganut Sanajjutit:",                                                  // Scaling Algorithm:

	// ── Frame Interpolation (RIFE) ────────────────────────────────────────────
	RIFEBoxTitle:        "Takujautiit Nutaat Sanarniq (RIFE)",                             // Frame Interpolation (RIFE)
	RIFEDetected:        "rife-ncnn-vulkan nalunnaiqtaulauqtuq",                           // rife detected
	RIFENotDetected:     "rife-ncnn-vulkan nalunnaiqtausimanngillaq. Inarngirviiggilugu:", // rife not detected
	RIFEEnabled:         "RIFE Takujautiit Nutaat Sanarniq Atuliqlugu",                    // Enable RIFE
	RIFEMultiplierLabel: "Amirsuqatigiit:",                                                // Multiplier:
	RIFEModelLabel:      "Atuqtaujutiit:",                                                 // Model:
	RIFEEstFPSFmt:       "Takujautiit Akiligaaksanga: %.0f fps",                           // Estimated output: %.0f fps
	RIFENote:            "RIFE takujautiit nutaat sanajuq tarvijaksaqmik akiligaaksamit.", // RIFE note
	RIFEInstallHint:     "Pissitsiurnikkut → Atuqtaujariit",                               // Install via Settings → Dependencies

	// ── Thumbnail / Contact Sheet ─────────────────────────────────────────
	ThumbnailGenerateNow:        "Pigiautilugu",                                                                      // GENERATE NOW
	ThumbnailContactSheet:       "Takujautiit Titiqaq",                                                               // Contact Sheet
	ThumbnailColumns:            "Atausiinnaqsimajut",                                                                // Columns
	ThumbnailRows:               "Atausiinnaqsimajut Akiligaaksangit",                                                // Rows
	ThumbnailOutputFolder:       "Upaktiivik Nunalingni",                                                             // Output Folder
	ThumbnailIndividual:         "Atausiinnaq Takujautiit",                                                           // Individual Thumbnails
	ThumbnailLoadVideo:          "Agaksillugu Tarvijaksaq",                                                           // Load Video
	ThumbnailNoFile:             "Ilanginnik Agaksisimanngillaq",                                                     // No file loaded
	ThumbnailFileLoaded:         "Ilanga: Tarvijaksaq agaksitaulauqtuq",                                              // File: video loaded
	ThumbnailInstructions:       "Tarvijaksaqmit takujautiit sanarlugit. Tarvijaksaqmik agaksillugu aaqiksuilugulu.", // Instructions
	ThumbnailContactSheetToggle: "Takujautiit Titiqaq Sanarlugu (Atausiinnaq Takujautiit)",                           // Contact sheet toggle
	ThumbnailShowTimestamps:     "Akiligaaksait Takujautinnit Saqquiilugit",                                          // Show timestamps
	ThumbnailContactSheetGrid:   "Takujautiit Titiqaq Katitigisimaujuq",                                              // Contact Sheet Grid
	ThumbnailOutputMode:         "Apigiiuqt Aviktausimaninga",                                                        // Output Mode // machine-generated, needs human review
	ThumbnailModeIndividual:     "Takujautinnit",                                                                     // Individual // machine-generated, needs human review
	ThumbnailModeContactSheet:   "Takujautiit Titiqaq",                                                               // Contact Sheet
	ThumbnailModeBoth:           "Takujautinnit Ama Titiqaq",                                                         // Both // machine-generated, needs human review
	ThumbnailSize:               "Takujautiit Angirninnga:",                                                          // Thumbnail Size:
	ThumbnailCountFmt:           "Takujautiit Amirsuqatigiit: %d",                                                    // Thumbnail Count: %d
	ThumbnailWidthFmt:           "Takujautiit Angirninnga: %d px",                                                    // Thumbnail Width: %d px
	ThumbnailColumnsFmt:         "Atausiinnaqsimajut: %d",                                                            // Columns: %d
	ThumbnailRowsFmt:            "Atausiinnaqsimajut Akiligaaksangit: %d",                                            // Rows: %d
	ThumbnailTotalFmt:           "Takujautiit Tamarmik: %d",                                                          // Total thumbnails: %d
	ThumbnailAddToQueue:         "Nuatausimaujunut Ilalliujjilugu",                                                   // Add to Queue
	ThumbnailAddAllToQueue:      "Tamarmik Nuatausimaujunut Ilalliujjilugit",                                         // Add All to Queue
	ThumbnailLoadedVideos:       "Agaksitausimaujut Tarvijaksait:",                                                   // Loaded Videos:
	ThumbnailVideoFmt:           "Tarvijaksaq %d",                                                                    // Video %d
	ThumbnailNoVideoTitle:       "Tarvijaksaq Nuqkaqtuq",                                                             // No Video
	ThumbnailNoVideoMsg:         "Tarvijaksaqmik agaksillugu sivulliqpaujumut.",                                      // Please load a video
	ThumbnailStartedTitle:       "Takujautiit",                                                                       // Thumbnails
	ThumbnailStartedMsg:         "Takujautiit sanarniq pigiaqtuq! Nuatausimaujut takulugu.",                          // Generation started
	ThumbnailJobQueuedTitle:     "Nuatausimaujut",                                                                    // Queue
	ThumbnailJobQueuedMsg:       "Takujautiit pilirijaksaq nuatausimaujunut ilalliujjitaulauqtuq!",                   // Job added
	ThumbnailNoVideosTitle:      "Tarvijaksait Nuqkaqtut",                                                            // No Videos
	ThumbnailNoVideosMsg:        "Tarvijaksait agaksillugit sivulliqpaujumut ilalliujjilugit.",                       // Load videos first
	ThumbnailJobsQueuedFmt:      "%d takujautiit pilirijaksait nuatausimaujunut ilalliujjitaulauqtut.",               // Queued %d jobs

	// ── About ─────────────────────────────────────────────────────────────
	AboutTitle:       "Mikssannnut VideoTools",                                                                                                       // About VideoTools
	AboutDescription: "Tarvijaksanik pilirijjutiit sukkaittuumik sanajautaulunillu.",                                                                 // A native video processing toolkit
	AboutLicense:     "Atuqniimut Anirraaq",                                                                                                          // License
	AboutSupport:     "Ikajuqtuiniq",                                                                                                                 // Support
	AboutLogsFolder:  "Titiraqtausimaujut Nunalingnni",                                                                                               // Logs Folder
	AboutScanForDocs: "Dev Builds",                                                                                                                   // Dev Builds
	AboutFeedback:    "Tusaajautitsijaksait: Titiraqtausimaujunik takulugit tarvijagalirijutinik; pijariaqartunik titiraqtausimaujunik tunisillugu.", // Feedback
	AboutClose:       "Matailugu",                                                                                                                    // Close

	// ── Errors ────────────────────────────────────────────────────────────
	ErrFileNotFound:   "Ilanga nalunnaiqtausimanngillaq: %s",                                        // File not found: %s
	ErrNoOutputFolder: "Upaktiivik nunalingni niruaqtausimanngillaq.",                               // No output folder selected
	ErrFFmpegMissing:  "FFmpeg inarngirsimanngillaq uqqaatijaksaqmilluunniit nalunnaiqtaunnginnaq.", // FFmpeg not installed
	ErrProcessFailed:  "Piliriniq ajurunniiqtaulauqtuq: %s",                                         // Process failed: %s
	ErrConfigLoad:     "Aaqiksijautit agaksillugit ajurunniiqtaulauqtuq: %s",                        // Failed to load config: %s
	ErrConfigSave:     "Aaqiksijautit sapummiijautilugit ajurunniiqtaulauqtuq: %s",                  // Failed to save config: %s
}

func init() {
	register("iu-Latn", iuLatn)
}
