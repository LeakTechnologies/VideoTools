package audio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// ExecuteOptions holds configuration for an audio extraction job.
type ExecuteOptions struct {
	InputPath        string
	OutputPath       string
	TrackIndex       int
	Format           string
	Bitrate          string
	Normalize        bool
	TargetLUFS       float64
	TruePeak         float64
	ProgressCallback func(float64)
}

// Execute performs an audio extraction job.
func Execute(ctx context.Context, opts ExecuteOptions) error {
	if opts.Normalize {
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(10.0)
		}
		params, err := analyzeLoudnorm(ctx, opts.InputPath, opts.TrackIndex, opts.TargetLUFS, opts.TruePeak)
		if err != nil {
			return fmt.Errorf("loudnorm analysis failed: %w", err)
		}
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(30.0)
		}
		return extractAudioWithNormalization(ctx, opts.InputPath, opts.OutputPath, opts.TrackIndex, opts.Format, opts.Bitrate, opts.TargetLUFS, opts.TruePeak, params, opts.ProgressCallback)
	}
	return extractAudioSimple(ctx, opts.InputPath, opts.OutputPath, opts.TrackIndex, opts.Format, opts.Bitrate, opts.ProgressCallback)
}

// ExecuteFromJob builds ExecuteOptions from a queue.Job and runs Execute.
func ExecuteFromJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		return fmt.Errorf("audio job config missing")
	}

	// Ensure output directory exists
	outputPath := job.OutputFile
	if outputDir := filepath.Dir(outputPath); outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	opts := ExecuteOptions{
		InputPath:        job.InputFile,
		OutputPath:       job.OutputFile,
		TrackIndex:       int(cfg["trackIndex"].(float64)),
		Format:           cfg["format"].(string),
		Bitrate:          cfg["bitrate"].(string),
		Normalize:        cfg["normalize"].(bool),
		ProgressCallback: progressCallback,
	}
	if opts.Normalize {
		opts.TargetLUFS = cfg["targetLUFS"].(float64)
		opts.TruePeak = cfg["truePeak"].(float64)
	}
	logging.Debug(logging.CatFFMPEG, "Audio extraction: track %d from %s to %s (format: %s, bitrate: %s, normalize: %v)",
		opts.TrackIndex, opts.InputPath, opts.OutputPath, opts.Format, opts.Bitrate, opts.Normalize)
	if err := Execute(ctx, opts); err != nil {
		return err
	}
	if progressCallback != nil {
		progressCallback(100.0)
	}
	logging.Debug(logging.CatFFMPEG, "Audio extraction completed: %s", opts.OutputPath)
	return nil
}

