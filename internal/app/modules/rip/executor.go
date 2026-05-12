package rip

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/css"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// DefaultOutputPath returns the default output path for a rip job.
func DefaultOutputPath(sourcePath, format string) string {
	if sourcePath == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	baseDir := filepath.Join(home, "Videos", "VideoTools", "DVD_Rips")
	name := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if strings.EqualFold(name, "video_ts") {
		name = filepath.Base(filepath.Dir(sourcePath))
	}
	name = SanitizeForPath(name)
	if name == "" {
		name = "dvd_rip"
	}
	ext := ".mkv"
	if format == FormatH264MP4 {
		ext = ".mp4"
	}
	return UniqueFilePath(filepath.Join(baseDir, name+ext))
}

// SanitizeForPath removes characters that are unsafe in file paths.
func SanitizeForPath(label string) string {
	label = strings.TrimSpace(label)
	var out []rune
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return strings.TrimRight(string(out), " ._")
}

// UniqueFilePath returns a path that does not conflict with an existing file.
func UniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// CreateLog opens (or creates) a rip log file, writes a header and returns the file.
func CreateLog(inputPath, outputPath, format, logsDir, logSuffix string) (*os.File, string, error) {
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	if base == "" {
		base = "rip"
	}
	logPath := filepath.Join(logsDir, base+"-rip"+logSuffix)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, logPath, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, logPath, err
	}
	header := fmt.Sprintf(`VideoTools Rip Log
Started: %s
Source: %s
Output: %s
Format: %s

`, time.Now().Format(time.RFC3339), inputPath, outputPath, format)
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

// ResolveVideoTSPath returns the VIDEO_TS (or BDMV) directory from a source path.
// For ISO files it extracts to a temp dir; cleanup must be called when done.
func ResolveVideoTSPath(path string) (string, func(), error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("source not found: %w", err)
	}
	if info.IsDir() {
		if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
			return path, nil, nil
		}
		videoTS := filepath.Join(path, "VIDEO_TS")
		if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
			return videoTS, nil, nil
		}
		return "", nil, fmt.Errorf("no VIDEO_TS folder found in %s", path)
	}
	if strings.HasSuffix(strings.ToLower(path), ".iso") {
		return resolveFromISO(path)
	}
	return "", nil, fmt.Errorf("unsupported source: %s", path)
}

func resolveFromISO(isoPath string) (string, func(), error) {
	// Import here to avoid pulling udf into the executor if not needed.
	// We keep the import at the top of executor.go via the helpers file.
	logging.Info(logging.CatDVD, "Using native Go UDF reader for extraction: %s", isoPath)

	tempDir, err := os.MkdirTemp(utils.TempDir(), "videotools-iso-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	f, err := os.Open(isoPath)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	defer f.Close()

	// Import udf via indirect call to avoid circular imports; caller passes resolver.
	// Here we call the udf package directly — it's allowed since executor is in internal/.
	return resolveISOWithUDF(f, isoPath, tempDir, cleanup)
}

// VobSet represents a group of VOB files from a single title set.
type VobSet struct {
	Name  string
	Files []string
	Size  int64
}

// CollectVOBSets scans a VIDEO_TS directory and returns sorted title sets.
func CollectVOBSets(videoTS string) ([]VobSet, error) {
	entries, err := os.ReadDir(videoTS)
	if err != nil {
		return nil, fmt.Errorf("read VIDEO_TS: %w", err)
	}
	sets := map[string]*VobSet{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".vob") {
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(name), "VTS_") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(name, ".VOB"), "_")
		if len(parts) < 3 {
			continue
		}
		setKey := strings.Join(parts[:2], "_")
		if sets[setKey] == nil {
			sets[setKey] = &VobSet{Name: setKey}
		}
		full := filepath.Join(videoTS, name)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		sets[setKey].Files = append(sets[setKey].Files, full)
		sets[setKey].Size += info.Size()
	}
	var result []VobSet
	for _, set := range sets {
		sort.Strings(set.Files)
		result = append(result, *set)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})
	return result, nil
}

