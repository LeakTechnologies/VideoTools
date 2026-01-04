package thumbnail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// Config contains configuration for thumbnail generation
type Config struct {
	VideoPath     string
	OutputDir     string
	Count         int     // Number of thumbnails to generate
	Interval      float64 // Interval in seconds between thumbnails (alternative to Count)
	Width         int     // Thumbnail width (0 = auto based on height)
	Height        int     // Thumbnail height (0 = auto based on width)
	Quality       int     // JPEG quality 1-100 (0 = PNG lossless)
	Format        string  // "png" or "jpg"
	StartOffset   float64 // Start generating from this timestamp
	EndOffset     float64 // Stop generating before this timestamp
	ContactSheet  bool    // Generate a single contact sheet instead of individual files
	Columns       int     // Contact sheet columns (if ContactSheet=true)
	Rows          int     // Contact sheet rows (if ContactSheet=true)
	ShowTimestamp bool    // Overlay timestamp on thumbnails
	ShowMetadata  bool    // Show metadata header on contact sheet
}

// Generator creates thumbnails from videos
type Generator struct {
	FFmpegPath string
}

// NewGenerator creates a new thumbnail generator
func NewGenerator(ffmpegPath string) *Generator {
	return &Generator{
		FFmpegPath: ffmpegPath,
	}
}

// Thumbnail represents a generated thumbnail
type Thumbnail struct {
	Path      string
	Timestamp float64
	Width     int
	Height    int
	Size      int64
}

// GenerateResult contains the results of thumbnail generation
type GenerateResult struct {
	Thumbnails    []Thumbnail
	ContactSheet  string // Path to contact sheet if generated
	TotalDuration float64
	VideoWidth    int
	VideoHeight   int
	VideoFPS      float64
	VideoCodec    string
	AudioCodec    string
	FileSize      int64
	Error         string
}

// Generate creates thumbnails based on the provided configuration
func (g *Generator) Generate(ctx context.Context, config Config) (*GenerateResult, error) {
	result := &GenerateResult{}

	// Validate config
	if config.VideoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}
	if config.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Set defaults
	if config.Count == 0 && config.Interval == 0 {
		config.Count = 9 // Default to 9 thumbnails (3x3 grid)
	}
	if config.Format == "" {
		config.Format = "jpg"
	}
	if config.Quality == 0 && config.Format == "jpg" {
		config.Quality = 85
	}
	if config.ContactSheet {
		if config.Columns == 0 {
			config.Columns = 3
		}
		if config.Rows == 0 {
			config.Rows = 3
		}
	}

	// Get video duration and dimensions
	duration, width, height, err := g.getVideoInfo(ctx, config.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}
	result.TotalDuration = duration
	result.VideoWidth = width
	result.VideoHeight = height

	// Calculate thumbnail dimensions
	thumbWidth, thumbHeight := g.calculateDimensions(width, height, config.Width, config.Height)

	if config.ContactSheet {
		// Generate contact sheet
		contactSheetPath, err := g.generateContactSheet(ctx, config, duration, thumbWidth, thumbHeight)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.ContactSheet = contactSheetPath

		// Get file size
		if fi, err := os.Stat(contactSheetPath); err == nil {
			result.Thumbnails = []Thumbnail{{
				Path:      contactSheetPath,
				Timestamp: 0,
				Width:     thumbWidth * config.Columns,
				Height:    thumbHeight * config.Rows,
				Size:      fi.Size(),
			}}
		}
	} else {
		// Generate individual thumbnails
		thumbnails, err := g.generateIndividual(ctx, config, duration, thumbWidth, thumbHeight)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Thumbnails = thumbnails
	}

	return result, nil
}

// getVideoInfo retrieves duration and dimensions from a video file
func (g *Generator) getVideoInfo(ctx context.Context, videoPath string) (duration float64, width, height int, err error) {
	// Use ffprobe to get video information
	cmd := exec.CommandContext(ctx, utils.GetFFprobePath(),
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-show_entries", "format=duration",
		"-of", "json",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON for robust extraction
	type streamInfo struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Duration string `json:"duration"`
	}
	type formatInfo struct {
		Duration string `json:"duration"`
	}
	type ffprobeResp struct {
		Streams []streamInfo `json:"streams"`
		Format  formatInfo   `json:"format"`
	}

	var resp ffprobeResp
	if err := json.Unmarshal(output, &resp); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse ffprobe json: %w", err)
	}

	var w, h int
	var d float64
	if len(resp.Streams) > 0 {
		w = resp.Streams[0].Width
		h = resp.Streams[0].Height
		if resp.Streams[0].Duration != "" {
			if val, err := strconv.ParseFloat(resp.Streams[0].Duration, 64); err == nil {
				d = val
			}
		}
	}
	if d == 0 && resp.Format.Duration != "" {
		if val, err := strconv.ParseFloat(resp.Format.Duration, 64); err == nil {
			d = val
		}
	}

	if w == 0 || h == 0 {
		return 0, 0, 0, fmt.Errorf("failed to parse video info (missing width/height)")
	}
	if d == 0 {
		return 0, 0, 0, fmt.Errorf("failed to parse video info (missing duration)")
	}

	return d, w, h, nil
}

