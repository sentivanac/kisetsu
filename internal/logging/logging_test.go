package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
		{" warn ", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.in)
		if err != nil {
			t.Errorf("ParseLevel(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseLevelRejectsUnknown(t *testing.T) {
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("expected an error for an unknown level")
	}
}

func TestNewFiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelWarn)

	log.Info("hidden")
	log.Warn("shown", "key", "value")

	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Errorf("info record was logged at warn level: %q", out)
	}
	if !strings.Contains(out, "msg=shown") || !strings.Contains(out, "key=value") {
		t.Errorf("warn record missing: %q", out)
	}
}

func TestOpenAppendsToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("existing line\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	log, closer, err := Open(dir, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("new record")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "existing line\n") {
		t.Errorf("existing content was not preserved: %q", content)
	}
	if !strings.Contains(string(content), "msg=\"new record\"") {
		t.Errorf("new record missing: %q", content)
	}
}
