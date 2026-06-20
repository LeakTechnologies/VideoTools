package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// perTestTimeout is the maximum time allowed for a single encoder test.
// Hardware encoder tests (AMF, NVENC, QSV) can hang indefinitely if the
// driver fails to initialize; this timeout ensures they are killed promptly.
const perTestTimeout = 2 * time.Minute

// testDurationSeconds is the length of the synthetic test video.
// 10 seconds gives stable FPS measurements with minimal run time.
const testDurationSeconds = 10

// testTotalFrames is frames expected in the test video (duration × 30 fps).
const testTotalFrames = testDurationSeconds * 30

// Result stores the outcome of a single encoder benchmark test.
type Result struct {
	Encoder      string  // e.g. "h264_nvenc", "libx264"
	HWAccel      string  // "nvenc", "amf", "qsv", or "none"
	FPS          float64 // Encoding frames per second
	EncodingTime float64 // Total encoding time in seconds
	InputSize    int64   // Input file size in bytes
	Score        float64 // Ranking score (= FPS)
	Error        string  // Non-empty when the test failed
}

// FriendlyName returns a human-readable label for an encoder string.
func FriendlyName(encoder string) string {
	switch encoder {
	case "libx264":
		return "Software H.264"
	case "libx265":
		return "Software H.265"
	case "h264_nvenc":
		return "NVENC H.264"
	case "hevc_nvenc":
		return "NVENC H.265"
	case "av1_nvenc":
		return "NVENC AV1"
	case "h264_qsv":
		return "QuickSync H.264"
	case "hevc_qsv":
		return "QuickSync H.265"
	case "h264_amf":
		return "AMF H.264"
	case "hevc_amf":
		return "AMF H.265"
	case "av1_amf":
		return "AMF AV1"
	default:
		return encoder
	}
}

// HWAccelLabel returns a display name for a hardware acceleration path.
func HWAccelLabel(hwAccel string) string {
	switch hwAccel {
	case "nvenc":
		return "NVIDIA NVENC"
	case "amf":
		return "AMD AMF"
	case "qsv":
		return "Intel QuickSync"
	case "none":
		return "Software"
	default:
		return hwAccel
	}
}

// hwAccelFor maps an encoder name to its hardware acceleration path.
func hwAccelFor(encoder string) string {
	switch {
	case strings.Contains(encoder, "nvenc"):
		return "nvenc"
	case strings.Contains(encoder, "amf"):
		return "amf"
	case strings.Contains(encoder, "qsv"):
		return "qsv"
	default:
		return "none"
	}
}

// Suite manages a complete benchmark test suite.
type Suite struct {
	TestVideoPath string
	OutputDir     string
	FFmpegPath    string
	Results       []Result

	// Progress is called before and after each encoder test.
	// label is the human-readable encoder name (e.g. "NVENC H.264").
	Progress func(current, total int, label string)
}

// NewSuite creates a new benchmark suite.
func NewSuite(ffmpegPath, outputDir string) *Suite {
	return &Suite{
		FFmpegPath: ffmpegPath,
		OutputDir:  outputDir,
		Results:    []Result{},
	}
}

// GenerateTestVideo creates a short synthetic 1080p test video.
// The duration parameter is accepted for API compatibility but the suite
// always uses testDurationSeconds for consistent measurements.
func (s *Suite) GenerateTestVideo(ctx context.Context, _ int) (string, error) {
	testPath := filepath.Join(s.OutputDir, "benchmark_test.mp4")

	dur := fmt.Sprintf("%d", testDurationSeconds)
	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%s:size=1920x1080:rate=30", dur),
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=1000:duration=%s", dur),
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

// UseTestVideo sets an existing video as the test source.
func (s *Suite) UseTestVideo(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("test video not found: %w", err)
	}
	s.TestVideoPath = path
	return nil
}