// BuildConcatList writes an ffmpeg concat list file and returns its path.
func BuildConcatList(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no VOB files to concatenate")
	}
	listFile, err := os.CreateTemp(utils.TempDir(), "vt-rip-list-*.txt")
	if err != nil {
		return "", err
	}
	writer := bufio.NewWriter(listFile)
	for _, f := range files {
		fmt.Fprintf(writer, "file '%s'\n", strings.ReplaceAll(f, "'", "'\\''"))
	}
	_ = writer.Flush()
	_ = listFile.Close()
	return listFile.Name(), nil
}

// RipArgs holds parameters for BuildRipArgs.
type RipArgs struct {
	ListFile       string
	OutputPath     string
	Format         string
	MetaFile       string   // path to ffmetadata file; empty = no chapter/title metadata
	AudioLangs     []string // per-stream ISO 639-1 language codes; nil = no tagging
	SubtitleLangs  []string // per-stream subtitle language codes; nil = no subs
	DiscTitle      string   // embedded title tag; empty = skip
	Interlaced     bool     // when true and format is H.264, adds yadif=mode=1 deinterlace filter
}

// BuildRipArgs returns the ffmpeg argument list for a rip job.
//
// -fflags +genpts: DVD VOB audio/subtitle packets frequently have missing PTS.
// The MKV muxer rejects them with "Can't write packet with unknown timestamp";
// genpts synthesises PTS from DTS so every packet has a valid timestamp.
//
// -max_interleave_delta 0: prevents ffmpeg from buffering indefinitely while
// waiting for each stream to reach the same presentation timestamp before flushing.
func BuildRipArgs(ra RipArgs) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-fflags", "+genpts",
		"-f", "concat",
		"-safe", "0",
		"-i", ra.ListFile,
	}

	metaInputIdx := -1
	if ra.MetaFile != "" {
		args = append(args, "-f", "ffmetadata", "-i", ra.MetaFile)
		metaInputIdx = 1
	}

	// Stream mapping
	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "0:a")
	// dvd_subtitle (VOBSUB bitmap) is valid in MKV but not in MP4
	if len(ra.SubtitleLangs) > 0 && ra.Format != FormatH264MP4 {
		args = append(args, "-map", "0:s?")
	}

	// Metadata source
	if metaInputIdx >= 0 {
		args = append(args, "-map_metadata", fmt.Sprintf("%d", metaInputIdx))
		args = append(args, "-map_chapters", fmt.Sprintf("%d", metaInputIdx))
	} else {
		args = append(args, "-map_metadata", "-1") // strip existing metadata
	}

	// Codec
	// Deinterlace filter for interlaced PAL sources on H.264 re-encode paths.
	if ra.Interlaced && (ra.Format == FormatH264MKV || ra.Format == FormatH264MP4) {
		args = append(args, "-vf", "yadif=mode=1")
		logging.Info(logging.CatDVD, "BuildRipArgs: interlaced source detected — added yadif=mode=1")
	}

	switch ra.Format {
	case FormatH264MKV:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "copy",
		)
	case FormatH264MP4:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "192k",
		)
	default:
		args = append(args, "-c", "copy")
	}

	// Per-stream language metadata
	for i, lang := range ra.AudioLangs {
		if lang != "" {
			args = append(args, fmt.Sprintf("-metadata:s:a:%d", i), "language="+lang)
		}
	}
	if ra.Format != FormatH264MP4 {
		for i, lang := range ra.SubtitleLangs {
			if lang != "" {
				args = append(args, fmt.Sprintf("-metadata:s:s:%d", i), "language="+lang)
			}
		}
	}

	// Disc/movie title
	if ra.DiscTitle != "" {
		args = append(args, "-metadata", "title="+ra.DiscTitle)
	}

	args = append(args, "-max_interleave_delta", "0")
	args = append(args, ra.OutputPath)
	return args
}

