package convert

// FormatOptions is the canonical list of output format presets.
var FormatOptions = []FormatOption{
	// H.264 — widely compatible
	{Label: "MP4 (H.264)", Ext: ".mp4", VideoCodec: "libx264"},
	{Label: "MOV (H.264)", Ext: ".mov", VideoCodec: "libx264"},
	// Remux — no re-encode
	{Label: "MKV (Remux)", Ext: ".mkv", VideoCodec: "copy"},
	// H.265/HEVC — higher quality
	{Label: "MP4 (H.265)", Ext: ".mp4", VideoCodec: "libx265"},
	{Label: "MKV (H.265)", Ext: ".mkv", VideoCodec: "libx265"},
	{Label: "MOV (H.265)", Ext: ".mov", VideoCodec: "libx265"},
	// AV1 — best compression
	{Label: "MP4 (AV1)", Ext: ".mp4", VideoCodec: "libaom-av1"},
	{Label: "MKV (AV1)", Ext: ".mkv", VideoCodec: "libaom-av1"},
	{Label: "WebM (AV1)", Ext: ".webm", VideoCodec: "libaom-av1"},
	// VP9 — Google codec, good for web
	{Label: "WebM (VP9)", Ext: ".webm", VideoCodec: "libvpx-vp9"},
	// ProRes — professional/editing
	{Label: "MOV (ProRes)", Ext: ".mov", VideoCodec: "prores_ks"},
	// MPEG-2 — DVD standard
	{Label: "DVD-NTSC (MPEG-2)", Ext: ".mpg", VideoCodec: "mpeg2video"},
	{Label: "DVD-PAL (MPEG-2)", Ext: ".mpg", VideoCodec: "mpeg2video"},
	// AVI — legacy Windows compatibility
	{Label: "AVI (H.264)", Ext: ".avi", VideoCodec: "libx264", Legacy: true},
	// TS — broadcast and streaming
	{Label: "TS (H.264)", Ext: ".ts", VideoCodec: "libx264"},
	{Label: "TS (MPEG-2)", Ext: ".ts", VideoCodec: "mpeg2video"},
	// FLV — legacy Flash Video (H.264 + AAC/MP3)
	{Label: "FLV (H.264)", Ext: ".flv", VideoCodec: "libx264", Legacy: true},
	// 3GP — legacy mobile (3G phones)
	{Label: "3GP (H.264)", Ext: ".3gp", VideoCodec: "libx264", Legacy: true},
	// OGG — legacy open container (Theora video + Vorbis audio)
	{Label: "OGG (Theora)", Ext: ".ogv", VideoCodec: "libtheora", Legacy: true},
}

// HWPreset is a complete device-optimised encoding configuration.
// Selecting a preset applies all fields to convert settings at once.
type HWPreset struct {
	Label         string
	FormatLabel   string // must match a Label in FormatOptions
	EncoderPreset string // e.g., "medium", "fast"
	Quality       string // e.g., "High (CRF 18)"
	H264Profile   string // "baseline", "main", "high"
	H264Level     string // "4.0", "4.1", "5.1"; "" = leave unset
	TargetRes     string // "Source", "720p", "1080p", "4K"
	PixelFormat   string // "yuv420p"
	AudioCodec    string // "AAC", "AC-3"
	AudioBitrate  string // "128k", "192k", "256k"
	AudioChannels string // "Source", "Stereo"
	AllowHEVC     bool
	AllowAV1      bool
}

// HWPresets is the canonical list of device-optimised encoding presets.
var HWPresets = []HWPreset{
	{"iPhone", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "4.0", "1080p", "yuv420p", "AAC", "192k", "Stereo", true, false},
	{"Android", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", true, true},
	{"Chromecast", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", true, true},
	{"Fire TV", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", true, false},
	{"Smart TV", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", true, false},
	{"PlayStation 4", "MP4 (H.264)", "medium", "High (CRF 18)", "main", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", false, false},
	{"PlayStation 5", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "5.1", "4K", "yuv420p", "AAC", "256k", "Stereo", true, false},
	{"Xbox One", "MP4 (H.264)", "medium", "High (CRF 18)", "main", "4.1", "1080p", "yuv420p", "AAC", "192k", "Stereo", false, false},
	{"Xbox Series X", "MP4 (H.264)", "medium", "High (CRF 18)", "high", "5.1", "4K", "yuv420p", "AAC", "256k", "Stereo", true, false},
	{"Nintendo Switch", "MP4 (H.264)", "medium", "Standard (CRF 23)", "main", "4.1", "720p", "yuv420p", "AAC", "192k", "Stereo", false, false},
	{"Web (Fast Start)", "MP4 (H.264)", "fast", "Standard (CRF 23)", "high", "", "Source", "yuv420p", "AAC", "128k", "Stereo", false, false},
}

// FormatVideoCodecs maps format extension to compatible video codec friendly names.
var FormatVideoCodecs = map[string][]string{
	".mp4":  {"H.264", "H.265", "AV1", "MPEG-2", "Copy"},
	".mkv":  {"H.264", "H.265", "AV1", "VP9", "MPEG-2", "Copy"},
	".mov":  {"H.264", "H.265", "AV1", "MPEG-2", "Copy"},
	".webm": {"VP9", "AV1"},
	".mpg":  {"MPEG-2", "Copy"},
	".avi":  {"H.264", "Copy"},
	".ts":   {"H.264", "H.265", "MPEG-2", "Copy"},
	".flv":  {"H.264", "Copy"},
	".3gp":  {"H.264", "Copy"},
	".ogv":  {"Theora", "Copy"},
}

// FormatAudioCodecs maps format extension to compatible audio codec friendly names.
var FormatAudioCodecs = map[string][]string{
	".mp4":  {"AAC", "MP3", "AC-3", "FLAC", "Copy"},
	".mkv":  {"AAC", "MP3", "AC-3", "FLAC", "Opus", "Copy"},
	".mov":  {"AAC", "MP3", "AC-3", "FLAC", "Copy"},
	".webm": {"Opus", "Vorbis"},
	".mpg":  {"MP2", "AC-3", "Copy"},
	".avi":  {"MP3", "AAC", "AC-3", "Copy"},
	".ts":   {"AAC", "MP2", "AC-3", "Copy"},
	".3gp":  {"AAC", "AMR-NB", "Copy"},
	".ogv":  {"Vorbis", "Copy"},
}
