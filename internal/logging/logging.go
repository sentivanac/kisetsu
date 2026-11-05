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

// MaxSize is the size above which the log is started anew when it is
// opened. The previous log is kept as FileName + ".1", so the logs take
// at most about twice this much space. Every rotation run opens the log,
// so the check happens often enough.
const MaxSize = 5 << 20

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

// Open creates a logger that appends to FileName in dir and, if mirror is
// not nil, writes the same records to mirror (the window mirrors to
// stderr; commands do not, so their terminal output stays readable). The
// caller must close the returned file.
func Open(dir string, level slog.Level, mirror io.Writer) (*slog.Logger, io.Closer, error) {
	path := filepath.Join(dir, FileName)
	if info, err := os.Stat(path); err == nil && info.Size() > MaxSize {
		// Replaces an older ".1". On Linux, a process that still has the
		// log open keeps writing into the renamed file until it is
		// restarted; nothing is lost. Windows refuses to rename a file
		// another process has open (the window, the agent); then this
		// process keeps appending to the same file, and a later start
		// rotates it. A full log must never stop Kisetsu from starting.
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	var w io.Writer = f
	if mirror != nil {
		w = io.MultiWriter(f, mirror)
	}
	return New(w, level), f, nil
}