// BuildFFmpegArgs is the legacy single-call signature kept for Archivist mode.
func BuildFFmpegArgs(listFile, outputPath, format string) []string {
	return BuildRipArgs(RipArgs{
		ListFile:   listFile,
		OutputPath: outputPath,
		Format:     format,
	})
}

// WriteChapterFile writes an ffmetadata file containing chapter timestamps and
// an optional title tag. Returns the file path; caller must remove it when done.
func WriteChapterFile(chapters []float64, totalDuration float64, title string) (string, error) {
	f, err := os.CreateTemp(utils.TempDir(), "vt-chapters-*.txt")
	if err != nil {
		return "", fmt.Errorf("create chapter file: %w", err)
	}
	defer f.Close()

	fmt.Fprintln(f, ";FFMETADATA1")
	if title != "" {
		fmt.Fprintf(f, "title=%s\n", title)
	}
	fmt.Fprintln(f)

	for i, start := range chapters {
		startMs := int64(math.Round(start * 1000))
		var endMs int64
		if i+1 < len(chapters) {
			endMs = int64(math.Round(chapters[i+1] * 1000))
		} else {
			endMs = int64(math.Round(totalDuration * 1000))
		}
		if endMs <= startMs {
			endMs = startMs + 1
		}
		fmt.Fprintf(f, "[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=Chapter %d\n\n",
			startMs, endMs, i+1)
	}
	return f.Name(), nil
}