// getDetailedVideoInfo retrieves codec, fps, and bitrate information from a video file
func (g *Generator) getDetailedVideoInfo(ctx context.Context, videoPath string) (videoCodec, audioCodec string, fps, bitrate, audioBitrate float64) {
	// Use ffprobe to get detailed video and audio information
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,r_frame_rate,bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return "unknown", "unknown", 0, 0, 0
	}

	// Parse video stream info
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 1 {
		videoCodec = strings.ToUpper(lines[0])
	}
	if len(lines) >= 2 {
		// Parse frame rate (format: "30000/1001" or "30/1")
		fpsStr := lines[1]
		var num, den float64
		if _, err := fmt.Sscanf(fpsStr, "%f/%f", &num, &den); err == nil && den > 0 {
			fps = num / den
		}
	}
	if len(lines) >= 3 && lines[2] != "N/A" {
		// Parse bitrate if available
		fmt.Sscanf(lines[2], "%f", &bitrate)
	}

	// Get audio codec and bitrate
	cmd = exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name,bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err = cmd.Output()
	if err == nil {
		audioLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(audioLines) >= 1 {
			audioCodec = strings.ToUpper(audioLines[0])
		}
		if len(audioLines) >= 2 && audioLines[1] != "N/A" {
			fmt.Sscanf(audioLines[1], "%f", &audioBitrate)
		}
	}

	// If bitrate wasn't available from video stream, try to get overall bitrate
	if bitrate == 0 {
		cmd = exec.CommandContext(ctx, "ffprobe",
			"-v", "error",
			"-show_entries", "format=bit_rate",
			"-of", "default=noprint_wrappers=1:nokey=1",
			videoPath,
		)

		output, err = cmd.Output()
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &bitrate)
		}
	}

	// Set defaults if still empty
	if videoCodec == "" {
		videoCodec = "unknown"
	}
	if audioCodec == "" {
		audioCodec = "none"
	}

	return videoCodec, audioCodec, fps, bitrate, audioBitrate
}

// calculateDimensions determines thumbnail dimensions maintaining aspect ratio
func (g *Generator) calculateDimensions(videoWidth, videoHeight, targetWidth, targetHeight int) (width, height int) {
	if targetWidth == 0 && targetHeight == 0 {
		// Default to 320 width
		targetWidth = 320
	}

	aspectRatio := float64(videoWidth) / float64(videoHeight)

	if targetWidth > 0 && targetHeight == 0 {
		// Calculate height from width
		width = targetWidth
		height = int(float64(width) / aspectRatio)
	} else if targetHeight > 0 && targetWidth == 0 {
		// Calculate width from height
		height = targetHeight
		width = int(float64(height) * aspectRatio)
	} else {
		// Both specified, use as-is
		width = targetWidth
		height = targetHeight
	}

	return width, height
}

// generateIndividual creates individual thumbnail files
func (g *Generator) generateIndividual(ctx context.Context, config Config, duration float64, thumbWidth, thumbHeight int) ([]Thumbnail, error) {
	var thumbnails []Thumbnail

	// Calculate timestamps
	timestamps := g.calculateTimestamps(config, duration)

	// Generate each thumbnail
	for i, ts := range timestamps {
		outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("thumb_%04d.%s", i+1, config.Format))

		// Build FFmpeg command
		args := []string{
			"-ss", fmt.Sprintf("%.2f", ts),
			"-i", config.VideoPath,
			"-vf", g.buildThumbFilter(thumbWidth, thumbHeight, config.ShowTimestamp),
			"-frames:v", "1",
			"-y",
		}

		// Add quality settings
		if config.Format == "jpg" {
			args = append(args, "-q:v", fmt.Sprintf("%d", 31-(config.Quality*30/100)))
		}

		args = append(args, outputPath)

		cmd := exec.CommandContext(ctx, g.FFmpegPath, args...)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to generate thumbnail %d: %w", i+1, err)
		}

		// Get file info
		fi, err := os.Stat(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat thumbnail %d: %w", i+1, err)
		}

		thumbnails = append(thumbnails, Thumbnail{
			Path:      outputPath,
			Timestamp: ts,
			Width:     thumbWidth,
			Height:    thumbHeight,
			Size:      fi.Size(),
		})
	}

	return thumbnails, nil
}

