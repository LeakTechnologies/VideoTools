package interlace

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DetectionResult contains the results of interlacing analysis
type DetectionResult struct {
	// Frame counts from idet filter
	TFF          int     // Top Field First frames
	BFF          int     // Bottom Field First frames
	Progressive  int     // Progressive frames
	Undetermined int     // Undetermined frames
	TotalFrames  int     // Total frames analyzed

	// Calculated metrics
	InterlacedPercent float64 // Percentage of interlaced frames
	Status            string  // "Progressive", "Interlaced", "Mixed"
	FieldOrder        string  // "TFF", "BFF", "Unknown"
	Confidence        string  // "High", "Medium", "Low"

	// Recommendations
	Recommendation    string  // Human-readable recommendation
	SuggestDeinterlace bool   // Whether deinterlacing is recommended
	SuggestedFilter    string // "yadif", "bwdif", etc.
}

// Detector analyzes video for interlacing
type Detector struct {
	FFmpegPath string
	FFprobePath string
}

// NewDetector creates a new interlacing detector
func NewDetector(ffmpegPath, ffprobePath string) *Detector {
	return &Detector{
		FFmpegPath: ffmpegPath,
		FFprobePath: ffprobePath,
	}
}

// Analyze performs interlacing detection on a video file
// sampleFrames: number of frames to analyze (0 = analyze entire video)
func (d *Detector) Analyze(ctx context.Context, videoPath string, sampleFrames int) (*DetectionResult, error) {
	// Build FFmpeg command with idet filter
	args := []string{
		"-i", videoPath,
		"-filter:v", "idet",
		"-frames:v", fmt.Sprintf("%d", sampleFrames),
		"-an", // No audio
		"-f", "null",
		"-",
	}

	if sampleFrames == 0 {
		// Remove frame limit to analyze entire video
		args = []string{
			"-i", videoPath,
			"-filter:v", "idet",
			"-an",
			"-f", "null",
			"-",
		}
	}

	cmd := exec.CommandContext(ctx, d.FFmpegPath, args...)

	// Capture stderr (where idet outputs its stats)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Parse idet output from stderr
	result := &DetectionResult{}
	scanner := bufio.NewScanner(stderr)

	// Regex patterns for idet statistics
	// Example: [Parsed_idet_0 @ 0x...] Multi frame detection: TFF:123 BFF:0 Progressive:456 Undetermined:7
	multiFrameRE := regexp.MustCompile(`Multi frame detection:\s+TFF:\s*(\d+)\s+BFF:\s*(\d+)\s+Progressive:\s*(\d+)\s+Undetermined:\s*(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()

		// Look for the final "Multi frame detection" line
		if matches := multiFrameRE.FindStringSubmatch(line); matches != nil {
			result.TFF, _ = strconv.Atoi(matches[1])
			result.BFF, _ = strconv.Atoi(matches[2])
			result.Progressive, _ = strconv.Atoi(matches[3])
			result.Undetermined, _ = strconv.Atoi(matches[4])
		}
	}

	if err := cmd.Wait(); err != nil {
		// FFmpeg might return error even on success with null output
		// Only fail if we got no results
		if result.TFF == 0 && result.BFF == 0 && result.Progressive == 0 {
			return nil, fmt.Errorf("ffmpeg failed: %w", err)
		}
	}

	// Calculate metrics
	result.TotalFrames = result.TFF + result.BFF + result.Progressive + result.Undetermined
	if result.TotalFrames == 0 {
		return nil, fmt.Errorf("no frames analyzed - check video file")
	}

	interlacedFrames := result.TFF + result.BFF
	result.InterlacedPercent = (float64(interlacedFrames) / float64(result.TotalFrames)) * 100

	// Determine status
	if result.InterlacedPercent < 5 {
		result.Status = "Progressive"
	} else if result.InterlacedPercent > 95 {
		result.Status = "Interlaced"
	} else {
		result.Status = "Mixed Content"
	}

	// Determine field order
	if result.TFF > result.BFF*2 {
		result.FieldOrder = "TFF (Top Field First)"
	} else if result.BFF > result.TFF*2 {
		result.FieldOrder = "BFF (Bottom Field First)"
	} else if interlacedFrames > 0 {
		result.FieldOrder = "Mixed/Unknown"
	} else {
		result.FieldOrder = "N/A (Progressive)"
	}

	// Determine confidence
	uncertainRatio := float64(result.Undetermined) / float64(result.TotalFrames)
	if uncertainRatio < 0.05 {
		result.Confidence = "High"
	} else if uncertainRatio < 0.15 {
		result.Confidence = "Medium"
	} else {
		result.Confidence = "Low"
	}

	// Generate recommendation
	if result.InterlacedPercent < 5 {
		result.Recommendation = "Video is progressive. No deinterlacing needed."
		result.SuggestDeinterlace = false
	} else if result.InterlacedPercent > 95 {
		result.Recommendation = "Video is fully interlaced. Deinterlacing strongly recommended."
		result.SuggestDeinterlace = true
		result.SuggestedFilter = "yadif"
	} else {
		result.Recommendation = fmt.Sprintf("Video has %.1f%% interlaced frames. Deinterlacing recommended for mixed content.", result.InterlacedPercent)
		result.SuggestDeinterlace = true
		result.SuggestedFilter = "yadif"
	}

	return result, nil
}

// QuickAnalyze performs a fast analysis using only the first N frames
func (d *Detector) QuickAnalyze(ctx context.Context, videoPath string) (*DetectionResult, error) {
	// Analyze first 500 frames for speed
	return d.Analyze(ctx, videoPath, 500)
}

// GenerateDeinterlacePreview generates a preview frame showing before/after deinterlacing
func (d *Detector) GenerateDeinterlacePreview(ctx context.Context, videoPath string, timestamp float64, outputPath string) error {
	// Extract frame at timestamp, apply yadif filter, and save
	args := []string{
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-vf", "yadif=0:-1:0", // Deinterlace with yadif
		"-frames:v", "1",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, d.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate preview: %w", err)
	}

	return nil
}

// GenerateComparisonPreview generates a side-by-side comparison of original vs deinterlaced
func (d *Detector) GenerateComparisonPreview(ctx context.Context, videoPath string, timestamp float64, outputPath string) error {
	// Create side-by-side comparison: original (left) vs deinterlaced (right)
	args := []string{
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-filter_complex", "[0:v]split=2[orig][deint];[deint]yadif=0:-1:0[d];[orig][d]hstack",
		"-frames:v", "1",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, d.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate comparison: %w", err)
	}

	return nil
}

// String returns a formatted string representation of the detection result
func (r *DetectionResult) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("Interlaced: %.1f%%\n", r.InterlacedPercent))
	sb.WriteString(fmt.Sprintf("Field Order: %s\n", r.FieldOrder))
	sb.WriteString(fmt.Sprintf("Confidence: %s\n", r.Confidence))
	sb.WriteString(fmt.Sprintf("\nFrame Analysis:\n"))
	sb.WriteString(fmt.Sprintf("  Progressive: %d\n", r.Progressive))
	sb.WriteString(fmt.Sprintf("  Top Field First: %d\n", r.TFF))
	sb.WriteString(fmt.Sprintf("  Bottom Field First: %d\n", r.BFF))
	sb.WriteString(fmt.Sprintf("  Undetermined: %d\n", r.Undetermined))
	sb.WriteString(fmt.Sprintf("  Total Analyzed: %d\n", r.TotalFrames))
	sb.WriteString(fmt.Sprintf("\nRecommendation: %s\n", r.Recommendation))

	return sb.String()
}