// Execute runs a rip job synchronously, calling back for progress and log lines.
func Execute(ctx context.Context, opts ExecuteOptions) error {
	sourcePath := opts.SourcePath
	outputPath := opts.OutputPath
	format := opts.Format

	var logFile *os.File
	if opts.GetLogsDir != nil {
		lf, logPath, logErr := CreateLog(sourcePath, outputPath, format, opts.GetLogsDir(), opts.LogSuffix)
		if logErr != nil {
			logging.Debug(logging.CatSystem, "rip log open failed: %v", logErr)
		} else {
			logFile = lf
			defer logFile.Close()
			if opts.OnLogFileCreated != nil {
				opts.OnLogFileCreated(logPath)
			}
		}
	}

	appendLog := func(line string) {
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		if opts.OnAppendLog != nil {
			opts.OnAppendLog(line)
		}
	}
	updateProgress := func(percent float64) {
		if opts.ProgressCallback != nil {
			opts.ProgressCallback(percent)
		}
		if opts.OnSetProgress != nil {
			opts.OnSetProgress(percent)
		}
	}

	appendLog(fmt.Sprintf("Rip started: %s", time.Now().Format(time.RFC3339)))
	appendLog(fmt.Sprintf("Source: %s", sourcePath))
	appendLog(fmt.Sprintf("Output: %s", outputPath))
	appendLog(fmt.Sprintf("Format: %s", format))

	videoTSPath, cleanup, err := ResolveVideoTSPath(sourcePath)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Check for CSS encryption
	ifoPath := filepath.Join(videoTSPath, "VIDEO_TS.IFO")
	isEncrypted, err := css.IsCSSEncrypted(ifoPath)
	if err != nil {
		appendLog(fmt.Sprintf("Warning: could not check encryption status: %v", err))
		isEncrypted = false
	}

	if isEncrypted {
		appendLog("CSS encryption detected - will decrypt during processing")
	}

	sets, err := CollectVOBSets(videoTSPath)
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return fmt.Errorf("no VOB files found in VIDEO_TS")
	}

	var set VobSet
	if opts.VTSNumber > 0 {
		vtsName := fmt.Sprintf("VTS_%02d", opts.VTSNumber)
		for _, s := range sets {
			if s.Name == vtsName {
				set = s
				break
			}
		}
		if set.Name == "" {
			return fmt.Errorf("VTS_%02d not found on disc", opts.VTSNumber)
		}
	} else {
		set = sets[0]
	}
	appendLog(fmt.Sprintf("Using title set: %s", set.Name))
	listFile, err := BuildConcatList(set.Files)
	if err != nil {
		return err
	}
	defer os.Remove(listFile)

	// Create output directory if it doesn't exist.
	outputDir := outputPath
	if format != FormatArchivist {
		outputDir = filepath.Dir(outputPath)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if format == FormatArchivist {
		return executeArchivist(ctx, opts, set, listFile, outputDir, appendLog, updateProgress)
	}

	// ── IFO enrichment ────────────────────────────────────────────────────────
	// Find the VTS IFO that corresponds to the selected title set (e.g. VTS_01).
	ra := RipArgs{
		ListFile:   listFile,
		OutputPath: outputPath,
		Format:     format,
		DiscTitle:  opts.DiscTitle,
	}

	vtsName := set.Name // e.g. "VTS_01"
	vtsIFO := filepath.Join(videoTSPath, vtsName+"_0.IFO")
	titleInfo, ifoErr := ifo.ReadTitleInfo(vtsIFO)
	if ifoErr != nil {
		appendLog(fmt.Sprintf("Warning: could not read IFO for enrichment: %v", ifoErr))
	}

	if titleInfo != nil {
		ra.Interlaced = titleInfo.Interlaced
		if titleInfo.Interlaced {
			appendLog("Source IFO reports interlaced video — H.264 re-encode will apply yadif deinterlace")
		}
	}

	if titleInfo != nil {
		if titleInfo.HasAngles {
			appendLog("Note: multi-angle content detected — only the primary angle will be ripped")
		}

		// Chapter embedding
		if opts.EmbedChapters && len(titleInfo.Chapters) > 1 {
			appendLog(fmt.Sprintf("Embedding %d chapters", len(titleInfo.Chapters)))
			metaPath, err := WriteChapterFile(titleInfo.Chapters, titleInfo.Duration, opts.DiscTitle)
			if err != nil {
				appendLog(fmt.Sprintf("Warning: chapter file creation failed: %v", err))
			} else {
				defer os.Remove(metaPath)
				ra.MetaFile = metaPath
				ra.DiscTitle = "" // already in metafile; avoid duplicate -metadata title=
			}
		} else if opts.DiscTitle != "" {
			// Title-only metadata file (no chapters)
			metaPath, err := WriteChapterFile(nil, titleInfo.Duration, opts.DiscTitle)
			if err == nil {
				defer os.Remove(metaPath)
				ra.MetaFile = metaPath
				ra.DiscTitle = ""
			}
		}

		// Audio language tags
		if opts.AllAudioTracks {
			for _, t := range titleInfo.Audio {
				ra.AudioLangs = append(ra.AudioLangs, t.Language)
			}
			if len(titleInfo.Audio) > 0 {
				appendLog(fmt.Sprintf("Mapping %d audio track(s)", len(titleInfo.Audio)))
			}
		}

		// Subtitle streams
		if opts.IncludeSubtitles && len(titleInfo.Subtitles) > 0 {
			for _, t := range titleInfo.Subtitles {
				ra.SubtitleLangs = append(ra.SubtitleLangs, t.Language)
			}
			appendLog(fmt.Sprintf("Including %d subtitle stream(s)", len(titleInfo.Subtitles)))
		}
	}

	args := BuildRipArgs(ra)
	appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
	updateProgress(10)
	if err := opts.OnRunCommand(utils.GetFFmpegPath(), args, appendLog); err != nil {
		return err
	}
	updateProgress(100)
	appendLog("Rip completed successfully.")
	return nil
}