// generateContactSheet creates a single contact sheet with all thumbnails
func (g *Generator) generateContactSheet(ctx context.Context, config Config, duration float64, thumbWidth, thumbHeight int) (string, error) {
	totalThumbs := config.Columns * config.Rows
	if config.Count > 0 && config.Count < totalThumbs {
		totalThumbs = config.Count
	}

	startTime := config.StartOffset
	endTime := duration - config.EndOffset
	if endTime <= startTime {
		endTime = duration
	}
	availableDuration := endTime - startTime
	if availableDuration <= 0 {
		availableDuration = duration
	}
	sampleFPS := float64(totalThumbs) / availableDuration
	if sampleFPS <= 0 {
		sampleFPS = 0.01
	}

	// Build select filter using trim + fps to evenly sample across duration
	selectFilter := fmt.Sprintf("trim=start=%.2f:end=%.2f,fps=%.6f,setpts=PTS-STARTPTS+%.2f/TB",
		startTime,
		endTime,
		sampleFPS,
		startTime,
	)

	baseName := strings.TrimSuffix(filepath.Base(config.VideoPath), filepath.Ext(config.VideoPath))
	outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("%s_contact_sheet.%s", baseName, config.Format))

	// Build tile filter with padding between thumbnails
	padding := 8 // Pixels of padding between each thumbnail
	tileFilter := fmt.Sprintf("%s,tile=%dx%d:padding=%d", g.buildThumbFilter(thumbWidth, thumbHeight, config.ShowTimestamp), config.Columns, config.Rows, padding)

	// Build video filter
	var vfilter string
	if config.ShowMetadata {
		// Add metadata header to contact sheet
		vfilter = g.buildMetadataFilter(config, duration, thumbWidth, thumbHeight, padding, selectFilter, tileFilter)
	} else {
		vfilter = fmt.Sprintf("%s,%s", selectFilter, tileFilter)
	}

	// Build FFmpeg command
	args := []string{
		"-i", config.VideoPath,
		"-vf", vfilter,
		"-frames:v", "1",
		"-y",
	}

	if config.Format == "jpg" {
		args = append(args, "-q:v", fmt.Sprintf("%d", 31-(config.Quality*30/100)))
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, g.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate contact sheet: %w", err)
	}

	return outputPath, nil
}

