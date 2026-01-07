package logging

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

var (
	file       *os.File
	history    []string
	logger     = log.New(os.Stderr, "[videotools] ", log.LstdFlags|log.Lmicroseconds)
	filePath   string
	historyMax = 500
)

const (
	CatUI      Category = "[UI]"
	CatCLI     Category = "[CLI]"
	CatFFMPEG  Category = "[FFMPEG]"
	CatSystem  Category = "[SYS]"
	CatModule  Category = "[MODULE]"
	CatPlayer  Category = "[PLAYER]"
	CatEnhance Category = "[ENHANCE]"
)

// Categories represents a log category
type Category string

// Init initializes logging system with organized log folders
func Init() {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot create logs directory: %v\n", err)
		return
	}

	// Use environment variable or default
	filePath = os.Getenv("VIDEOTOOLS_LOG_FILE")
	if filePath == "" {
		filePath = "logs/videotools.log"
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "videotools: cannot open log file %s: %v\n", filePath, err)
		return
	}
	file = f
}

// GetCrashLogPath returns path for crash-specific log file
func GetCrashLogPath() string {
	return "logs/crashes.log"
}

// GetConversionLogPath returns path for conversion-specific log file
func GetConversionLogPath() string {
	return "logs/conversion.log"
}

// GetPlayerLogPath returns path for player-specific log file
func GetPlayerLogPath() string {
	return "logs/player.log"
}

// getStackTrace returns current goroutine stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
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

// Debug logs a debug message with a category
func Debug(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
	}
	logger.Printf("%s %s", timestamp, msg)
}

// Info logs an informational message
func Info(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s INFO: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
	}
	logger.Printf("%s %s", timestamp, msg)
}

// Crash logs a critical error with stack trace for debugging crashes
func Crash(cat Category, format string, args ...interface{}) {
	msg := fmt.Sprintf("%s CRASH: %s", cat, fmt.Sprintf(format, args...))
	timestamp := time.Now().Format(time.RFC3339Nano)

	// Log to main log file
	if file != nil {
		fmt.Fprintf(file, "%s %s\n", timestamp, msg)
	}
	logger.Printf("%s %s", timestamp, msg)

	// Also log to dedicated crash log
	if crashFile, err := os.OpenFile(GetCrashLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(crashFile, "%s %s\n", timestamp, msg)
		fmt.Fprintf(crashFile, "Stack trace:\n%s\n", timestamp, getStackTrace())
		crashFile.Sync()
	}
}

// Fatal logs a fatal error and exits (always logged, even when debug is off)
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

// Close closes log file
func Close() {
	if file != nil {
		file.Close()
	}
}
