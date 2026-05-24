package logging

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	file       *os.File
	history    []string
	logger     = log.New(os.Stderr, "[videotools] ", log.LstdFlags|log.Lmicroseconds)
	filePath   string
	historyMax = 500
	debugOn        = false
	verboseDiscOn  = false
	logsDir        string

	// fileMu serialises concurrent writes to file so goroutines never interleave
	// partial log lines, and lets Error/Fatal call Sync() without a race.
	fileMu sync.Mutex

	suppressMu       sync.Mutex
	suppressCount    = make(map[string]int)
	suppressLastMsg  = make(map[string]time.Time)
	suppressThrottle = 5 * time.Second

	// sessionVersion is set by SetVersion (called from main before Init) and
	// written into every session / clear header so the log is self-describing.
	sessionVersion string

	// playerTraceMu guards traceFile and traceEnabled.
	playerTraceMu      sync.Mutex
	traceFile          *os.File
	playerTraceEnabled = false
)

const (
	CatUI      Category = "[UI]"
	CatCLI     Category = "[CLI]"
	CatFFMPEG  Category = "[FFMPEG]"
	CatSystem  Category = "[SYS]"
	CatModule  Category = "[MODULE]"
	CatPlayer  Category = "[PLAYER]"
	CatDVD     Category = "[DVD]"
	CatDisc    Category = "[DISC]"
	CatConvert Category = "[CONVERT]"
	CatTrim    Category = "[TRIM]"
	CatMerge   Category = "[MERGE]"
	CatFilters Category = "[FILTERS]"
	CatAudio   Category = "[AUDIO]"
	CatAuthor  Category = "[AUTHOR]"
	CatBurn    Category = "[BURN]"
	CatInspect Category = "[INSPECT]"
	CatQueue   Category = "[QUEUE]"
)

// Categories represents a log category
type Category string

const sessionMarker = "=== VideoTools session started"

// SetVersion stores the application version string (e.g. "v0.1.1-dev50-abc1234")
// so that Init and Clear can write it into the session header.
// Must be called before Init for the startup header to include the version.
func SetVersion(v string) {
	fileMu.Lock()
	sessionVersion = v
	fileMu.Unlock()
}

// sessionHeader returns the single-line session marker written at the start of
// every session and after a clear.  Includes the version when available.
func sessionHeader() string {
	ts := time.Now().Format(time.RFC3339)
	if sessionVersion != "" {
		return fmt.Sprintf("%s at %s — version %s\n", sessionMarker, ts, sessionVersion)
	}
	return fmt.Sprintf("%s at %s\n", sessionMarker, ts)
}

// rotateLog keeps at most 2 sessions in the log file.
// It reads the existing content, finds session markers, and trims everything
// before the last session boundary so only the most recent previous session
// plus the new one accumulate.
func rotateLog(path string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}

	marker := []byte("\n" + sessionMarker)
	// Find all marker positions.
	var positions []int
	search := data
	offset := 0
	for {
		idx := bytes.Index(search, marker)
		if idx < 0 {
			break
		}
		positions = append(positions, offset+idx)
		search = search[idx+1:]
		offset += idx + 1
	}

	// Fewer than 2 existing sessions — nothing to trim.
	if len(positions) < 2 {
		return
	}

	// Keep from the second-to-last session start (preserves last complete
	// session + whatever was written after it).
	keepFrom := positions[len(positions)-1] // start of most recent session
	trimmed := data[keepFrom:]

	if err := os.WriteFile(path, trimmed, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "videotools: log rotate failed: %v\n", err)
	}
}

// Init initializes logging system with organized log folders
func Init() {
	// Use environment variable or configured default
	filePath = os.Getenv("VIDEOTOOLS_LOG_FILE")
	if filePath == "" {
		filePath = logFilePath("videotools.log")
	} else {
		logsDir = filepath.Dir(filePath)
	}

	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(LogsDir(), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot create logs directory: %v\n", err)
		return
	}

	// Trim old sessions before opening for append.
	rotateLog(filePath)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot open log file %s: %v\n", filePath, err)
		return
	}
	file = f

	// Write session boundary so rotateLog can find it next run.
	fmt.Fprintf(file, "\n%s", sessionHeader())

	// Enable per-frame playback trace when the env var is set.
	if os.Getenv("VIDEOTOOLS_PLAYER_TRACE") != "" {
		SetPlayerTrace(true)
	}

	// Flush the OS write cache every 500 ms so log entries appear on disk
	// promptly even when the process hangs between Sync() calls.
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for range t.C {
			fileMu.Lock()
			if file != nil {
				file.Sync()
			}
			fileMu.Unlock()
		}
	}()
}

