package convert

import (
	"path/filepath"
	"testing"
)

func TestBuildJobConfigSnapshot(t *testing.T) {
	in := JobConfigInput{
		InputPath:         "neutral-input.mkv",
		OutputPath:        filepath.Join("neutral-output", "neutral.mp4"),
		OutputBase:        "neutral-output",
		SelectedFormat:    FormatOption{Ext: ".mp4", VideoCodec: "H.264"},
		Quality:           "High",
		Mode:              "Simple",
		PreserveChapters:  true,
		VideoCodec:        "H.264",
		AudioCodec:        "AAC",
		AudioBitrate:      "192k",
		NormalizeAudio:    true,
		NormalizeLUFS:     -16,
		NormalizeTruePeak: -1.5,
		AutoCrop:          false,
		CropWidth:         "10",
		CropHeight:        "10",
		CropX:             "0",
		CropY:             "0",
		SourceWidth:       720,
		SourceHeight:      480,
		SourceDuration:    12.5,
		SourceBitrate:     1000,
		SourceFrameRate:   29.97,
		FieldOrder:        "tt",
		AutoCompare:       true,
	}

	got := BuildJobConfig(in)
	if got["inputPath"] != in.InputPath || got["outputPath"] != in.OutputPath {
		t.Fatalf("paths not copied: %#v", got)
	}
	if got["outputDir"] != filepath.Dir(in.OutputPath) {
		t.Fatalf("outputDir = %#v", got["outputDir"])
	}
	if got["audioCodec"] != "AAC" || got["normalizeAudio"] != true {
		t.Fatalf("audio fields not copied: %#v", got)
	}
	if got["sourceWidth"] != 720 || got["fieldOrder"] != "tt" || got["autoCompare"] != true {
		t.Fatalf("source fields not copied: %#v", got)
	}
	if got["cropWidth"] != "" || got["cropX"] != "" {
		t.Fatalf("AutoCrop=false must clear crop fields: %#v", got)
	}

	in.AutoCrop = true
	got = BuildJobConfig(in)
	if got["cropWidth"] != "10" || got["cropHeight"] != "10" {
		t.Fatalf("AutoCrop=true must preserve crop fields: %#v", got)
	}
}
