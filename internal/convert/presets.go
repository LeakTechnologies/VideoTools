package convert

// FormatOptions contains all available output format presets
var FormatOptions = []FormatOption{
	{Label: "MP4 (H.264)", Ext: ".mp4", VideoCodec: "libx264"},
	{Label: "MKV (H.265)", Ext: ".mkv", VideoCodec: "libx265"},
	{Label: "MOV (ProRes)", Ext: ".mov", VideoCodec: "prores_ks"},
	{Label: "DVD-NTSC (MPEG-2)", Ext: ".mpg", VideoCodec: "mpeg2video"},
}
