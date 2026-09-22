package rip

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/css"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// DefaultOutputPath returns the default output path for a rip job.
// dvdVideoSupported caches whether the FFmpeg binary supports -f dvdvideo.
var dvdVideoSupported bool
var dvdVideoChecked bool
var dvdVideoCheckMu sync.Mutex

// SupportsDVDVideo returns true if the FFmpeg binary has the dvdvideo demuxer.
// The dvdvideo demuxer (FFmpeg ≥ 6.0) reads IFO cell playback tables directly,
// correctly handling seamless branching discs.
func SupportsDVDVideo() bool {
	dvdVideoCheckMu.Lock()
	defer dvdVideoCheckMu.Unlock()
	if dvdVideoChecked {
		return dvdVideoSupported
	}
	dvdVideoChecked = true

	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-h", "demuxer=dvdvideo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logging.Debug(logging.CatDVD, "SupportsDVDVideo: ffmpeg -h demuxer=dvdvideo failed: %v", err)
		return false
	}
	// Probe for the demuxer's short name: `ffmpeg -h demuxer=dvdvideo` prints
	// "dvdvideo demuxer:" (long_name "DVD-Video" is hyphenated, not spaced).
	dvdVideoSupported = strings.Contains(string(out), "dvdvideo")
	logging.Info(logging.CatDVD, "SupportsDVDVideo: %v", dvdVideoSupported)
	return dvdVideoSupported
}

func DefaultOutputPath(sourcePath, format string) string {
	if sourcePath == "" {
		return ""
	}
	name := SanitizeForPath(sourceBaseName(sourcePath))
	if name == "" {
		name = "dvd_rip"
	}
	return UniqueFilePath(filepath.Join(defaultOutputDir(sourcePath), name+formatExt(format)))
}

// DefaultOutputTitlePath returns a default output path whose filename is
// driven by the user-facing title instead of the source folder's base name
// (the rip view calls it when the user sets a Title). Falls back to the
// folder-derived name when the title is empty or sanitises to nothing.
func DefaultOutputTitlePath(sourcePath, format, title string) string {
	if sourcePath == "" {
		return ""
	}
	name := SanitizeForPath(title)
	if name == "" {
		return DefaultOutputPath(sourcePath, format)
	}
	return UniqueFilePath(filepath.Join(defaultOutputDir(sourcePath), name+formatExt(format)))
}

func defaultOutputDir(sourcePath string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, "Videos", "VideoTools", "DVD_Rips")
}

func sourceBaseName(sourcePath string) string {
	name := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if strings.EqualFold(name, "video_ts") {
		name = filepath.Base(filepath.Dir(sourcePath))
	}
	return name
}

