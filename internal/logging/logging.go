package logging

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"
)

var (
	filePath     string
	file         *os.File
	history      []string
	debugEnabled bool
	logger       = log.New(os.Stderr, "[videotools] ", log.LstdFlags|log.Lmicroseconds)
)

const historyMax = 500

// Category represents a log category
type Category string

const (
	CatUI     Category = "[UI]"
	CatCLI    Category = "[CLI]"
	CatFFMPEG Category = "[FFMPEG]"
	CatSystem Category = "[SYS]"
	CatModule Category = "[MODULE]"
	CatPlayer Category = "[PLAYER]"
)

// Init initializes the logging system
func Init() {
	filePath = os.Getenv("VIDEOTOOLS_LOG_FILE")
	if filePath == "" {
		filePath = "videotools.log"
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot open log file %s: %v\n", filePath, err)
		return
	}
	file = f
}

// Close closes the log file
func Close() {
	if file != nil {
		file.Close()
	}
}

// SetDebug enables or disables debug logging
func SetDebug(on bool) {
	debugEnabled = on
	Debug(CatSystem, "debug logging toggled -> %v (VIDEOTOOLS_DEBUG=%s)", on, os.Getenv("VIDEOTOOLS_DEBUG"))
}

// Debug logs a debug message with a category
func Debug(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
	}
	history = append(history, fmt.Sprintf("%s %s", timestamp, msg))
	if len(history) > historyMax {
		history = history[len(history)-historyMax:]
	}
	if debugEnabled {
		logger.Printf("%s %s", timestamp, msg)
	}
}

// FilePath returns the current log file path
func FilePath() string {
	return filePath
}

// History returns the log history
func History() []string {
	return history
}

// Error logs an error message with a category (always logged, even when debug is off)
func Error(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s ERROR: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
	}
	history = append(history, fmt.Sprintf("%s %s", timestamp, msg))
	if len(history) > historyMax {
		history = history[len(history)-historyMax:]
	}
	logger.Printf("%s %s", timestamp, msg)
}

// Fatal logs a fatal error and exits (always logged)
func Fatal(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s FATAL: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
		file.Sync()
	}
	logger.Printf("%s %s", timestamp, msg)
	os.Exit(1)
}

// Panic logs a panic with stack trace
func Panic(recovered interface{}) {
	msg := fmt.Sprintf("%s PANIC: %v\nStack trace:\n%s", CatSystem, recovered, string(debug.Stack()))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
		file.Sync()
	}
	history = append(history, fmt.Sprintf("%s %s", timestamp, msg))
	logger.Printf("%s %s", timestamp, msg)
}

// RecoverPanic should be used with defer to catch and log panics
func RecoverPanic() {
	if r := recover(); r != nil {
		Panic(r)
		// Re-panic to let the program crash with the logged info
		panic(r)
	}
}
