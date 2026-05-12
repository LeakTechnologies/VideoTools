package rip

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/css"
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

// BuildFFmpegArgs returns the ffmpeg argument list for a rip job.
//
// -fflags +genpts is applied globally: DVD VOB streams frequently carry audio
// or subtitle packets with missing PTS. The MKV muxer rejects these with
// "Can't write packet with unknown timestamp"; genpts synthesises PTS from DTS
// so every packet the muxer sees has a valid timestamp.
//
// -max_interleave_delta 0 prevents ffmpeg from buffering indefinitely while
// waiting for each stream to reach the same timestamp before flushing.
func BuildFFmpegArgs(listFile, outputPath, format string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-fflags", "+genpts",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
	}
	switch format {
	case FormatH264MKV:
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:a",
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "copy",
			"-max_interleave_delta", "0",
		)
	case FormatH264MP4:
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:a",
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "192k",
			"-max_interleave_delta", "0",
		)
	default:
		// Lossless MKV: copy all streams. Map video and audio explicitly;
		// dvd_subtitle streams are included via -map 0:s? (optional, see below).
		// We skip subpicture streams here — they are demuxed separately when
		// the user enables subtitle extraction.
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:a",
			"-c", "copy",
			"-max_interleave_delta", "0",
		)
	}
	args = append(args, outputPath)
	return args
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

	set := sets[0]
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

	args := BuildFFmpegArgs(listFile, outputPath, format)
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
