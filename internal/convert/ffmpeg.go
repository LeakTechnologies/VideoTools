package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// CRFForQuality returns the CRF value for a given quality preset
func CRFForQuality(q string) string {
	switch q {
	case "Draft (CRF 28)":
		return "28"
	case "High (CRF 18)":
		return "18"
	case "Lossless":
		return "0"
	default:
		return "23"
	}
}

// DetermineVideoCodec maps user-friendly codec names to FFmpeg codec names
func DetermineVideoCodec(cfg ConvertConfig) string {
	switch cfg.VideoCodec {
	case "H.264":
		if cfg.HardwareAccel == "nvenc" {
			return "h264_nvenc"
		} else if cfg.HardwareAccel == "qsv" {
			return "h264_qsv"
		} else if cfg.HardwareAccel == "videotoolbox" {
			return "h264_videotoolbox"
		}
		return "libx264"
	case "H.265":
		if cfg.HardwareAccel == "nvenc" {
			return "hevc_nvenc"
		} else if cfg.HardwareAccel == "qsv" {
			return "hevc_qsv"
		} else if cfg.HardwareAccel == "videotoolbox" {
			return "hevc_videotoolbox"
		}
		return "libx265"
	case "VP9":
		return "libvpx-vp9"
	case "AV1":
		return "libaom-av1"
	case "Copy":
		return "copy"
	default:
		return "libx264"
	}
}

// DetermineAudioCodec maps user-friendly codec names to FFmpeg codec names
func DetermineAudioCodec(cfg ConvertConfig) string {
	switch cfg.AudioCodec {
	case "AAC":
		return "aac"
	case "Opus":
		return "libopus"
	case "MP3":
		return "libmp3lame"
	case "FLAC":
		return "flac"
	case "Copy":
		return "copy"
	default:
		return "aac"
	}
}

// ProbeVideo uses ffprobe to extract metadata from a video file
func ProbeVideo(path string) (*VideoSource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Format struct {
			Filename   string `json:"filename"`
			Format     string `json:"format_long_name"`
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index        int    `json:"index"`
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			Duration     string `json:"duration"`
			BitRate      string `json:"bit_rate"`
			PixFmt       string `json:"pix_fmt"`
			SampleRate   string `json:"sample_rate"`
			Channels     int    `json:"channels"`
			AvgFrameRate string `json:"avg_frame_rate"`
			FieldOrder   string `json:"field_order"`
			Disposition  struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	src := &VideoSource{
		Path:        path,
		DisplayName: filepath.Base(path),
		Format:      utils.FirstNonEmpty(result.Format.Format, result.Format.FormatName),
	}
	if rate, err := utils.ParseInt(result.Format.BitRate); err == nil {
		src.Bitrate = rate
	}
	if durStr := result.Format.Duration; durStr != "" {
		if val, err := utils.ParseFloat(durStr); err == nil {
			src.Duration = val
		}
	}
	// Track if we've found the main video stream (not cover art)
	foundMainVideo := false
	var coverArtStreamIndex int = -1

	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			// Check if this is an attached picture (cover art)
			if stream.Disposition.AttachedPic == 1 {
				coverArtStreamIndex = stream.Index
				logging.Debug(logging.CatFFMPEG, "found embedded cover art at stream %d", stream.Index)
				continue
			}
			// Only use the first non-cover-art video stream
			if !foundMainVideo {
				foundMainVideo = true
				src.VideoCodec = stream.CodecName
				src.FieldOrder = stream.FieldOrder
				if stream.Width > 0 {
					src.Width = stream.Width
				}
				if stream.Height > 0 {
					src.Height = stream.Height
				}
				if dur, err := utils.ParseFloat(stream.Duration); err == nil && dur > 0 {
					src.Duration = dur
				}
				if fr := utils.ParseFraction(stream.AvgFrameRate); fr > 0 {
					src.FrameRate = fr
				}
				if stream.PixFmt != "" {
					src.PixelFormat = stream.PixFmt
				}
			}
			if src.Bitrate == 0 {
				if br, err := utils.ParseInt(stream.BitRate); err == nil {
					src.Bitrate = br
				}
			}
		case "audio":
			if src.AudioCodec == "" {
				src.AudioCodec = stream.CodecName
				if rate, err := utils.ParseInt(stream.SampleRate); err == nil {
					src.AudioRate = rate
				}
				if stream.Channels > 0 {
					src.Channels = stream.Channels
				}
			}
		}
	}

	// Extract embedded cover art if present
	if coverArtStreamIndex >= 0 {
		coverPath := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-embedded-cover-%d.png", time.Now().UnixNano()))
		extractCmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", path,
			"-map", fmt.Sprintf("0:%d", coverArtStreamIndex),
			"-frames:v", "1",
			"-y",
			coverPath,
		)
		if err := extractCmd.Run(); err != nil {
			logging.Debug(logging.CatFFMPEG, "failed to extract embedded cover art: %v", err)
		} else {
			src.EmbeddedCoverArt = coverPath
			logging.Debug(logging.CatFFMPEG, "extracted embedded cover art to %s", coverPath)
		}
	}

	return src, nil
}
