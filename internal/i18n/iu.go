package i18n

// iu is the Inuktitut translation (Unified Canadian Aboriginal Syllabics).
// The Latin script variant shares these keys — the script toggle in Settings
// switches font rendering only, not the string content.
// Empty fields fall back to enCA automatically.
// Translation progress: see README for current percentage.
var iu = Strings{
	// ── App ──────────────────────────────────────────────────────────────
	AppTitle: "VideoTools",

	// ── Module Category Labels ────────────────────────────────────────────
	CategoryConvert:  "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ",  // Converting
	CategoryInspect:  "ᖃᐅᔨᓴᕐᓂᖅ",     // Inspecting
	CategoryDisc:     "ᑕᐃᑯᓂᖓ",        // Disc
	CategoryPlayback: "ᐅᖃᐅᓯᖅ",        // Playback

	// ── Common Actions ────────────────────────────────────────────────────
	ActionCancel: "ᐊᑎᖕᓂᖓ",   // Cancel
	ActionClose:  "ᒪᑕᐃᓗᒍ",   // Close
	ActionBack:   "ᐅᑎᕐᓗᒍ",   // Back
	ActionOpen:   "ᐅᒃᐱᕆᓗᒍ",  // Open
	ActionSave:   "ᓴᐳᒻᒥᔪᒍ",  // Save

	// ── Common Labels ─────────────────────────────────────────────────────
	LabelLanguage: "ᐅᖃᐅᓯᖅ", // Language
	LabelVersion:  "ᑎᑎᕋᖅᓯᒪᔪᖅ", // Version

	// ── Settings ──────────────────────────────────────────────────────────
	SettingsTitle:           "ᐋᖅᑭᒃᓱᐃᓂᖅ",       // Settings
	SettingsLanguage:        "ᐅᖃᐅᓯᖅ",           // Language
	SettingsLanguageScript:  "ᑎᑎᕋᐅᓯᖅ",          // Script
	SettingsScriptSyllabics: "ᖃᓂᐅᔮᖅᐸᐃᑦ",       // Traditional Syllabics
	SettingsScriptLatin:     "ᓚᑎᓐ",              // Latin

	// ── Status ────────────────────────────────────────────────────────────
	StatusReady:    "ᓴᐳᒻᒥᔪᖅ",      // Ready
	StatusComplete: "ᐃᓱᓕᓚᐅᖅᑐᖅ",  // Complete
	StatusFailed:   "ᐊᔪᕈᓐᓃᖅᑐᖅ",  // Failed
	StatusPending:  "ᐊᑦᑐᓂᖅ",      // Pending
}

func init() {
	register("iu", iu)
}
