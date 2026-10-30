// Package logging sets up the application logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the log file created in the state directory.
const FileName = "kisetsu.log"

// LevelEnv is the environment variable that selects the log level.
const LevelEnv = "KISETSU_LOG_LEVEL"

// ParseLevel converts debug, info, warn or error (case-insensitive)
// into a slog.Level. An empty string means info.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("unknown log level %q (use debug, info, warn or error)", s)
}

// New returns a logger that writes text records to w.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

// Open creates a logger that appends to FileName in dir and mirrors
// records to stderr. The caller must close the returned file.
func Open(dir string, level slog.Level) (*slog.Logger, io.Closer, error) {
	path := filepath.Join(dir, FileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	return New(io.MultiWriter(f, os.Stderr), level), f, nil
}