func executeArchivist(ctx context.Context, opts ExecuteOptions, set VobSet, listFile, outputDir string, appendLog func(string), updateProgress func(float64)) error {
	appendLog("Archivist Mode: Extracting individual streams for reconstruction...")

	var audio []AudioStream
	var subtitles []SubtitleStream

	if opts.OnProbeVideo != nil {
		if pr, err := opts.OnProbeVideo(set.Files[0]); err == nil {
			audio = pr.Audio
			subtitles = pr.Subtitles
		} else {
			return fmt.Errorf("probe for archivist failed: %w", err)
		}
	}

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", listFile}

	// Map Video
	args = append(args, "-map", "0:v:0", "-c:v", "copy", filepath.Join(outputDir, "video.m2v"))

	// Map all Audio
	for i, at := range audio {
		args = append(args, "-map", fmt.Sprintf("0:%d", at.Index), "-c:a", "copy", filepath.Join(outputDir, fmt.Sprintf("audio_%d_%s.ac3", i, at.Language)))
	}

	// Map all Subtitles
	for i, st := range subtitles {
		args = append(args, "-map", fmt.Sprintf("0:%d", st.Index), "-c:s", "copy", filepath.Join(outputDir, fmt.Sprintf("subs_%d_%s.sup", i, st.Language)))
	}

	appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
	updateProgress(20)
	if err := opts.OnRunCommand(utils.GetFFmpegPath(), args, appendLog); err != nil {
		return err
	}

	// Create project file
	projPath := filepath.Join(outputDir, "author_project.json")
	appendLog(fmt.Sprintf("Creating project file: %s", projPath))

	projJSON := fmt.Sprintf(`{
  "title": %q,
  "type": "dvd",
  "assets": [
    {
      "path": "video.m2v",
      "type": "feature"
    }
  ]
}`, filepath.Base(outputDir))
	_ = os.WriteFile(projPath, []byte(projJSON), 0644)

	updateProgress(100)
	appendLog("Archivist extraction completed successfully.")
	return nil
}

// TryMountISO attempts to mount the ISO and copy VIDEO_TS to a temp directory.
func TryMountISO(isoPath string) (string, func(), error) {
	mountPoint, err := os.MkdirTemp(utils.TempDir(), "videotools-mount-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create mount point: %w", err)
	}

	mountCmd := exec.Command("mount", "-o", "loop,ro", isoPath, mountPoint)
	if err := mountCmd.Run(); err != nil {
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("mount failed: %w", err)
	}

	videoTSMounted := filepath.Join(mountPoint, "VIDEO_TS")
	if info, err := os.Stat(videoTSMounted); err != nil || !info.IsDir() {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("VIDEO_TS not found in mounted ISO")
	}

	tempDir, err := os.MkdirTemp(utils.TempDir(), "videotools-iso-")
	if err != nil {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cpCmd := exec.Command("cp", "-r", videoTSMounted, tempDir)
	if err := cpCmd.Run(); err != nil {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("copy failed: %w", err)
	}

	exec.Command("umount", mountPoint).Run()
	os.RemoveAll(mountPoint)

	videoTS := filepath.Join(tempDir, "VIDEO_TS")
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	return videoTS, cleanup, nil
}

// BuildISOExtractCommand returns the best available ISO extraction command.
func BuildISOExtractCommand(isoPath, destDir string) (string, []string, error) {
	if _, err := exec.LookPath("xorriso"); err == nil {
		return "xorriso", []string{"-osirrox", "on", "-indev", isoPath, "-extract", "/VIDEO_TS", destDir}, nil
	}
	if _, err := exec.LookPath("7z"); err == nil {
		return "7z", []string{"x", "-o" + destDir, isoPath, "VIDEO_TS"}, nil
	}
	if _, err := exec.LookPath("bsdtar"); err == nil {
		return "bsdtar", []string{"-C", destDir, "-xf", isoPath, "VIDEO_TS"}, nil
	}
	return "", nil, fmt.Errorf("no ISO extraction tool found (install xorriso, 7z, or bsdtar)")
}