// TestEncoder runs a single encode pass for the given encoder at its
// designated neutral quality preset and returns the result.
func (s *Suite) TestEncoder(ctx context.Context, encoder, preset string) Result {
	result := Result{
		Encoder: encoder,
		HWAccel: hwAccelFor(encoder),
	}

	if s.TestVideoPath == "" {
		result.Error = "no test video specified"
		return result
	}

	inputInfo, err := os.Stat(s.TestVideoPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to stat input: %v", err)
		return result
	}
	result.InputSize = inputInfo.Size()

	// Encode to null — no output file needed
	args := []string{"-y", "-i", s.TestVideoPath, "-c:v", encoder}

	if preset != "" {
		switch {
		case strings.HasSuffix(encoder, "_nvenc"):
			// NVENC: -preset p1-p7
			args = append(args, "-preset", preset)
		case strings.HasSuffix(encoder, "_qsv"):
			// QuickSync: -preset slow/medium/fast/…
			args = append(args, "-preset", preset)
		case strings.HasSuffix(encoder, "_amf"):
			// AMF: -quality speed/balanced/quality
			args = append(args, "-quality", preset)
		default:
			// Software encoders
			args = append(args, "-preset", preset)
		}
	}

	args = append(args, "-c:a", "copy", "-f", "null", "-")

	start := time.Now()
	cmd := utils.CreateCommand(ctx, s.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		result.Error = fmt.Sprintf("encoding failed: %v", err)
		return result
	}

	result.EncodingTime = time.Since(start).Seconds()
	result.FPS = float64(testTotalFrames) / result.EncodingTime
	result.Score = result.FPS

	return result
}

// testEntry defines one encoder test: a single encoder at its designated
// neutral preset. One entry per encoder — no preset sweeps.
type testEntry struct {
	encoder string
	preset  string
}

// testMatrix lists every encoder the benchmark knows about, in the order
// they should be tested. Encoders not present in availableEncoders are
// skipped at runtime.
var testMatrix = []testEntry{
	// Software baselines
	{"libx264", "medium"},
	{"libx265", "medium"},
	// NVIDIA NVENC — p4 is the balanced mid-point of the p1-p7 scale
	{"h264_nvenc", "p4"},
	{"hevc_nvenc", "p4"},
	{"av1_nvenc", "p4"},
	// Intel QuickSync
	{"h264_qsv", "medium"},
	{"hevc_qsv", "medium"},
	// AMD AMF
	{"h264_amf", "balanced"},
	{"hevc_amf", "balanced"},
	{"av1_amf", "balanced"},
}

// RunFullSuite runs one test per encoder for every available encoder in the
// test matrix and appends results to s.Results.
func (s *Suite) RunFullSuite(ctx context.Context, availableEncoders []string) error {
	available := make(map[string]bool, len(availableEncoders))
	for _, enc := range availableEncoders {
		available[enc] = true
	}

	var active []testEntry
	for _, t := range testMatrix {
		if available[t.encoder] {
			active = append(active, t)
		}
	}
	total := len(active)

	for i, t := range active {
		label := FriendlyName(t.encoder)

		if s.Progress != nil {
			s.Progress(i, total, label)
		}

		testCtx, testCancel := context.WithTimeout(ctx, perTestTimeout)
		result := s.TestEncoder(testCtx, t.encoder, t.preset)
		testCancel()

		s.Results = append(s.Results, result)

		if s.Progress != nil {
			s.Progress(i+1, total, label)
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

// GetRecommendation returns the best result by FPS score. Hardware encoders
// are naturally much faster than software, so the winner will be a hardware
// encoder when one works. Returns an empty Result if no tests were run.
func (s *Suite) GetRecommendation() (encoder string, result Result) {
	if len(s.Results) == 0 {
		return "", Result{}
	}

	best := s.Results[0]
	for _, r := range s.Results {
		if r.Error == "" && r.Score > best.Score {
			best = r
		}
	}

	return best.Encoder, best
}