// GetCrashLogPath returns path for crash-specific log file
func GetCrashLogPath() string {
	return logFilePath("crashes.log")
}

// GetConversionLogPath returns path for conversion-specific log file
func GetConversionLogPath() string {
	return logFilePath("conversion.log")
}

// GetPlayerLogPath returns path for player-specific log file
func GetPlayerLogPath() string {
	return logFilePath("player.log")
}

// getStackTrace returns current goroutine stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// RecoverPanic logs a recovered panic with a stack trace.
// Intended for use in deferred calls inside goroutines.
func RecoverPanic() {
	if r := recover(); r != nil {
		Crash(CatSystem, "Recovered panic: %v", r)
	}
}

// RecoverPanicWithCallback calls RecoverPanic and also invokes a callback if provided.
func RecoverPanicWithCallback(callback func()) {
	if r := recover(); r != nil {
		Crash(CatSystem, "Recovered panic: %v", r)
		if callback != nil {
			callback()
		}
	}
}

// RecoverPanicAndReturn returns the recovered value if any.
func RecoverPanicAndReturn() (err interface{}) {
	defer func() {
		if r := recover(); r != nil {
			err = r
			Crash(CatSystem, "Recovered panic: %v", r)
		}
	}()
	return nil
}

// FullStackTrace returns a full goroutine dump including all goroutines.
func FullStackTrace() string {
	buf := make([]byte, 65536)
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

// LogAllGoroutines dumps all goroutine stacks to the crash log.
func LogAllGoroutines() {
	Crash(CatSystem, "=== Goroutine Dump ===")
	Crash(CatSystem, "%s", FullStackTrace())
}

// writeLog writes a pre-formatted log line to the log file under fileMu.
// It must NOT be called with fileMu already held (no re-entrancy).
func writeLog(line string) {
	fileMu.Lock()
	if file != nil {
		fmt.Fprintln(file, line)
	}
	fileMu.Unlock()
}

// Error logs an error message with a category (always logged, even when debug is off)
func Error(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s ERROR: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	line := timestamp + " " + msg
	writeLog(line)
	fileMu.Lock()
	if file != nil {
		file.Sync()
	}
	fileMu.Unlock()
	history = append(history, line)
	if len(history) > historyMax {
		history = history[len(history)-historyMax:]
	}
	logger.Printf("%s %s", timestamp, msg)
}

// Warning logs a warning message with a category
func Warning(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s WARNING: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	writeLog(timestamp + " " + msg)
	logger.Printf("%s %s", timestamp, msg)
}

// Debug logs a debug message with a category
func Debug(cat Category, format string, args ...interface{}) {
	if !debugOn {
		return
	}
	msg := fmt.Sprintf("%s %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	writeLog(timestamp + " " + msg)
	logger.Printf("%s %s", timestamp, msg)
}

// shouldSuppress returns true if the message should be suppressed due to repetition.
// Thread-safe: all access to the suppress maps is protected by suppressMu.
func shouldSuppress(msg string) bool {
	now := time.Now()
	suppressMu.Lock()
	defer suppressMu.Unlock()

	lastMsg, exists := suppressLastMsg[msg]
	if exists && now.Sub(lastMsg) < suppressThrottle {
		suppressCount[msg]++
		return true
	}

	suppressCount[msg] = 0
	suppressLastMsg[msg] = now
	return false
}

// Info logs an informational message with automatic suppression for repeated messages
func Info(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s INFO: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)

	if shouldSuppress(msg) {
		return
	}

	writeLog(timestamp + " " + msg)
	logger.Printf("%s %s", timestamp, msg)
}

// Crash logs a critical error with stack trace for debugging crashes
func Crash(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s CRASH: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)

	line := timestamp + " " + msg
	writeLog(line)
	fileMu.Lock()
	if file != nil {
		file.Sync()
	}
	fileMu.Unlock()
	logger.Printf("%s %s", timestamp, msg)

	// Also log to dedicated crash log
	if crashFile, err := os.OpenFile(GetCrashLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(crashFile, "%s %s\n", timestamp, msg)
		fmt.Fprintf(crashFile, "Stack trace:\n%s\n", getStackTrace())
		crashFile.Sync()
	}
}

