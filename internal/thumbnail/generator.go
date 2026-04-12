package thumbnail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	Progress      func(float64)
	// OnThumbGenerated is called each time a thumbnail file is written to disk.
	// For individual thumbnails it fires once per frame; for a contact sheet it
	// fires once when the final composite image is complete.
	OnThumbGenerated func(path string)
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
		// Generate contact sheet — pass pre-computed dimensions to avoid duplicate ffprobe calls
		contactSheetPath, err := g.generateContactSheet(ctx, config, duration, width, height, thumbWidth, thumbHeight)
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
	hideCmd(cmd)

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
	cmd := exec.CommandContext(ctx, utils.GetFFprobePath(),
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,r_frame_rate,bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	hideCmd(cmd)

	output, err := cmd.Output()
	if err != nil {
		return "unknown", "unknown", 0, 0, 0
	}

	// Parse video stream info
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 1 {
		videoCodec = strings.ToUpper(strings.TrimSpace(lines[0]))
	}
	if len(lines) >= 2 {
		// Parse frame rate (format: "30000/1001" or "30/1")
		fpsStr := strings.TrimSpace(lines[1])
		var num, den float64
		if _, err := fmt.Sscanf(fpsStr, "%f/%f", &num, &den); err == nil && den > 0 {
			fps = num / den
		}
	}
	if len(lines) >= 3 && strings.TrimSpace(lines[2]) != "N/A" {
		// Parse bitrate if available
		fmt.Sscanf(strings.TrimSpace(lines[2]), "%f", &bitrate)
	}

	// Get audio codec and bitrate
	cmd = exec.CommandContext(ctx, utils.GetFFprobePath(),
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name,bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	hideCmd(cmd)

	output, err = cmd.Output()
	if err == nil {
		audioLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(audioLines) >= 1 {
			audioCodec = strings.ToUpper(strings.TrimSpace(audioLines[0]))
		}
		if len(audioLines) >= 2 && strings.TrimSpace(audioLines[1]) != "N/A" {
			fmt.Sscanf(strings.TrimSpace(audioLines[1]), "%f", &audioBitrate)
		}
	}

	// If bitrate wasn't available from video stream, try to get overall bitrate
	if bitrate == 0 {
		cmd = exec.CommandContext(ctx, utils.GetFFprobePath(),
			"-v", "error",
			"-show_entries", "format=bit_rate",
			"-of", "default=noprint_wrappers=1:nokey=1",
			videoPath,
		)
		hideCmd(cmd)

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
	total := len(timestamps)

	// Generate each thumbnail
	for i, ts := range timestamps {
		outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("thumbnail_%04d.%s", i+1, config.Format))

		// Build FFmpeg command
		args := []string{
			"-ss", fmt.Sprintf("%.2f", ts),
			"-nostdin",
			"-i", config.VideoPath,
			"-vf", g.buildThumbFilter(thumbWidth, thumbHeight, config.ShowTimestamp, ts),
			"-frames:v", "1",
			"-y",
		}

		// Add quality settings
		if config.Format == "jpg" {
			args = append(args, "-q:v", fmt.Sprintf("%d", 31-(config.Quality*30/100)))
		}

		args = append(args, outputPath)

		cmd := exec.CommandContext(ctx, g.FFmpegPath, args...)
		hideCmd(cmd)
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
		if config.OnThumbGenerated != nil {
			config.OnThumbGenerated(outputPath)
		}
		if config.Progress != nil && total > 0 {
			config.Progress((float64(i+1) / float64(total)) * 100)
		}
	}

	return thumbnails, nil
}

// generateContactSheet creates a single contact sheet with all thumbnails.
// videoWidth/videoHeight are the source video dimensions (passed in to avoid a duplicate ffprobe call).
func (g *Generator) generateContactSheet(ctx context.Context, config Config, duration float64, videoWidth, videoHeight, thumbWidth, thumbHeight int) (string, error) {
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

	// Use fps + setpts as the select filter — trim is intentionally omitted here.
	// The trim filter causes FFmpeg to flush and reinitialize downstream filters at its
	// end boundary, which can expose the codec's MCU-padded coded dimensions (e.g.
	// 480×368 for a 480×360 display) and make the inner pad filter fail with
	// "padded dimensions cannot be smaller than input dimensions".
	// Instead, we apply the time window via -ss/-t input options (see args below).
	selectFilter := fmt.Sprintf("fps=%.6f,setpts=PTS-STARTPTS+%.2f/TB",
		sampleFPS,
		startTime,
	)

	baseName := strings.TrimSuffix(filepath.Base(config.VideoPath), filepath.Ext(config.VideoPath))
	outputPath := filepath.Join(config.OutputDir, fmt.Sprintf("%s_contact_sheet.%s", baseName, config.Format))

	// Build tile filter with padding between thumbnails.
	// Pass ts=0 so that the contact sheet timestamps use %{pts:hms} (the setpts above
	// restores the original presentation timestamps for each sampled frame).
	padding := 8 // Pixels of padding between each thumbnail
	tileFilter := fmt.Sprintf("%s,tile=%dx%d:padding=%d", g.buildThumbFilter(thumbWidth, thumbHeight, config.ShowTimestamp, 0), config.Columns, config.Rows, padding)

	// Build video filter — fetch detailed info once here to avoid duplicate ffprobe calls
	var vfilter string
	if config.ShowMetadata {
		videoCodec, audioCodec, fps, bitrate, audioBitrate := g.getDetailedVideoInfo(ctx, config.VideoPath)
		vfilter = g.buildMetadataFilter(config, duration, videoWidth, videoHeight, thumbWidth, thumbHeight, padding, selectFilter, tileFilter, videoCodec, audioCodec, fps, bitrate, audioBitrate)
	} else {
		vfilter = fmt.Sprintf("%s,%s", selectFilter, tileFilter)
	}

	// Build FFmpeg command.
	// -ss / -t as INPUT options (before -i) perform fast keyframe seeking without
	// inserting a trim filter, avoiding the filter-graph reinitialisation that was
	// causing the pad failure on MCU-padded codecs (Xvid, MPEG-4 ASP, etc.).
	args := []string{
		"-nostdin",
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-t", fmt.Sprintf("%.2f", availableDuration),
		"-i", config.VideoPath,
		"-vf", vfilter,
		"-frames:v", "1",
		"-y",
	}

	if config.Format == "jpg" {
		args = append(args, "-q:v", fmt.Sprintf("%d", 31-(config.Quality*30/100)))
	}
	args = append(args, outputPath)

	// Contact sheet is a single FFmpeg pass that produces exactly 1 output frame
	// (the tiled composite). FFmpeg's -progress frame= would only tick once at the
	// very end, giving no intermediate feedback. Use a time-based ticker instead so
	// the UI shows smooth progress while the command runs.
	cmd := exec.CommandContext(ctx, g.FFmpegPath, args...)
	hideCmd(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if config.Progress != nil {
		config.Progress(0)

		// Heuristic estimate: ~1s processing per 60s of video, min 3s, max 120s.
		estimatedMs := int64(availableDuration / 60 * 1000)
		if estimatedMs < 3000 {
			estimatedMs = 3000
		} else if estimatedMs > 120000 {
			estimatedMs = 120000
		}

		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(300 * time.Millisecond)
			defer ticker.Stop()
			start := time.Now()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					elapsed := time.Since(start).Milliseconds()
					pct := float64(elapsed) / float64(estimatedMs) * 90 // cap at 90%; 100% on completion
					if pct > 90 {
						pct = 90
					}
					config.Progress(pct)
				}
			}
		}()

		runErr := cmd.Run()
		close(done)
		if runErr != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", fmt.Errorf("failed to generate contact sheet: %w (%s)", runErr, strings.TrimSpace(stderr.String()))
		}
		config.Progress(100)
	} else {
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", fmt.Errorf("failed to generate contact sheet: %w (%s)", err, strings.TrimSpace(stderr.String()))
		}
	}

	if config.OnThumbGenerated != nil {
		config.OnThumbGenerated(outputPath)
	}

	return outputPath, nil
}

