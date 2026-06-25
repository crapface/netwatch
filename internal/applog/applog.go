// Package applog is a tiny thread-safe, leveled file logger.
// It writes a plain-text app.log next to the executable (portable: no %APPDATA%).
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	f    *os.File
	path string
	sink func(level, line string) // optional live sink for the GUI status/log view
)

// Init opens (or creates) app.log inside dir. Safe to call once at startup.
func Init(dir string) error {
	mu.Lock()
	defer mu.Unlock()
	path = filepath.Join(dir, "app.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	f = file
	return nil
}

// SetSink registers a callback that receives every formatted log line
// (used by the GUI to mirror recent activity). Pass nil to detach.
func SetSink(s func(level, line string)) {
	mu.Lock()
	sink = s
	mu.Unlock()
}

// Path returns the absolute path of the active log file.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return path
}

func write(level, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	line := fmt.Sprintf("%s [%-5s] %s", time.Now().Format("2006-01-02 15:04:05"), level, msg)
	mu.Lock()
	if f != nil {
		fmt.Fprintln(f, line)
		_ = f.Sync()
	}
	s := sink
	mu.Unlock()
	if s != nil {
		s(level, line)
	}
}

// Info logs an informational message.
func Info(format string, a ...interface{}) { write("INFO", format, a...) }

// Warn logs a warning.
func Warn(format string, a ...interface{}) { write("WARN", format, a...) }

// Error logs an error.
func Error(format string, a ...interface{}) { write("ERROR", format, a...) }

// Close flushes and closes the log file.
func Close() {
	mu.Lock()
	if f != nil {
		_ = f.Sync()
		_ = f.Close()
		f = nil
	}
	mu.Unlock()
}