// buildMetadataFilter creates a filter that adds metadata header to contact sheet
func (g *Generator) buildMetadataFilter(config Config, duration float64, thumbWidth, thumbHeight, padding int, selectFilter, tileFilter string) string {
	// Get file info
	fileInfo, _ := os.Stat(config.VideoPath)
	fileSize := fileInfo.Size()
	fileSizeMB := float64(fileSize) / (1024 * 1024)

	// Get video info (we already have duration, just need dimensions)
	_, videoWidth, videoHeight, _ := g.getVideoInfo(context.Background(), config.VideoPath)

	// Get additional video metadata using ffprobe
	videoCodec, audioCodec, fps, bitrate, audioBitrate := g.getDetailedVideoInfo(context.Background(), config.VideoPath)

	// Format duration as HH:MM:SS
	hours := int(duration) / 3600
	minutes := (int(duration) % 3600) / 60
	seconds := int(duration) % 60
	durationStr := fmt.Sprintf("%02d\\:%02d\\:%02d", hours, minutes, seconds)

	// Get just the filename without path
	filename := filepath.Base(config.VideoPath)

	// Calculate sheet dimensions accounting for padding between thumbnails
	// Padding is added between tiles: (cols-1) horizontal gaps and (rows-1) vertical gaps
	sheetWidth := (thumbWidth * config.Columns) + (padding * (config.Columns - 1))
	sheetHeight := (thumbHeight * config.Rows) + (padding * (config.Rows - 1))
	headerHeight := 110

	// Build metadata text lines
	// Line 1: Filename and file size
	line1 := fmt.Sprintf("%s (%.1f MB)", filename, fileSizeMB)
	// Line 2: Resolution and frame rate
	line2 := fmt.Sprintf("%dx%d @ %.2f fps", videoWidth, videoHeight, fps)
	// Line 3: Codecs with audio bitrate, overall bitrate, and duration
	bitrateKbps := int(bitrate / 1000)
	var audioInfo string
	if audioBitrate > 0 {
		audioBitrateKbps := int(audioBitrate / 1000)
		audioInfo = fmt.Sprintf("%s %dkbps", audioCodec, audioBitrateKbps)
	} else {
		audioInfo = audioCodec
	}
	line3 := fmt.Sprintf("Video\\: %s | Audio\\: %s | %d kbps | %s", videoCodec, audioInfo, bitrateKbps, durationStr)

	// Create filter that:
	// 1. Generates contact sheet from selected frames
	// 2. Creates a blank header area with app background color
	// 3. Draws metadata text on header (using monospace font)
	// 4. Stacks header on top of contact sheet
	// App background color: #0B0F1A (dark navy blue)
	baseFilter := fmt.Sprintf(
		"%s,%s,pad=%d:%d:0:%d:0x0B0F1A,"+
			"drawtext=text='%s':fontcolor=white:fontsize=15:font='DejaVu Sans Mono':x=10:y=12,"+
			"drawtext=text='%s':fontcolor=white:fontsize=13:font='DejaVu Sans Mono':x=10:y=40,"+
			"drawtext=text='%s':fontcolor=white:fontsize=12:font='DejaVu Sans Mono':x=10:y=68",
		selectFilter,
		tileFilter,
		sheetWidth,
		sheetHeight+headerHeight,
		headerHeight,
		line1,
		line2,
		line3,
	)

	logoPath := g.findLogoPath()
	if logoPath == "" {
		return baseFilter
	}

	logoScale := 68
	logoFilter := fmt.Sprintf("%s[sheet];movie='%s',scale=%d:%d[logo];[sheet][logo]overlay=x=main_w-overlay_w-12:y=(%d-overlay_h)/2",
		baseFilter,
		escapeFilterPath(logoPath),
		logoScale,
		logoScale,
		headerHeight,
	)

	return logoFilter
}

func (g *Generator) buildThumbFilter(thumbWidth, thumbHeight int, showTimestamp bool) string {
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		thumbWidth,
		thumbHeight,
		thumbWidth,
		thumbHeight,
	)
	if showTimestamp {
		filter += ",drawtext=text='%{pts\\:hms}':fontcolor=white:fontsize=18:font='DejaVu Sans Mono':box=1:boxcolor=black@0.5:boxborderw=4:x=w-text_w-6:y=h-text_h-6"
	}
	return filter
}

func (g *Generator) findLogoPath() string {
	search := []string{
		filepath.Join("assets", "logo", "VT_Icon.png"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		search = append(search, filepath.Join(dir, "assets", "logo", "VT_Icon.png"))
	}
	for _, p := range search {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func escapeFilterPath(path string) string {
	escaped := strings.ReplaceAll(path, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, ":", "\\:")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	return escaped
}

// calculateTimestamps generates timestamps for thumbnail extraction
func (g *Generator) calculateTimestamps(config Config, duration float64) []float64 {
	var timestamps []float64

	startTime := config.StartOffset
	endTime := duration - config.EndOffset
	if endTime <= startTime {
		endTime = duration
	}

	availableDuration := endTime - startTime

	if config.Interval > 0 {
		// Use interval mode
		for ts := startTime; ts < endTime; ts += config.Interval {
			timestamps = append(timestamps, ts)
		}
	} else {
		// Use count mode
		if config.Count <= 1 {
			// Single thumbnail at midpoint
			timestamps = append(timestamps, startTime+availableDuration/2)
		} else {
			// Distribute evenly
			step := availableDuration / float64(config.Count+1)
			for i := 1; i <= config.Count; i++ {
				ts := startTime + (step * float64(i))
				timestamps = append(timestamps, ts)
			}
		}
	}

	return timestamps
}

// ExtractFrame extracts a single frame at a specific timestamp
func (g *Generator) ExtractFrame(ctx context.Context, videoPath string, timestamp float64, outputPath string, width, height int) error {
	args := []string{
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-frames:v", "1",
		"-y",
	}

	if width > 0 || height > 0 {
		if width == 0 {
			width = -1 // Auto
		}
		if height == 0 {
			height = -1 // Auto
		}
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, g.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract frame: %w", err)
	}

	return nil
}

// CleanupThumbnails removes all generated thumbnails
func CleanupThumbnails(outputDir string) error {
	return os.RemoveAll(outputDir)
}