// buildMetadataFilter creates a filter that adds metadata header to contact sheet.
// All video metadata is passed in to avoid redundant ffprobe calls.
func (g *Generator) buildMetadataFilter(
	config Config, duration float64,
	videoWidth, videoHeight, thumbWidth, thumbHeight, padding int,
	selectFilter, tileFilter string,
	videoCodec, audioCodec string, fps, bitrate, audioBitrate float64,
) string {
	// Get file info
	fileInfo, _ := os.Stat(config.VideoPath)
	fileSize := fileInfo.Size()
	fileSizeMB := float64(fileSize) / (1024 * 1024)

	// Format duration as HH:MM:SS
	hours := int(duration) / 3600
	minutes := (int(duration) % 3600) / 60
	seconds := int(duration) % 60
	durationStr := fmt.Sprintf("%02d\\:%02d\\:%02d", hours, minutes, seconds)

	// Get just the filename without path, handle for 2-line title support
	filename := filepath.Base(config.VideoPath)
	ext := strings.ToLower(filepath.Ext(filename)) // always show extension lowercase
	name := strings.TrimSuffix(filename, ext)

	var line1a, line1b string
	maxLineLen := 40 // max characters per line
	if len(filename) > maxLineLen*2 {
		// Very long filename - split into 2 lines, preserve extension at end of line 2
		extLen := len(ext)
		if extLen > 6 {
			extLen = 6 // cap for display
		}
		// First line takes up to maxLineLen
		if len(name) > maxLineLen-3 {
			line1a = name[:maxLineLen-3] + "..."
		} else {
			line1a = name
		}
		// Second line: remaining + extension
		remaining := len(name) - (maxLineLen - 3)
		if remaining > 0 {
			line1b = "..." + name[len(name)-remaining:] + ext
		} else {
			line1b = ext
		}
	} else if len(filename) > maxLineLen {
		// Medium length - wrap once
		half := len(name) / 2
		line1a = name[:half]
		line1b = name[half:] + ext
	} else {
		// Short enough - single line
		line1a = filename
		line1b = ""
	}

	// Calculate sheet dimensions accounting for padding between thumbnails
	sheetWidth := (thumbWidth * config.Columns) + (padding * (config.Columns - 1))
	sheetHeight := (thumbHeight * config.Rows) + (padding * (config.Rows - 1))
	headerHeight := 150 // Increased to fit all metadata lines without overlap

	// Build metadata text lines.
	// Use · (middle dot) as separator — pipe (|) is treated as a newline by FFmpeg drawtext.
	// Line 1: Filename (up to 2 lines) and file size (bold) — escape for drawtext single-quoted text= value
	var line1 string
	if line1b != "" {
		line1 = fmt.Sprintf("%s\\n%s (%.1f MB)", escapeDrawtextText(line1a), escapeDrawtextText(line1b), fileSizeMB)
	} else {
		line1 = fmt.Sprintf("%s (%.1f MB)", escapeDrawtextText(line1a), fileSizeMB)
	}
	// Line 2: Resolution, frame rate, and duration
	line2 := fmt.Sprintf("%dx%d @ %.2f fps · %s", videoWidth, videoHeight, fps, durationStr)
	// Line 3: Codecs and bitrates
	bitrateKbps := int(bitrate / 1000)
	var audioInfo string
	if audioBitrate > 0 {
		audioBitrateKbps := int(audioBitrate / 1000)
		audioInfo = fmt.Sprintf("%s %dkbps", audioCodec, audioBitrateKbps)
	} else {
		audioInfo = audioCodec
	}
	line3 := fmt.Sprintf("Video\\: %s / Audio\\: %s / %d kbps", videoCodec, audioInfo, bitrateKbps)

	// Font args: bold for line 1 (filename), regular for lines 2 and 3.
	regularFontPath := g.findFontPath()
	boldFontPath := g.findBoldFontPath()
	regularFontArg := "font='DejaVu Sans Mono'"
	boldFontArg := regularFontArg // fall back to regular if bold not available
	if regularFontPath != "" {
		regularFontArg = fmt.Sprintf("fontfile='%s'", escapeFilterPath(regularFontPath))
	}
	if boldFontPath != "" {
		boldFontArg = fmt.Sprintf("fontfile='%s'", escapeFilterPath(boldFontPath))
	}

	// Build the composite filter with adjusted positions for larger fonts and 2-line title.
	// Title (line1): fontsize 24, 2 lines = ~48px, starts at y=20
	// Line 2 (resolution): fontsize 18, starts at y=80
	// Line 3 (codecs): fontsize 18, starts at y=112
	textPadding := 32
	baseFilter := fmt.Sprintf(
		"%s,%s,pad=%d:%d:0:%d:0x0B0F1A,"+
			"drawtext=text='%s':fontcolor=0x4CE870:fontsize=24:%s:x=%d:y=20,"+
			"drawtext=text='%s':fontcolor=white:fontsize=18:%s:x=%d:y=80,"+
			"drawtext=text='%s':fontcolor=white:fontsize=18:%s:x=%d:y=112",
		selectFilter,
		tileFilter,
		sheetWidth,
		sheetHeight+headerHeight,
		headerHeight,
		line1, boldFontArg, textPadding,
		line2, regularFontArg, textPadding,
		line3, regularFontArg, textPadding,
	)

	logoPath := g.findLogoPath()
	if logoPath == "" {
		return baseFilter
	}

	logoScale := 82
	logoFilter := fmt.Sprintf("%s[sheet];movie='%s',scale=%d:%d[logo];[sheet][logo]overlay=x=main_w-overlay_w-32:y=(%d-overlay_h)/2",
		baseFilter,
		escapeFilterPath(logoPath),
		logoScale,
		logoScale,
		headerHeight,
	)

	return logoFilter
}

