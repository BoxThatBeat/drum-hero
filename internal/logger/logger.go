package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var fileLogger *log.Logger

// Init sets up file logging to ~/.cache/drum-hero/drum-hero.log.
// Call this once at startup.
func Init() {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	logDir := filepath.Join(dir, "drum-hero")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return
	}

	logPath := filepath.Join(logDir, "drum-hero.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}

	fileLogger = log.New(f, "", log.Ltime|log.Lmicroseconds)
	fileLogger.Printf("=== drum-hero started ===")
	fmt.Fprintf(os.Stderr, "Logging to %s\n", logPath)
}

// Log writes a formatted message to the log file.
func Log(format string, args ...any) {
	if fileLogger != nil {
		fileLogger.Printf(format, args...)
	}
}