// ProbeAudioTracks returns all audio tracks in the given file.
func ProbeAudioTracks(path string) ([]TrackInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := utils.CreateCommand(ctx, utils.GetFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "a",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result struct {
		Streams []struct {
			Index       int                    `json:"index"`
			CodecName   string                 `json:"codec_name"`
			Channels    int                    `json:"channels"`
			SampleRate  string                 `json:"sample_rate"`
			BitRate     string                 `json:"bit_rate"`
			Duration    string                 `json:"duration"`
			Tags        map[string]interface{} `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
			} `json:"disposition"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var tracks []TrackInfo
	for _, stream := range result.Streams {
		track := TrackInfo{
			Index:    stream.Index,
			Codec:    stream.CodecName,
			Channels: stream.Channels,
			Default:  stream.Disposition.Default == 1,
		}
		if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
			track.SampleRate = sr
		}
		if br, err := strconv.Atoi(stream.BitRate); err == nil {
			track.Bitrate = br
		}
		if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
			track.Duration = dur
		}
		if lang, ok := stream.Tags["language"].(string); ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"].(string); ok {
			track.Title = title
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// GetAudioFileExtension returns the file extension for the given format.
func GetAudioFileExtension(format string) string {
	switch format {
	case "MP3":
		return "mp3"
	case "AAC":
		return "m4a"
	case "FLAC":
		return "flac"
	case "WAV":
		return "wav"
	default:
		return "mp3"
	}
}

// loudnormParams holds measured values from loudnorm analysis.
type loudnormParams struct {
	MeasuredI      float64
	MeasuredTP     float64
	MeasuredLRA    float64
	MeasuredThresh float64
}

func analyzeLoudnorm(ctx context.Context, inputPath string, trackIndex int, targetI, targetTP float64) (*loudnormParams, error) {
	args := []string{
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
		"-af", fmt.Sprintf("loudnorm=I=%.1f:TP=%.1f:print_format=json", targetI, targetTP),
		"-f", "null",
		"-",
	}

	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	logging.Debug(logging.CatFFMPEG, "Loudnorm analysis: %s %v", utils.GetFFmpegPath(), args)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "Loudnorm analysis output: %s", string(output))
		return nil, fmt.Errorf("loudnorm analysis failed: %w", err)
	}

	outputStr := string(output)
	jsonStart := strings.Index(outputStr, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON output from loudnorm")
	}
	jsonData := outputStr[jsonStart:]
	jsonEnd := strings.LastIndex(jsonData, "}")
	if jsonEnd == -1 {
		return nil, fmt.Errorf("malformed JSON output from loudnorm")
	}
	jsonData = jsonData[:jsonEnd+1]

	var result struct {
		InputI      string `json:"input_i"`
		InputTP     string `json:"input_tp"`
		InputLRA    string `json:"input_lra"`
		InputThresh string `json:"input_thresh"`
	}
	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		logging.Debug(logging.CatFFMPEG, "Failed to parse JSON: %s", jsonData)
		return nil, fmt.Errorf("failed to parse loudnorm JSON: %w", err)
	}

	params := &loudnormParams{}
	params.MeasuredI, _ = strconv.ParseFloat(result.InputI, 64)
	params.MeasuredTP, _ = strconv.ParseFloat(result.InputTP, 64)
	params.MeasuredLRA, _ = strconv.ParseFloat(result.InputLRA, 64)
	params.MeasuredThresh, _ = strconv.ParseFloat(result.InputThresh, 64)

	logging.Debug(logging.CatFFMPEG, "Loudnorm measured: I=%.2f, TP=%.2f, LRA=%.2f, thresh=%.2f",
		params.MeasuredI, params.MeasuredTP, params.MeasuredLRA, params.MeasuredThresh)

	return params, nil
}

func extractAudioWithNormalization(ctx context.Context, inputPath, outputPath string, trackIndex int, format, bitrate string, targetI, targetTP float64, params *loudnormParams, progressCallback func(float64)) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
		"-af", fmt.Sprintf("loudnorm=I=%.1f:TP=%.1f:measured_I=%.2f:measured_TP=%.2f:measured_LRA=%.2f:measured_thresh=%.2f",
			targetI, targetTP, params.MeasuredI, params.MeasuredTP, params.MeasuredLRA, params.MeasuredThresh),
	}
	args = append(args, getAudioCodecArgs(format, bitrate)...)
	args = append(args, outputPath)
	return runFFmpegExtraction(ctx, args, progressCallback, 30.0, 100.0)
}

func extractAudioSimple(ctx context.Context, inputPath, outputPath string, trackIndex int, format, bitrate string, progressCallback func(float64)) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
	}
	args = append(args, getAudioCodecArgs(format, bitrate)...)
	args = append(args, outputPath)
	return runFFmpegExtraction(ctx, args, progressCallback, 0.0, 100.0)
}

func getAudioCodecArgs(format, bitrate string) []string {
	switch format {
	case "MP3":
		args := []string{"-c:a", "libmp3lame"}
		if bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
		return args
	case "AAC":
		args := []string{"-c:a", "aac"}
		if bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
		return args
	case "FLAC":
		return []string{"-c:a", "flac"}
	case "WAV":
		return []string{"-c:a", "pcm_s16le"}
	default:
		return []string{"-c:a", "copy"}
	}
}

func runFFmpegExtraction(ctx context.Context, args []string, progressCallback func(float64), startPct, endPct float64) error {
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	logging.Debug(logging.CatFFMPEG, "Running: %s %v", utils.GetFFmpegPath(), args)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logging.Debug(logging.CatFFMPEG, "FFmpeg: %s", line)
		if strings.Contains(line, "time=") && progressCallback != nil {
			progressCallback(startPct + (endPct-startPct)/2)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("FFmpeg failed: %w", err)
	}
	if progressCallback != nil {
		progressCallback(endPct)
	}
	return nil
}