// buildThumbFilter constructs an FFmpeg video filter string for thumbnail scaling,
// padding, and optional timestamp overlay.
//
// ts controls the timestamp drawtext behaviour:
//   - ts == 0: use the FFmpeg %{pts:hms} expression (suitable for contact sheets where
//     setpts has already restored real presentation timestamps).
//   - ts > 0: hardcode the formatted timestamp string (suitable for individual thumbnails
//     generated with an input seek (-ss), which resets the PTS counter to 0 and makes
//     %{pts:hms} always read 00:00:00.000).
func (g *Generator) buildThumbFilter(thumbWidth, thumbHeight int, showTimestamp bool, ts float64) string {
	// Use math.Ceil to ensure dimensions are never smaller than target after scaling
	// This prevents "Padded dimensions cannot be smaller than input dimensions" error
	w := thumbWidth
	h := thumbHeight

	// setsar=1 normalises the sample aspect ratio before scale so that
	// non-square-pixel sources (e.g. some DVDs and older MPEG-4 ASPs) are
	// handled correctly without the scale filter seeing a stretched frame.
	filter := fmt.Sprintf("setsar=1,scale=%d:%d:force_original_aspect_ratio=decrease,pad=ceil(%d):ceil(%d):(ow-iw)/2:(oh-ih)/2:black",
		w, h, w, h,
	)
	fontPath := g.findFontPath()
	fontArg := "font='DejaVu Sans Mono'"
	if fontPath != "" {
		fontArg = fmt.Sprintf("fontfile='%s'", escapeFilterPath(fontPath))
	}
	if showTimestamp {
		var tsText string
		if ts > 0 {
			// Hardcode the timestamp so that input-seek PTS resets don't corrupt the overlay.
			totalSec := int(ts)
			h := totalSec / 3600
			m := (totalSec % 3600) / 60
			s := totalSec % 60
			ms := int((ts - float64(totalSec)) * 1000)
			tsText = fmt.Sprintf("%02d\\:%02d\\:%02d.%03d", h, m, s, ms)
		} else {
			// Contact sheet: setpts restores real PTS so %{pts:hms} is accurate.
			tsText = "%{pts\\:hms}"
		}
		filter += fmt.Sprintf(",drawtext=text='%s':fontcolor=white:fontsize=18:%s:box=1:boxcolor=black@0.5:boxborderw=4:x=w-text_w-6:y=h-text_h-6", tsText, fontArg)
	}
	return filter
}

