package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// Result stores the outcome of a single encoder benchmark test
type Result struct {
	Encoder      string  // e.g., "libx264", "h264_nvenc"
	Preset       string  // e.g., "fast", "medium"
	FPS          float64 // Encoding frames per second
	EncodingTime float64 // Total encoding time in seconds
	InputSize    int64   // Input file size in bytes
	OutputSize   int64   // Output file size in bytes
	PSNR         float64 // Peak Signal-to-Noise Ratio (quality metric)
	Score        float64 // Overall ranking score
	Error        string  // Error message if test failed
}

// Suite manages a complete benchmark test suite
type Suite struct {
	TestVideoPath string
	OutputDir     string
	FFmpegPath    string
	Results       []Result
	Progress      func(current, total int, encoder, preset string)
}

// NewSuite creates a new benchmark suite
func NewSuite(ffmpegPath, outputDir string) *Suite {
	return &Suite{
		FFmpegPath: ffmpegPath,
		OutputDir:  outputDir,
		Results:    []Result{},
	}
}

// GenerateTestVideo creates a short test video for benchmarking
// Returns path to test video
func (s *Suite) GenerateTestVideo(ctx context.Context, duration int) (string, error) {
	// Generate a 30-second 1080p test pattern video
	testPath := filepath.Join(s.OutputDir, "benchmark_test.mp4")

	// Use FFmpeg's testsrc to generate test video
	args := []string{
		"-f", "lavfi",
		"-i", "testsrc=duration=30:size=1920x1080:rate=30",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=30",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-c:a", "aac",
		"-y",
		testPath,
	}

	cmd := utils.CreateCommand(ctx, s.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate test video: %w", err)
	}

	s.TestVideoPath = testPath
	return testPath, nil
}

// UseTestVideo sets an existing video as the test file
func (s *Suite) UseTestVideo(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("test video not found: %w", err)
	}
	s.TestVideoPath = path
	return nil
}

// TestEncoder runs a benchmark test for a specific encoder and preset
func (s *Suite) TestEncoder(ctx context.Context, encoder, preset string) Result {
	result := Result{
		Encoder: encoder,
		Preset:  preset,
	}

	if s.TestVideoPath == "" {
		result.Error = "no test video specified"
		return result
	}

	// Get input file size
	inputInfo, err := os.Stat(s.TestVideoPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to stat input: %v", err)
		return result
	}
	result.InputSize = inputInfo.Size()

	// Output path
	outputPath := filepath.Join(s.OutputDir, fmt.Sprintf("bench_%s_%s.mp4", encoder, preset))
	defer os.Remove(outputPath) // Clean up after test

	// Build FFmpeg command
	args := []string{
		"-y",
		"-i", s.TestVideoPath,
		"-c:v", encoder,
	}

	// Add preset if not a hardware encoder with different preset format
	if preset != "" {
		switch {
		case encoder == "h264_nvenc" || encoder == "hevc_nvenc":
			// NVENC uses -preset with p1-p7
			args = append(args, "-preset", preset)
		case encoder == "h264_qsv" || encoder == "hevc_qsv":
			// QSV uses -preset
			args = append(args, "-preset", preset)
		case encoder == "h264_amf" || encoder == "hevc_amf":
			// AMF uses -quality
			args = append(args, "-quality", preset)
		default:
			// Software encoders (libx264, libx265)
			args = append(args, "-preset", preset)
		}
	}

	args = append(args, "-c:a", "copy", "-f", "null", "-")

	// Measure encoding time
	start := time.Now()
	cmd := utils.CreateCommand(ctx, s.FFmpegPath, args...)

	if err := cmd.Run(); err != nil {
		result.Error = fmt.Sprintf("encoding failed: %v", err)
		return result
	}

	elapsed := time.Since(start)
	result.EncodingTime = elapsed.Seconds()

	// Get output file size (if using actual output instead of null)
	// For now, using -f null for speed, so skip output size

	// Calculate FPS (need to parse from FFmpeg output or calculate from duration)
	// Placeholder: assuming 30s video at 30fps = 900 frames
	totalFrames := 900.0
	result.FPS = totalFrames / result.EncodingTime

	// Calculate score (FPS is primary metric)
	result.Score = result.FPS

	return result
}

// RunFullSuite runs all available encoder tests
func (s *Suite) RunFullSuite(ctx context.Context, availableEncoders []string) error {
	// Test matrix
	tests := []struct {
		encoder string
		presets []string
	}{
		{"libx264", []string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium"}},
		{"libx265", []string{"ultrafast", "superfast", "veryfast", "fast"}},
		{"h264_nvenc", []string{"fast", "medium", "slow"}},
		{"hevc_nvenc", []string{"fast", "medium"}},
		{"h264_qsv", []string{"fast", "medium"}},
		{"hevc_qsv", []string{"fast", "medium"}},
		{"h264_amf", []string{"speed", "balanced", "quality"}},
	}

	totalTests := 0
	for _, test := range tests {
		// Check if encoder is available
		available := false
		for _, enc := range availableEncoders {
			if enc == test.encoder {
				available = true
				break
			}
		}
		if available {
			totalTests += len(test.presets)
		}
	}

	current := 0
	for _, test := range tests {
		// Skip if encoder not available
		available := false
		for _, enc := range availableEncoders {
			if enc == test.encoder {
				available = true
				break
			}
		}
		if !available {
			continue
		}

		for _, preset := range test.presets {
			// Report progress before starting test
			if s.Progress != nil {
				s.Progress(current, totalTests, test.encoder, preset)
			}

			// Run the test
			result := s.TestEncoder(ctx, test.encoder, preset)
			s.Results = append(s.Results, result)

			// Increment and report completion
			current++
			if s.Progress != nil {
				s.Progress(current, totalTests, test.encoder, preset)
			}

			// Check for context cancellation
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}

	return nil
}

// GetRecommendation returns the best encoder based on benchmark results
func (s *Suite) GetRecommendation() (encoder, preset string, result Result) {
	if len(s.Results) == 0 {
		return "", "", Result{}
	}

	best := s.Results[0]
	for _, r := range s.Results {
		if r.Error == "" && r.Score > best.Score {
			best = r
		}
	}

	return best.Encoder, best.Preset, best
}

// GetTopN returns the top N encoders by score
func (s *Suite) GetTopN(n int) []Result {
	// Filter out errors
	valid := []Result{}
	for _, r := range s.Results {
		if r.Error == "" {
			valid = append(valid, r)
		}
	}

	// Sort by score (simple bubble sort for now)
	for i := 0; i < len(valid); i++ {
		for j := i + 1; j < len(valid); j++ {
			if valid[j].Score > valid[i].Score {
				valid[i], valid[j] = valid[j], valid[i]
			}
		}
	}

	if len(valid) > n {
		return valid[:n]
	}
	return valid
}