func formatExt(format string) string {
	if format == FormatH264MP4 {
		return ".mp4"
	}
	return ".mkv"
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
Version: %s
Started: %s
Source: %s
Output: %s
Format: %s

`, logging.Version(), time.Now().Format(time.RFC3339), inputPath, outputPath, format)
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

// ResolveVideoTSPath returns the VIDEO_TS (or BDMV) directory from a source path.
// For ISO files it extracts to a temp dir; cleanup must be called when done.
// ctx is forwarded to the UDF extractor and may be used to cancel ISO extraction.
func ResolveVideoTSPath(ctx context.Context, path string) (string, func(), error) {
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
		return resolveFromISO(ctx, path)
	}
	// User may have selected an IFO/VOB file from within a VIDEO_TS directory via
	// the file browser (file dialogs return files, not folders). Resolve to the
	// containing VIDEO_TS dir, or its parent's VIDEO_TS sibling.
	dir := filepath.Dir(path)
	if strings.EqualFold(filepath.Base(dir), "VIDEO_TS") {
		return dir, nil, nil
	}
	videoTS := filepath.Join(dir, "VIDEO_TS")
	if fi, err := os.Stat(videoTS); err == nil && fi.IsDir() {
		return videoTS, nil, nil
	}
	return "", nil, fmt.Errorf("unsupported source: %s", path)
}

func resolveFromISO(ctx context.Context, isoPath string) (string, func(), error) {
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

	return resolveISOWithUDF(ctx, f, isoPath, tempDir, cleanup)
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
		// VTS_XX_0.VOB is the menu VOB for this title set — exclude it from
		// the content VOB set to avoid menu data bleeding into the ripped video.
		// Menu VOBs are collected separately for optional menu-only export.
		if parts[len(parts)-1] == "0" {
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
	ListFile      string
	OutputPath    string
	Format        string
	MetaFile      string   // path to ffmetadata file; empty = no chapter/title metadata
	AudioLangs    []string // per-stream ISO 639-1 language codes; nil = no tagging
	SubtitleLangs []string // per-stream subtitle language codes; nil = no subs
	SubtitleSel   []int    // source stream indices of the selected subs (parallel to SubtitleLangs); empty = map all
	DiscTitle     string   // embedded title tag; empty = skip
	MaxDuration   float64  // output cap in seconds; <=0 disables. Used on the VOB-concat fallback to stop before phantom-tail packets carrying a stale PTS offset
	Interlaced    bool     // when true and format is H.264, adds yadif=mode=1 deinterlace filter
	RegionConvert string   // "" (none), "pal2ntsc", "ntsc2pal"
	VideoTSPath   string   // VIDEO_TS directory for -f dvdvideo (seamless branching)
	TitleNumber   int      // 1-based title index from VMG TT_SRPT for -f dvdvideo

	// ChapterStartSec/ChapterEndSec trim the output to a chapter range as
	// output-side -ss/-to. Start > 0 enables the trim; End must be > Start.
	// Used on both the dvdvideo path and the whole-file VOB-concat path when
	// the cell-accurate list cannot restrict the span itself.
	ChapterStartSec float64
	ChapterEndSec   float64
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
	args := []string{"-y", "-hide_banner", "-loglevel", "error"}

	if ra.VideoTSPath != "" && ra.TitleNumber > 0 {
		// -f dvdvideo reads the IFO cell playback table natively,
		// correctly handling seamless branching discs.
		// -title MUST precede -i: a demuxer option placed after -i binds
		// to the NEXT input, so putting it on the far side of the chapters
		// ffmetadata input failed with "Option title not found" and sent
		// every rip down the VOB-concat fallback.
		args = append(args, "-f", "dvdvideo", "-title", fmt.Sprintf("%d", ra.TitleNumber), "-i", ra.VideoTSPath)
	} else {
		args = append(args, "-fflags", "+genpts", "-f", "concat", "-safe", "0", "-i", ra.ListFile)
	}

	metaInputIdx := -1
	if ra.MetaFile != "" {
		args = append(args, "-f", "ffmetadata", "-i", ra.MetaFile)
		metaInputIdx = 1
	}

	// Stream mapping
	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "0:a?")
	// dvd_subtitle (VOBSUB bitmap) is valid in MKV but not in MP4
	// Map only the subtitle streams the user selected, targeting each chosen
	// stream's source index (VTS subpicture order = demuxed stream order).
	// SubtitleSel empty + SubtitleLangs set = map all (legacy callers).
	if len(ra.SubtitleLangs) > 0 && ra.Format != FormatH264MP4 {
		if len(ra.SubtitleSel) == 0 {
			args = append(args, "-map", "0:s?")
		} else {
			for _, idx := range ra.SubtitleSel {
				args = append(args, "-map", "0:s:"+strconv.Itoa(idx))
			}
		}
	}

	// Metadata source
	if metaInputIdx >= 0 {
		args = append(args, "-map_metadata", fmt.Sprintf("%d", metaInputIdx))
		args = append(args, "-map_chapters", fmt.Sprintf("%d", metaInputIdx))
	} else {
		args = append(args, "-map_metadata", "-1") // strip existing metadata
	}

	// Video filter chain — built before codec args so -vf precedes -c:v.
	// RegionConvert takes priority and includes deinterlace + scale + fps in one pass.
	// Interlaced-only falls back to a plain yadif when no region conversion is set.
	isH264 := ra.Format == FormatH264MKV || ra.Format == FormatH264MP4
	switch {
	case ra.RegionConvert == "pal2ntsc" && isH264:
		args = append(args, "-vf", "yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001")
		logging.Info(logging.CatDVD, "BuildRipArgs: PAL→NTSC — yadif + scale 720×480 + 29.97 fps")
	case ra.RegionConvert == "ntsc2pal" && isH264:
		args = append(args, "-vf", "yadif=mode=1,scale=720:576:flags=lanczos,fps=25")
		logging.Info(logging.CatDVD, "BuildRipArgs: NTSC→PAL — yadif + scale 720×576 + 25 fps")
	case ra.Interlaced && isH264:
		args = append(args, "-vf", "yadif=mode=1")
		logging.Info(logging.CatDVD, "BuildRipArgs: interlaced source — yadif=mode=1")
	}

	switch ra.Format {
	case FormatH264MKV:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "copy",
		)
		switch ra.RegionConvert {
		case "pal2ntsc":
			args = append(args, "-af", "atempo=0.9600")
		case "ntsc2pal":
			args = append(args, "-af", "atempo=1.0417")
		}
	case FormatH264MP4:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "192k",
		)
		switch ra.RegionConvert {
		case "pal2ntsc":
			args = append(args, "-af", "atempo=0.9600")
		case "ntsc2pal":
			args = append(args, "-af", "atempo=1.0417")
		}
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

	if ra.MaxDuration > 0 {
		args = append(args, "-t", strconv.FormatFloat(ra.MaxDuration, 'f', 3, 64))
	}

	// Chapter-range trim as output options. ffmpeg normalises the output
	// timeline to start at 0, which keeps the remapped chapters metafile
	// aligned. Applied to the stream after demuxing on both the dvdvideo and
	// the VOB-concat inputs (when a cell-accurate list did not already bound
	// the span).
	if ra.ChapterStartSec > 0 {
		args = append(args, "-ss", strconv.FormatFloat(ra.ChapterStartSec, 'f', 3, 64))
		if ra.ChapterEndSec > ra.ChapterStartSec {
			args = append(args, "-to", strconv.FormatFloat(ra.ChapterEndSec, 'f', 3, 64))
		}
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

// chapterRange resolves a 1-based inclusive chapter range (cs..ce) against a
// title's chapter start times into an output-side trim. Returns the remapped
// chapter list (start times relative to the range base, so the output
// timeline starts at 0 and chapter N still sits at its boundary), the absolute
// start second, the absolute exclusive end second, the range duration, and
// whether the range is usable. cs < 1 or ce <= 0 mean "whole title".
func chapterRange(chapters []float64, duration float64, cs, ce int) (remap []float64, base float64, endSec float64, dur float64, ok bool) {
	n := len(chapters)
	if n == 0 {
		return nil, 0, 0, 0, false
	}
	if cs < 1 {
		cs = 1
	}
	if ce == 0 || ce > n {
		ce = n
	}
	if cs > n || ce < cs {
		return nil, 0, 0, 0, false
	}
	base = chapters[cs-1]
	endSec = duration
	if ce < n {
		endSec = chapters[ce]
	}
	if endSec < base {
		endSec = base
	}
	dur = endSec - base
	for i := cs - 1; i < ce; i++ {
		remap = append(remap, chapters[i]-base)
	}
	return remap, base, endSec, dur, len(remap) > 0
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

// resolveVTS_TTN maps a VMG-level title number (1-based TT_SRPT order, as the
// rip UI passes it) to the per-VTS title number the IFO reader needs to select
// that title's PGC. Returns 0 (first title-domain PGC) when the VMG cannot be
// read, the title is out of range, or the title belongs to a different VTS
// than the one being ripped — the safe fallback for mixed-VTS selection.
func resolveVTS_TTN(videoTSPath string, titleNum, setVTS int) int {
	if titleNum <= 0 {
		return 0
	}
	tsps, err := ifo.ReadTitleList(filepath.Join(videoTSPath, "VIDEO_TS.IFO"))
	if err != nil || len(tsps) < titleNum {
		return 0
	}
	t := tsps[titleNum-1]
	if setVTS > 0 && int(t.VTSNumber) != setVTS {
		return 0
	}
	return int(t.VTS_TitleNumber)
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

	appendLog("VideoTools Rip Log")
	appendLog(fmt.Sprintf("Version: %s", logging.Version()))
	appendLog(fmt.Sprintf("Rip started: %s", time.Now().Format(time.RFC3339)))
	appendLog(fmt.Sprintf("Source: %s", sourcePath))
	appendLog(fmt.Sprintf("Output: %s", outputPath))
	appendLog(fmt.Sprintf("Format: %s", format))

	videoTSPath, cleanup, err := ResolveVideoTSPath(ctx, sourcePath)
	if err != nil {
		appendLog(fmt.Sprintf("Error resolving source path: %v", err))
		return fmt.Errorf("resolve source: %w", err)
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

	// Full-disc extraction mode — processes all VTS sets + menu, regenerates IFOs.
	if opts.ExtractMode == "full" {
		return executeFullDiscRip(ctx, opts, videoTSPath, isEncrypted, appendLog, updateProgress)
	}

	sets, err := CollectVOBSets(videoTSPath)
	if err != nil {
		appendLog(fmt.Sprintf("Error collecting VOB sets: %v", err))
		return fmt.Errorf("collect VOB sets: %w", err)
	}
	if len(sets) == 0 {
		appendLog("Error: no VOB files found in VIDEO_TS — ISO extraction may have produced an incomplete result")
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
			appendLog(fmt.Sprintf("Error: VTS_%02d not found on disc", opts.VTSNumber))
			return fmt.Errorf("VTS_%02d not found on disc", opts.VTSNumber)
		}
	} else {
		set = sets[0]
	}
	appendLog(fmt.Sprintf("Using title set: %s", set.Name))

	// Prefer -f dvdvideo when available — it reads the IFO cell playback table
	// natively, correctly handling seamless branching discs and producing
	// continuous timestamps across VOB file boundaries (concat + -c copy can
	// write raw PTS discontinuities that crash external players).
	titleNum := opts.TitleNumber
	useDVDVideo := opts.ExtractMode != "full" && format != FormatArchivist && SupportsDVDVideo()
	if useDVDVideo {
		if titleNum == 0 {
			titleNum = 1
			appendLog("No title selected — dvdvideo demuxer will use title 1")
		} else {
			appendLog(fmt.Sprintf("FFmpeg dvdvideo demuxer available — using cell-accurate title playback (title %d)", titleNum))
		}
	} else {
		if opts.TitleNumber > 0 && !SupportsDVDVideo() {
			appendLog("FFmpeg dvdvideo demuxer not available — falling back to VOB concatenation (may break on seamless branching discs)")
		}
	}

	// Build the concat list up-front regardless of path. It is the reliable
	// fallback when -f dvdvideo cannot open the source or fails mid-run. The
	// list is cheap (a temp file); it lets us retry with VOB concat after a
	// dvdvideo open/run failure instead of hard-failing the rip.
	listFile, err := BuildConcatList(set.Files)
	if err != nil {
		appendLog(fmt.Sprintf("Error building concat list: %v", err))
		return fmt.Errorf("build concat list: %w", err)
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
	if useDVDVideo {
		ra.VideoTSPath = videoTSPath
		ra.TitleNumber = titleNum
	}

	vtsName := set.Name // e.g. "VTS_01"
	vtsIFO := filepath.Join(videoTSPath, vtsName+"_0.IFO")
	// Read the PGC for the selected title, not just the first title-domain
	// PGC: on multi-PGC discs (scene-segmented titles) each title's duration,
	// chapter points, and angle cells live in its own PGC, and the first PGC's
	// values would describe the wrong title.
	setVTS := opts.VTSNumber
	if setVTS == 0 && strings.HasPrefix(set.Name, "VTS_") {
		fmt.Sscanf(set.Name[4:], "%d", &setVTS)
	}
	ttn := resolveVTS_TTN(videoTSPath, opts.TitleNumber, setVTS)
	titleInfo, ifoErr := ifo.ReadTitleInfoForTTN(vtsIFO, ttn)
	if ifoErr != nil {
		appendLog(fmt.Sprintf("Warning: could not read IFO for enrichment: %v", ifoErr))
	}

	// Resolve an optional chapter range against the title's PGC chapters. The
	// range is expressed in 1-based inclusive chapter numbers; 0 means the
	// whole title. When active, the output is trimmed to the span (output-side
	// -ss/-to on both demux paths), the cell-accurate concat list is restricted
	// to exactly the span's cells, and the embedded chapters are remapped so
	// chapter N of the ripped file still marks the same boundary.
	var remapChapters []float64
	concatCS, concatCE := 0, 0
	rangeDur := 0.0
	if opts.ChapterStart > 0 && titleInfo != nil && len(titleInfo.Chapters) > 0 {
		cs, ce := opts.ChapterStart, opts.ChapterEnd
		n := len(titleInfo.Chapters)
		if ce == 0 || ce > n {
			ce = n
		}
		if cs > n {
			cs = n
		}
		// A range spanning every chapter is the whole title — leave the
		// pipeline untouched so dvdvideo/cell behaviour is exactly today's.
		if !(cs == 1 && ce == n) {
			remap, base, endSec, durRange, ok := chapterRange(titleInfo.Chapters, titleInfo.Duration, cs, ce)
			if ok {
				concatCS, concatCE = cs, ce
				rangeDur = durRange
				remapChapters = remap
				ra.ChapterStartSec = base
				ra.ChapterEndSec = endSec
				appendLog(fmt.Sprintf("Chapter range: chapters %d–%d of %d → %.2fs–%.2fs (%.2fs span)",
					cs, ce, n, base, endSec, durRange))
			}
		}
	}

	if titleInfo != nil {
		ra.Interlaced = titleInfo.Interlaced
		ra.RegionConvert = opts.RegionConvert
		switch opts.RegionConvert {
		case "pal2ntsc":
			appendLog("PAL→NTSC conversion enabled — yadif deinterlace + scale 720×480 + 29.97 fps + pitch correction")
		case "ntsc2pal":
			appendLog("NTSC→PAL conversion enabled — yadif deinterlace + scale 720×576 + 25 fps + pitch correction")
		default:
			if titleInfo.Interlaced {
				appendLog("Source IFO reports interlaced video — H.264 re-encode will apply yadif deinterlace")
			}
		}
	}

	if titleInfo != nil {
		if titleInfo.HasAngles {
			appendLog("Note: multi-angle content detected — only the primary angle will be ripped")
		}

		// Chapter embedding
		if opts.EmbedChapters && len(titleInfo.Chapters) > 1 {
			chapTimes := titleInfo.Chapters
			chapDur := titleInfo.Duration
			if len(remapChapters) > 0 {
				chapTimes = remapChapters
				chapDur = rangeDur
				appendLog(fmt.Sprintf("Embedding %d chapters remapped to the chapter range (span=%.2fs)", len(chapTimes), chapDur))
			} else {
				appendLog(fmt.Sprintf("Embedding %d chapters (duration=%.2fs, chapter[0]=%.2fs, chapter[%d]=%.2fs)",
					len(titleInfo.Chapters), titleInfo.Duration,
					titleInfo.Chapters[0], len(titleInfo.Chapters)-1, titleInfo.Chapters[len(titleInfo.Chapters)-1]))
			}
			metaPath, err := WriteChapterFile(chapTimes, chapDur, opts.DiscTitle)
			if err != nil {
				appendLog(fmt.Sprintf("Warning: chapter file creation failed: %v", err))
			} else {
				defer os.Remove(metaPath)
				ra.MetaFile = metaPath
				ra.DiscTitle = "" // already in metafile; avoid duplicate -metadata title=
			}
		} else if opts.DiscTitle != "" {
			// Title-only metadata file (no chapters)
			appendLog(fmt.Sprintf("No chapters to embed (%d chapter points from IFO, embed=%v)", len(titleInfo.Chapters), opts.EmbedChapters))
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

		// Subtitle streams — map only the languages the user chose. VTS subpicture
		// order defines the demuxed subtitle stream order, so each chosen
		// language's source index is recorded in SubtitleSel and the -map flags
		// will target exactly those streams. An empty selection list maps every
		// subtitle stream (legacy include-all behaviour).
		if opts.IncludeSubtitles && len(titleInfo.Subtitles) > 0 {
			selected := map[string]bool{}
			for _, l := range opts.SelectedSubtitleLangs {
				selected[strings.ToUpper(l)] = true
			}
			for i, t := range titleInfo.Subtitles {
				if len(selected) > 0 && !selected[strings.ToUpper(t.Language)] {
					continue
				}
				ra.SubtitleLangs = append(ra.SubtitleLangs, t.Language)
				ra.SubtitleSel = append(ra.SubtitleSel, i)
			}
			if len(ra.SubtitleLangs) > 0 {
				appendLog(fmt.Sprintf("Including %d subtitle stream(s): %s",
					len(ra.SubtitleLangs), strings.Join(ra.SubtitleLangs, ", ")))
			}
		}
	}

	// On the VOB-concat path, prefer a cell-accurate input list over whole-file
	// concatenation. On multi-PGC VTS discs (scene-segmented "extras" titles
	// sharing one VOB set — e.g. the Red Hairy Teens disc's seven titles in
	// VTS_01), whole-file concat reads the movie's opening for every non-first
	// title; slicing the VOBs to the selected title's PGC cell sectors yields
	// the actual title content. The dvdvideo path reads the IFO natively and
	// needs no slicing — but when it fails, the retry below rebuilds the concat
	// list cell-accurately the same way.
	if !useDVDVideo {
		cellList, cellCleanup, cellErr := cellConcatList(videoTSPath, set, titleInfo, concatCS, concatCE)
		if cellErr != nil {
			appendLog(fmt.Sprintf("Warning: cell-accurate concat unavailable: %v — using whole-file concatenation", cellErr))
		} else if cellList != "" {
			appendLog(fmt.Sprintf("VOB concat: using cell-accurate input list (%d cell ranges) for the selected title",
				len(titleInfo.Cells)))
			listFile = cellList
			defer cellCleanup()
			ra.ListFile = cellList
			// The cell slice already bounds the span exactly; output-side -ss/-to
			// would double-trim.
			ra.ChapterStartSec, ra.ChapterEndSec = 0, 0
		} else if titleInfo != nil && len(titleInfo.Cells) > 0 {
			if concatCS > 0 {
				appendLog("VOB concat: chapter range cell span unresolved — using whole-file list with output-side trim")
			} else {
				appendLog("VOB concat: the selected title's PGC cells cover the whole VOB set — whole-file list is exact")
			}
		}
	}

	// runWithArgs builds the ffmpeg command from a RipArgs struct and runs it,
	// returning any error. Extracted into a closure so the rip can be retried
	// with the VOB concat path after a dvdvideo open/run failure.
	dur := 0.0
	if titleInfo != nil {
		dur = titleInfo.Duration
	}
	if rangeDur > 0 {
		// Progress and percent-complete track the chapter range, not the whole
		// title — the output is only the span.
		dur = rangeDur
	}
	if dur <= 0 && len(set.Files) > 0 {
		probed := probeDuration(set.Files[0], opts.OnRunCommand, appendLog)
		if probed > 0 {
			dur = probed
			appendLog(fmt.Sprintf("Probed duration from VOB: %.1f seconds", dur))
		}
	}
	updateStatus := func(msg string) {
		if opts.OnSetStatus != nil {
			opts.OnSetStatus(msg)
		}
	}
	runWithArgs := func(r RipArgs) error {
		args := BuildRipArgs(r)
		// Insert progress tracking before the output path (last arg)
		progressArgs := []string{"-progress", "pipe:1", "-nostats"}
		args = append(args[:len(args)-1], append(progressArgs, args[len(args)-1])...)
		appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
		updateProgress(10)
		return runFFmpegWithProgress(ctx, utils.GetFFmpegPath(), args, dur, updateProgress, appendLog, updateStatus)
	}

	err = runWithArgs(ra)
	if err != nil && useDVDVideo && ctx.Err() == nil {
		// -f dvdvideo opened/ran but failed (demux error, CSS auth unavailable,
		// etc.). Retry with the reliable VOB concat path.
		// Chapter/audio/subtitle enrichment is preserved; only the demuxer input
		// changes. Concat + -c copy can write PTS discontinuities at VOB
		// boundaries, so log the fallback for diagnosis. Do NOT fall back on
		// context cancellation (the user aborted the rip).
		appendLog(fmt.Sprintf("dvdvideo demuxer failed (%v) — retrying with VOB concatenation", err))
		ra.VideoTSPath = ""
		ra.TitleNumber = 0
		// When the disc's titles share one VOB set, whole-file concatenation
		// would rip the movie's opening for a scene-segmented extra title. Use
		// a cell-accurate list (VOBs sliced to the selected title's PGC cell
		// sector ranges) whenever the IFO provides per-title cell metadata.
		if cellList, cellCleanup, cellErr := cellConcatList(videoTSPath, set, titleInfo, concatCS, concatCE); cellErr != nil {
			appendLog(fmt.Sprintf("Warning: cell-accurate concat unavailable: %v — using whole-file concatenation", cellErr))
			ra.ListFile = listFile
		} else if cellList != "" {
			if titleInfo != nil {
				appendLog(fmt.Sprintf("VOB concat fallback: using cell-accurate input list (%d cell ranges) for the selected title",
					len(titleInfo.Cells)))
			}
			defer cellCleanup()
			ra.ListFile = cellList
			// The cell slice already bounds the span exactly; output-side -ss/-to
			// would double-trim.
			ra.ChapterStartSec, ra.ChapterEndSec = 0, 0
		} else {
			ra.ListFile = listFile
		}
		// Clamp the subtitle mapping to what the concat input actually exposes.
		// Some discs' IFOs advertise more subtitle languages than the VOBs carry
		// physical subpicture streams for (common on grey-market/bootleg media) —
		// the per-stream -map 0:s:<idx> flags then reference streams that don't
		// exist and ffmpeg hard-fails the fallback. Probing the concat input and
		// keeping only the leading languages' indices makes the rip succeed while
		// preserving correct labels for the streams that are actually present.
		if len(ra.SubtitleLangs) > 0 {
			subCount := probeSubtitleCount(listFile, opts.OnRunCommand, appendLog)
			if subCount >= 0 && subCount < len(ra.SubtitleLangs) {
				appendLog(fmt.Sprintf("VOB concatenation exposes %d subtitle stream(s) (IFO advertised %d) — dropping %d trailing subtitle mapping(s)",
					subCount, len(ra.SubtitleLangs), len(ra.SubtitleLangs)-subCount))
				ra.SubtitleLangs = ra.SubtitleLangs[:subCount]
				ra.SubtitleSel = ra.SubtitleSel[:subCount]
			}
		}
		// Cap the concat output at a safe duration. Grey-market discs can carry
		// trailing phantom packets (or whole VOB tails) stamped with a stale PTS
		// offset far beyond the real title ("26hrs" for a 30-min rip) — with
		// -c copy those packets land in the MKV and inflate its reported
		// duration even though the real content is intact. A -t cap stops the
		// muxer before them while preserving all real content.
		//
		// A chapter range already bounds the span precisely (cells or -to), so
		// the cap is just a 5 s safety margin over the range duration — the
		// phantom-tail risk is identical and the probes would be wasted.
		//
		// Otherwise the cap is taken from the per-VOB media durations when they
		// look sane (dev69 behaviour), else from the IFO's authored PGC play
		// time. The IFO fallback matters because per-VOB probes are NOT immune
		// to the offset: on some discs the stale PTS bakes into an entire VOB,
		// so that VOB probes to +hours and either fails the app's static
		// ffprobe or produces an absurd sum that would cap nothing.
		if rangeDur > 0 {
			ra.MaxDuration = math.Ceil(rangeDur + 5)
			appendLog(fmt.Sprintf("VOB concat fallback: capping output at %.0f s (chapter range span %.2fs + 5 s margin)",
				ra.MaxDuration, rangeDur))
		} else {
			maxDur := 0.0
			allProbed := len(set.Files) > 0
			for _, f := range set.Files {
				d := probeDuration(f, opts.OnRunCommand, appendLog)
				if d <= 0 {
					allProbed = false
					break
				}
				maxDur += d
			}
			capDur := 0.0
			capDesc := ""
			if allProbed && maxDur > 0 && (titleInfo == nil || titleInfo.Duration <= 0 || maxDur < titleInfo.Duration*3) {
				capDur = maxDur
				capDesc = fmt.Sprintf("sum of %d VOB duration(s) %.0f s + 60 s margin", len(set.Files), maxDur)
			} else if !allProbed && titleInfo != nil && titleInfo.Duration > 0 {
				capDur = titleInfo.Duration
				capDesc = "IFO PGC duration (per-VOB duration probes failed) + 60 s margin"
			} else if titleInfo != nil && titleInfo.Duration > 0 {
				capDur = titleInfo.Duration
				capDesc = "IFO PGC duration (per-VOB duration sum is stale) + 60 s margin"
			}
			if capDur > 0 {
				ra.MaxDuration = math.Ceil(capDur + 60)
				appendLog(fmt.Sprintf("VOB concat: capping output at %.0f s (%s) — prevents stale-PTS tail packets inflating the rip duration",
					ra.MaxDuration, capDesc))
			} else {
				appendLog("VOB concat: could not determine a safe output duration — leaving output duration uncapped")
			}
		}
		if err2 := runWithArgs(ra); err2 != nil {
			return err2
		}
	} else if err != nil {
		return err
	}

	// ── Menu export ───────────────────────────────────────────────────────────
	// After the main content rip succeeds, export menu VOBs as separate files
	// if the user opted to preserve menus. The same menus are re-collected on
	// every title rip of a disc, so skip a menu when an identical one (same
	// source VOB size, same label) was already exported into the output
	// directory — ripping 11 titles must not produce 11 copies of each menu.
	if opts.IncludeMenus && videoTSPath != "" {
		menuSets := CollectAllMenuVOBs(videoTSPath)
		if len(menuSets) > 0 {
			appendLog(fmt.Sprintf("Exporting %d menu VOB(s) as separate files...", len(menuSets)))
			ext := filepath.Ext(outputPath)
			base := strings.TrimSuffix(outputPath, ext)
			outDir := filepath.Dir(outputPath)
			for i, ms := range menuSets {
				menuLabel := ms.Name
				menuOut := fmt.Sprintf("%s_Menu_%s%s", base, menuLabel, ext)
				if menuAlreadyExported(outDir, menuLabel, ext, ms.Size) {
					appendLog(fmt.Sprintf("[%d/%d] Menu: %s already exported (same menu content) — skipping", i+1, len(menuSets), ms.Name))
					continue
				}
				appendLog(fmt.Sprintf("[%d/%d] Menu: %s → %s", i+1, len(menuSets), ms.Name, filepath.Base(menuOut)))
				if err := exportMenuVOB(ctx, opts, ms.Files[0], menuOut, format, updateStatus); err != nil {
					appendLog(fmt.Sprintf("Warning: menu export failed for %s: %v", ms.Name, err))
					continue
				}
				appendLog(fmt.Sprintf("Menu exported: %s", menuOut))
			}
		}
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

// CollectMenuVOB returns the VIDEO_TS.VOB menu VOB as a single-entry VobSet.
// Returns nil if the menu VOB doesn't exist.
func CollectMenuVOB(videoTS string) *VobSet {
	menuVOB := filepath.Join(videoTS, "VIDEO_TS.VOB")
	info, err := os.Stat(menuVOB)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return nil
	}
	return &VobSet{
		Name:  "VIDEO_TS",
		Files: []string{menuVOB},
		Size:  info.Size(),
	}
}

// CollectAllMenuVOBs returns all menu VOBs found in a VIDEO_TS directory:
// the VMG menu (VIDEO_TS.VOB) and any VTS-level menus (VTS_XX_0.VOB).
func CollectAllMenuVOBs(videoTS string) []VobSet {
	var menus []VobSet

	// VMG menu
	if vmg := CollectMenuVOB(videoTS); vmg != nil {
		menus = append(menus, *vmg)
	}

	// VTS-level menus (VTS_XX_0.VOB)
	entries, err := os.ReadDir(videoTS)
	if err != nil {
		return menus
	}
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
		// Only VTS_XX_0.VOB (menu VOBs)
		if parts[len(parts)-1] != "0" {
			continue
		}
		full := filepath.Join(videoTS, name)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		menus = append(menus, VobSet{
			Name:  parts[0] + "_" + parts[1],
			Files: []string{full},
			Size:  info.Size(),
		})
	}
	return menus
}

// menuAlreadyExported reports whether a menu file for the given label already
// exists in the output directory with the same size as the source menu VOB.
// Every title rip of a disc re-collects the same menu VOBs; the base prefix in
// the name it would use changes per title, so the dedup key is (label, size)
// rather than the exact filename. A size match is treated as "same menu
// content" — identical menu VOBs on a disc always extract to identical sizes.
func menuAlreadyExported(outDir, menuLabel, ext string, wantSize int64) bool {
	pattern := "*_Menu_" + menuLabel + ext
	matches, err := filepath.Glob(filepath.Join(outDir, pattern))
	if err != nil {
		return false
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.Size() == wantSize {
			return true
		}
	}
	return false
}

// exportMenuVOB copies or re-encodes a menu VOB to the output path using the
// same format as the main rip. This is called after a successful content rip
// when the user opted to preserve menus.
func exportMenuVOB(ctx context.Context, opts ExecuteOptions, menuVOBPath, outputPath, format string, updateStatus func(string)) error {
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-fflags", "+genpts",
		"-i", menuVOBPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-map", "0:s?",
		"-map_metadata", "-1",
	}

	switch format {
	case FormatH264MKV:
		args = append(args, "-c:v", "libx264", "-crf", "18", "-preset", "medium", "-c:a", "copy")
	case FormatH264MP4:
		args = append(args, "-c:v", "libx264", "-crf", "18", "-preset", "medium", "-c:a", "aac", "-b:a", "192k")
	default:
		args = append(args, "-c", "copy")
	}

	args = append(args, "-max_interleave_delta", "0", outputPath)

	if updateStatus != nil {
		updateStatus(fmt.Sprintf("Exporting menu: %s", filepath.Base(outputPath)))
	}
	if opts.OnRunCommand != nil {
		return opts.OnRunCommand(utils.GetFFmpegPath(), args, nil)
	}
	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)
	return cmd.Run()
}

// FullDiscOutputPath returns the directory path for full-disc extraction output.
// The output is always a VIDEO_TS directory structure.
func FullDiscOutputPath(sourcePath string) string {
	if sourcePath == "" {
		return ""
	}
	name := SanitizeForPath(sourceBaseName(sourcePath))
	if name == "" {
		name = "dvd_disc"
	}
	return UniqueFilePath(filepath.Join(defaultOutputDir(sourcePath), name))
}

// FullDiscOutputTitlePath returns the full-disc extraction output directory
// driven by the user-facing title instead of the source folder's base name
// (mirrors DefaultOutputTitlePath for the single-file rips). Falls back to the
// source-derived name when the title is empty or sanitises to nothing.
func FullDiscOutputTitlePath(sourcePath, title string) string {
	if sourcePath == "" {
		return ""
	}
	name := SanitizeForPath(title)
	if name == "" {
		return FullDiscOutputPath(sourcePath)
	}
	return UniqueFilePath(filepath.Join(defaultOutputDir(sourcePath), name))
}

// executeFullDiscRip runs full-disc extraction with region conversion and IFO regeneration.
// Stages 1-3 combined: extracts all VTS sets + menu VOB, applies region conversion,
// and regenerates IFO/BUP files with correct NTSC/PAL timing.
func executeFullDiscRip(ctx context.Context, opts ExecuteOptions, videoTSPath string, isEncrypted bool, appendLog func(string), updateProgress func(float64)) error {
	outputDir := opts.OutputPath

	// Collect all VTS sets
	sets, err := CollectVOBSets(videoTSPath)
	if err != nil {
		appendLog(fmt.Sprintf("Error collecting VOB sets: %v", err))
		return fmt.Errorf("collect VOB sets: %w", err)
	}
	if len(sets) == 0 {
		appendLog("Error: no VOB files found in VIDEO_TS")
		return fmt.Errorf("no VOB files found in VIDEO_TS")
	}

	// Collect menu VOB
	menuSet := CollectMenuVOB(videoTSPath)
	if menuSet != nil {
		appendLog(fmt.Sprintf("Found menu VOB: VIDEO_TS.VOB (%d MB)", menuSet.Size/(1024*1024)))
	}

	// Determine conversion direction
	isNTSC := true // default for output
	pal2ntsc := opts.RegionConvert == "pal2ntsc"
	ntsc2pal := opts.RegionConvert == "ntsc2pal"
	hasConversion := pal2ntsc || ntsc2pal

	if hasConversion {
		dir := "NTSC"
		if ntsc2pal {
			dir = "PAL"
			isNTSC = false
		}
		appendLog(fmt.Sprintf("Full-disc extraction: converting all VTS sets to %s (DVD-Video MPEG-2)", dir))
	} else {
		appendLog("Full-disc extraction: extracting all VTS sets (no region conversion)")
	}

	// Create output VIDEO_TS directory
	videoTSOut := filepath.Join(outputDir, "VIDEO_TS")
	if err := os.MkdirAll(videoTSOut, 0755); err != nil {
		return fmt.Errorf("create output VIDEO_TS dir: %w", err)
	}

	// Build per-VTS ffmpeg filter chain
	var vfFilter, afFilter string
	if pal2ntsc {
		vfFilter = "yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001"
		afFilter = "atempo=0.9600"
		appendLog("Applying PAL→NTSC conversion: yadif deinterlace + scale 720x480 + 29.97 fps + pitch correction")
	} else if ntsc2pal {
		vfFilter = "yadif=mode=1,scale=720:576:flags=lanczos,fps=25"
		afFilter = "atempo=1.0417"
		appendLog("Applying NTSC→PAL conversion: yadif deinterlace + scale 720x576 + 25 fps + pitch correction")
	}

	var convertedSets []convertedVTS

	totalSteps := len(sets)
	if menuSet != nil {
		totalSteps++
	}
	step := 0

	// Process each VTS set
	for _, set := range sets {
		step++
		appendLog(fmt.Sprintf("[%d/%d] Processing %s (%d files, %d MB)...",
			step, totalSteps, set.Name, len(set.Files), set.Size/(1024*1024)))

		vtsOut := filepath.Join(videoTSOut, set.Name+".VOB")

		// Build concat list
		listFile, err := BuildConcatList(set.Files)
		if err != nil {
			return fmt.Errorf("build concat list for %s: %w", set.Name, err)
		}

		// Probe input duration for per-VTS progress tracking
		inputDuration := probeDurationConcat(listFile, opts.OnRunCommand, appendLog)

		// Map this VTS's progress to its slice of the 0-80% overall range
		startPct := float64(step-1) / float64(totalSteps) * 80
		endPct := float64(step) / float64(totalSteps) * 80
		subProgress := func(pct float64) {
			updateProgress(startPct + pct/100*(endPct-startPct))
		}

		if err := convertVOBWithRegion(ctx, opts, listFile, vtsOut, set.Name, vfFilter, afFilter, isEncrypted, inputDuration, appendLog, subProgress); err != nil {
			os.Remove(listFile)
			return err
		}
		os.Remove(listFile)

		// Probe converted VOB duration
		duration := probeDuration(vtsOut, opts.OnRunCommand, appendLog)
		vobInfo, _ := os.Stat(vtsOut)

		// Read chapter info from original IFO
		var chapterSec []float64
		var titleDuration float64
		vtsIFO := filepath.Join(videoTSPath, set.Name+"_0.IFO")
		if titleInfo, err := ifo.ReadTitleInfo(vtsIFO); err == nil && len(titleInfo.Chapters) > 1 {
			chapterSec = titleInfo.Chapters
			titleDuration = titleInfo.Duration
		}

		// Scale chapter timestamps for region conversion
		if hasConversion && duration > 0 && titleDuration > 0 {
			factor := duration / titleDuration
			for i := range chapterSec {
				chapterSec[i] *= factor
			}
		}

		convertedSets = append(convertedSets, convertedVTS{
			Name:       set.Name,
			VOBPath:    vtsOut,
			VOBSize:    vobInfo.Size(),
			Duration:   duration,
			ChapterSec: chapterSec,
		})
		updateProgress(float64(step) / float64(totalSteps) * 80)
	}

	// Process menu VOB
	if menuSet != nil {
		step++
		appendLog(fmt.Sprintf("[%d/%d] Processing menu VIDEO_TS.VOB (%d MB)...",
			step, totalSteps, menuSet.Size/(1024*1024)))

		menuOut := filepath.Join(videoTSOut, "VIDEO_TS.VOB")
		listFile, err := BuildConcatList(menuSet.Files)
		if err != nil {
			return fmt.Errorf("build concat list for menu: %w", err)
		}

		menuInputDuration := probeDurationConcat(listFile, opts.OnRunCommand, appendLog)
		startPct := float64(step-1) / float64(totalSteps) * 80
		endPct := float64(step) / float64(totalSteps) * 80
		menuSubProgress := func(pct float64) {
			updateProgress(startPct + pct/100*(endPct-startPct))
		}

		if err := convertVOBWithRegion(ctx, opts, listFile, menuOut, "VIDEO_TS", vfFilter, afFilter, isEncrypted, menuInputDuration, appendLog, menuSubProgress); err != nil {
			os.Remove(listFile)
			return err
		}
		os.Remove(listFile)

		vobInfo, _ := os.Stat(menuOut)
		convertedSets = append(convertedSets, convertedVTS{
			Name:    "VIDEO_TS",
			VOBPath: menuOut,
			VOBSize: vobInfo.Size(),
			IsMenu:  true,
		})
		updateProgress(float64(step) / float64(totalSteps) * 80)
	}

	appendLog("VOB extraction and conversion complete. Regenerating IFO/BUP files...")
	updateProgress(85)

	// Stage 3: IFO regeneration
	if err := RegenerateIFOs(videoTSPath, videoTSOut, convertedSets, isNTSC, opts.RegionConvert, appendLog); err != nil {
		appendLog(fmt.Sprintf("IFO regeneration error: %v", err))
		return fmt.Errorf("IFO regeneration: %w", err)
	}

	updateProgress(100)
	appendLog(fmt.Sprintf("Full-disc extraction complete. Output: %s", videoTSOut))
	return nil
}

// convertVOBWithRegion runs ffmpeg to convert a VOB concat list with optional region conversion.
// duration is the input duration in seconds (used for progress tracking; 0 = no progress).
func convertVOBWithRegion(ctx context.Context, opts ExecuteOptions, listFile, outputPath, setName, vfFilter, afFilter string, isEncrypted bool, duration float64, appendLog func(string), updateProgress func(float64)) error {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-fflags", "+genpts",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
	}

	// Map all video and all audio streams.
	// Subtitles are dropped on region conversion: VOBSUB PTS values are baked at the
	// source frame rate and cannot be cleanly remapped by a simple stream copy.
	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "0:a?")
	if vfFilter == "" {
		// No region conversion: include subtitles as-is (PTS remain valid).
		args = append(args, "-map", "0:s?")
	}
	args = append(args, "-map_metadata", "-1")

	if vfFilter != "" {
		args = append(args, "-vf", vfFilter)
	}
	if afFilter != "" {
		args = append(args, "-af", afFilter)
	}

	// DVD-compliant MPEG-2 video + AC-3 audio
	args = append(args,
		"-c:v", "mpeg2video",
		"-q:v", "5",
		"-c:a", "ac3",
		"-b:a", "192k",
	)
	if vfFilter == "" {
		args = append(args, "-c:s", "copy")
	}
	args = append(args, "-max_interleave_delta", "0", "-progress", "pipe:1", "-nostats", outputPath)

	appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))

	statusFn := func(msg string) {
		if opts.OnSetStatus != nil {
			if msg != "" {
				opts.OnSetStatus(fmt.Sprintf("[%s] %s", setName, msg))
			} else {
				opts.OnSetStatus("")
			}
		}
	}
	if err := runFFmpegWithProgress(ctx, utils.GetFFmpegPath(), args, duration, updateProgress, appendLog, statusFn); err != nil {
		return fmt.Errorf("%s conversion failed: %w", setName, err)
	}
	return nil
}

// probeDurationConcat returns duration from a concat list file.
func probeDurationConcat(listFile string, onRunCommand func(string, []string, func(string)) error, appendLog func(string)) float64 {
	args := []string{
		"-v", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
	}
	var durationStr string
	logFn := func(line string) {
		if durationStr == "" {
			durationStr = strings.TrimSpace(line)
		}
	}
	if err := onRunCommand(utils.GetFFprobePath(), args, logFn); err != nil {
		return 0
	}
	var d float64
	if _, err := fmt.Sscanf(durationStr, "%f", &d); err != nil {
		return 0
	}
	return d
}

// probeSubtitleCount returns the number of subtitle streams the concat input
// exposes (the first VOB's stream set governs the concat demuxer's view).
// Returns -1 when the probe fails so callers can skip clamping.
func probeSubtitleCount(listFile string, onRunCommand func(string, []string, func(string)) error, appendLog func(string)) int {
	args := []string{
		"-v", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-select_streams", "s",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
	}
	count := 0
	logFn := func(line string) {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	if err := onRunCommand(utils.GetFFprobePath(), args, logFn); err != nil {
		appendLog(fmt.Sprintf("Warning: could not probe subtitle streams for VOB concat fallback: %v", err))
		return -1
	}
	return count
}

// probeDuration returns the duration in seconds of a VOB file by quick ffprobe.
func probeDuration(vobPath string, onRunCommand func(string, []string, func(string)) error, appendLog func(string)) float64 {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		vobPath,
	}
	var durationStr string
	logFn := func(line string) {
		if durationStr == "" {
			durationStr = strings.TrimSpace(line)
		}
	}
	if err := onRunCommand(utils.GetFFprobePath(), args, logFn); err != nil {
		appendLog(fmt.Sprintf("Warning: could not probe duration for %s: %v", filepath.Base(vobPath), err))
		return 0
	}
	var d float64
	if _, err := fmt.Sscanf(durationStr, "%f", &d); err != nil {
		return 0
	}
	return d
}

// runFFmpegWithProgress runs ffmpeg with -progress pipe:1 output and parses
// out_time_us to call progressCallback with a percentage and statusCallback with
// a compact ETA string (e.g. "42% — ETA 2m 34s").
func runFFmpegWithProgress(ctx context.Context, ffmpegPath string, args []string, totalDur float64, progressCallback func(float64), logFn func(string), statusCallback func(string)) error {
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	utils.ApplyNoWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	startTime := time.Now()
	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("ffmpeg start: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		var lastPct float64
		var lastStatusUpdate time.Time
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			if parts[0] == "out_time_us" && totalDur > 0 {
				if micros, err := strconv.ParseFloat(parts[1], 64); err == nil {
					currentSec := micros / 1000000.0
					pct := (currentSec / totalDur) * 100
					if pct > 100 {
						pct = 100
					}
					if pct-lastPct >= 0.5 {
						lastPct = pct
						if progressCallback != nil {
							progressCallback(pct)
						}
						remainingSec := totalDur - currentSec
						if remainingSec > 0 && statusCallback != nil && time.Since(lastStatusUpdate) > 2*time.Second {
							lastStatusUpdate = time.Now()
							elapsed := time.Since(startTime).Seconds()
							rate := 1.0
							if elapsed > 1 {
								rate = currentSec / elapsed
							}
							etaSec := remainingSec / rate
							eta := time.Duration(etaSec) * time.Second
							statusCallback(formatETA(int(pct), eta))
						}
					}
				}
			}
		}
	}()

	err = cmd.Wait()
	if progressCallback != nil {
		progressCallback(100)
	}
	if statusCallback != nil {
		statusCallback("")
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" && logFn != nil {
			logFn(stderrStr)
		}
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}

// formatETA returns a compact string like "ETA 2m 34s" or "ETA < 1s".
func formatETA(pct int, eta time.Duration) string {
	if eta <= 0 {
		return fmt.Sprintf("%d%% — ETA < 1s", pct)
	}
	eta = eta.Round(time.Second)
	m := int(eta.Minutes())
	s := int(eta.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%d%% — ETA %dm %ds", pct, m, s)
	}
	return fmt.Sprintf("%d%% — ETA %ds", pct, s)
}