func (g *Generator) findLogoPath() string {
	// First try embedded logo
	if len(LogoData) > 0 {
		tmpDir := os.TempDir()
		logoPath := filepath.Join(tmpDir, "vt_logo.png")
		if err := os.WriteFile(logoPath, LogoData, 0644); err == nil {
			return logoPath
		}
	}

	// Fallback to file-based search
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

func (g *Generator) findFontPath() string {
	if len(FontData) > 0 {
		fontPath := filepath.Join(os.TempDir(), "IBMPlexMono-Regular.ttf")
		if err := os.WriteFile(fontPath, FontData, 0644); err == nil {
			return fontPath
		}
	}
	search := []string{filepath.Join("assets", "fonts", "IBMPlexMono-Regular.ttf")}
	if exe, err := os.Executable(); err == nil {
		search = append(search, filepath.Join(filepath.Dir(exe), "assets", "fonts", "IBMPlexMono-Regular.ttf"))
	}
	for _, p := range search {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (g *Generator) findBoldFontPath() string {
	if len(BoldFontData) > 0 {
		fontPath := filepath.Join(os.TempDir(), "IBMPlexMono-Bold.ttf")
		if err := os.WriteFile(fontPath, BoldFontData, 0644); err == nil {
			return fontPath
		}
	}
	search := []string{filepath.Join("assets", "fonts", "IBMPlexMono-Bold.ttf")}
	if exe, err := os.Executable(); err == nil {
		search = append(search, filepath.Join(filepath.Dir(exe), "assets", "fonts", "IBMPlexMono-Bold.ttf"))
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

// escapeDrawtextText escapes a string for use as a drawtext text= value
// wrapped in single quotes. Handles apostrophes, backslashes, and % expressions.
func escapeDrawtextText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "%", "%%")
	return s
}

func runFFmpegWithProgress(ctx context.Context, ffmpegPath string, args []string, totalDuration float64, expectedFrames int, progress func(float64)) error {
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	hideCmd(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if progress != nil {
		progress(0)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	go func() {
		if progress == nil {
			return
		}
		scanner := bufio.NewScanner(stdout)
		var lastPct float64
		var lastFrame int
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, val := parts[0], parts[1]
			var pct float64
			updated := false
			if key == "out_time_ms" && totalDuration > 0 {
				if ms, err := strconv.ParseFloat(val, 64); err == nil {
					currentSec := ms / 1000000.0
					pct = (currentSec / totalDuration) * 100
					updated = true
				}
			} else if key == "frame" && expectedFrames > 0 {
				if frame, err := strconv.Atoi(val); err == nil {
					if frame > lastFrame {
						lastFrame = frame
					}
					pct = (float64(lastFrame) / float64(expectedFrames)) * 100
					updated = true
				}
			}
			if !updated {
				continue
			}
			if pct > 100 {
				pct = 100
			}
			if pct-lastPct >= 0.5 || pct >= 100 {
				lastPct = pct
				progress(pct)
			}
		}
	}()

	err = cmd.Wait()
	if progress != nil {
		progress(100)
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
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
		"-nostdin",
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
	hideCmd(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract frame: %w", err)
	}

	return nil
}

// CleanupThumbnails removes all generated thumbnails
func CleanupThumbnails(outputDir string) error {
	return os.RemoveAll(outputDir)
}