// Fatal logs a fatal error and exits (always logged, even when debug is off)
func Fatal(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s FATAL: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	writeLog(timestamp + " " + msg)
	fileMu.Lock()
	if file != nil {
		file.Sync()
	}
	fileMu.Unlock()
	logger.Printf("%s %s", timestamp, msg)
	os.Exit(1)
}

// Sync flushes the log file to disk immediately. Call this at startup checkpoints
// before code that could crash at the C/OS level — ensures log entries survive.
func Sync() {
	fileMu.Lock()
	if file != nil {
		file.Sync()
	}
	fileMu.Unlock()
}

// Close closes log file
func Close() {
	if file != nil {
		file.Close()
	}
}

// Reopen re-initializes logging output with the current configuration.
func Reopen() {
	if file != nil {
		_ = file.Close()
		file = nil
	}
	Init()
}

// Clear truncates the active log file and writes a fresh session header.
// Safe to call while the app is running — holds fileMu for the operation.
//
// On Windows, Truncate(0) on a file opened O_APPEND fails with "Access is
// denied".  The workaround is to close the handle and reopen with O_TRUNC,
// which Windows allows unconditionally.
func Clear() error {
	fileMu.Lock()
	defer fileMu.Unlock()

	if filePath == "" {
		return fmt.Errorf("log file not open")
	}

	// Flush and close the current handle before truncating.
	if file != nil {
		_ = file.Sync()
		_ = file.Close()
		file = nil
	}

	// Reopen with O_TRUNC — this works on Windows where Truncate(0) on an
	// O_APPEND handle is refused.
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("clear log: reopen: %w", err)
	}
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "=== Log cleared at %s ===\n", ts)
	fmt.Fprintf(f, "%s", sessionHeader())
	_ = f.Close()

	// Reopen in append mode so subsequent writes land after the header.
	file, err = os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("clear log: reopen append: %w", err)
	}
	return nil
}

// SetDebug enables or disables debug logging.
func SetDebug(enabled bool) {
	debugOn = enabled
}

// SetVerboseDisc enables or disables verbose disc-subsystem logging.
func SetVerboseDisc(enabled bool) {
	verboseDiscOn = enabled
}

// IsVerboseDisc reports whether verbose disc logging is active.
func IsVerboseDisc() bool {
	return verboseDiscOn
}

// FilePath returns the active log file path, if initialized.
func FilePath() string {
	return filePath
}

// SetLogsDir sets the base directory for log files.
func SetLogsDir(dir string) {
	logsDir = dir
}

// LogsDir returns the active log directory.
func LogsDir() string {
	if logsDir != "" {
		return logsDir
	}
	return "logs"
}

func logFilePath(name string) string {
	return filepath.Join(LogsDir(), name)
}

// GetPlayerTracePath returns the path for the per-frame playback trace log.
func GetPlayerTracePath() string {
	return logFilePath("player_trace.log")
}

// SetPlayerTrace enables or disables per-frame playback trace logging.
// When enabled, a CSV trace is written to player_trace.log for post-mortem
// frame-by-frame AV sync analysis. Enabling opens the trace file; disabling
// closes it.
func SetPlayerTrace(enabled bool) {
	playerTraceMu.Lock()
	defer playerTraceMu.Unlock()
	if enabled == playerTraceEnabled {
		return
	}
	playerTraceEnabled = enabled
	if enabled {
		f, err := os.OpenFile(GetPlayerTracePath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "videotools: cannot open player trace: %v\n", err)
			playerTraceEnabled = false
			return
		}
		traceFile = f
		fmt.Fprintln(traceFile, "timestamp,frame,pts,clock,action,behind_ms")
	} else {
		if traceFile != nil {
			traceFile.Sync()
			traceFile.Close()
			traceFile = nil
		}
	}
}

// PlayerTraceEnabled reports whether per-frame trace logging is active.
func PlayerTraceEnabled() bool {
	playerTraceMu.Lock()
	defer playerTraceMu.Unlock()
	return playerTraceEnabled
}

// PlayerFrameTrace records one frame decision to the trace file.
// action is one of: "display", "drop", "stall", "snap".
// behindMs is clock-pts in milliseconds (positive = clock ahead of pts).
func PlayerFrameTrace(frameNum int64, pts, clock float64, action string, behindMs float64) {
	playerTraceMu.Lock()
	defer playerTraceMu.Unlock()
	if !playerTraceEnabled || traceFile == nil {
		return
	}
	fmt.Fprintf(traceFile, "%s,%d,%.3f,%.3f,%s,%.0f\n",
		time.Now().Format(time.RFC3339Nano), frameNum, pts, clock, action, behindMs)
}
