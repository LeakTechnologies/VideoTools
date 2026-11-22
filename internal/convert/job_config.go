package convert

import "path/filepath"

// JobConfigInput is the complete, already-resolved input for one Convert queue
// job. Callers own source selection, output naming and codec adjustment; this
// package owns the serialized job snapshot.
type JobConfigInput struct {
	InputPath  string
	OutputPath string
	OutputBase string

	SelectedFormat   FormatOption
	Quality          string
	Mode             string
	PreserveChapters bool

	VideoCodec        string
	EncoderPreset     string
	CRF               string
	BitrateMode       string
	BitratePreset     string
	VideoBitrate      string
	TargetFileSize    string
	TargetResolution  string
	FrameRate         string
	PixelFormat       string
	HardwareAccel     string
	TwoPass           bool
	EncoderTune       string
	H264Profile       string
	H264Level         string
	Deinterlace       string
	DeinterlaceMethod string
	AutoCrop          bool
	CropWidth         string
	CropHeight        string
	CropX             string
	CropY             string
	FlipHorizontal    bool
	FlipVertical      bool
	Rotation          string

	AudioCodec        string
	AudioBitrate      string
	AudioChannels     string
	AudioSampleRate   string
	NormalizeAudio    bool
	NormalizeLUFS     float64
	NormalizeTruePeak float64

	InverseTelecine bool
	CoverArtPath    string
	AspectHandling  string
	OutputAspect    string
	ForceAspect     bool

	SourceWidth        int
	SourceHeight       int
	SampleAspectRatio  string
	DisplayAspectRatio string
	SourceDuration     float64
	SourceBitrate      int
	SourceFrameRate    float64
	FieldOrder         string

	AutoCompare bool
}

// BuildJobConfig serializes one immutable Convert job snapshot. A job owns the
// returned map; later UI changes must not mutate it.
func BuildJobConfig(in JobConfigInput) map[string]interface{} {
	cfg := map[string]interface{}{
		"inputPath":          in.InputPath,
		"outputPath":         in.OutputPath,
		"outputDir":          filepath.Dir(in.OutputPath),
		"outputBase":         in.OutputBase,
		"selectedFormat":     in.SelectedFormat,
		"quality":            in.Quality,
		"mode":               in.Mode,
		"preserveChapters":   in.PreserveChapters,
		"videoCodec":         in.VideoCodec,
		"encoderPreset":      in.EncoderPreset,
		"crf":                in.CRF,
		"bitrateMode":        in.BitrateMode,
		"bitratePreset":      in.BitratePreset,
		"videoBitrate":       in.VideoBitrate,
		"targetFileSize":     in.TargetFileSize,
		"targetResolution":   in.TargetResolution,
		"frameRate":          in.FrameRate,
		"pixelFormat":        in.PixelFormat,
		"hardwareAccel":      in.HardwareAccel,
		"twoPass":            in.TwoPass,
		"encoderTune":        in.EncoderTune,
		"h264Profile":        in.H264Profile,
		"h264Level":          in.H264Level,
		"deinterlace":        in.Deinterlace,
		"deinterlaceMethod":  in.DeinterlaceMethod,
		"autoCrop":           in.AutoCrop,
		"cropWidth":          in.CropWidth,
		"cropHeight":         in.CropHeight,
		"cropX":              in.CropX,
		"cropY":              in.CropY,
		"flipHorizontal":     in.FlipHorizontal,
		"flipVertical":       in.FlipVertical,
		"rotation":           in.Rotation,
		"audioCodec":         in.AudioCodec,
		"audioBitrate":       in.AudioBitrate,
		"audioChannels":      in.AudioChannels,
		"audioSampleRate":    in.AudioSampleRate,
		"normalizeAudio":     in.NormalizeAudio,
		"normalizeLUFS":      in.NormalizeLUFS,
		"normalizeTruePeak":  in.NormalizeTruePeak,
		"inverseTelecine":    in.InverseTelecine,
		"coverArtPath":       in.CoverArtPath,
		"aspectHandling":     in.AspectHandling,
		"outputAspect":       in.OutputAspect,
		"forceAspect":        in.ForceAspect,
		"sourceWidth":        in.SourceWidth,
		"sourceHeight":       in.SourceHeight,
		"sampleAspectRatio":  in.SampleAspectRatio,
		"displayAspectRatio": in.DisplayAspectRatio,
		"sourceDuration":     in.SourceDuration,
		"sourceBitrate":      in.SourceBitrate,
		"sourceFrameRate":    in.SourceFrameRate,
		"fieldOrder":         in.FieldOrder,
		"autoCompare":        in.AutoCompare,
	}
	if !in.AutoCrop {
		cfg["cropWidth"] = ""
		cfg["cropHeight"] = ""
		cfg["cropX"] = ""
		cfg["cropY"] = ""
	}
	return cfg
}
