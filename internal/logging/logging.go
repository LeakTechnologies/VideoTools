package logging

import (
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
	debugOn    = false
	logsDir    string

	// fileMu serialises concurrent writes to file so goroutines never interleave
	// partial log lines, and lets Error/Fatal call Sync() without a race.
	fileMu sync.Mutex

	suppressMu       sync.Mutex
	suppressCount    = make(map[string]int)
	suppressLastMsg  = make(map[string]time.Time)
	suppressThrottle = 5 * time.Second

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

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot open log file %s: %v\n", filePath, err)
		return
	}
	file = f

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

// SetDebug enables or disables debug logging.
func SetDebug(enabled bool) {
	debugOn = enabled
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
