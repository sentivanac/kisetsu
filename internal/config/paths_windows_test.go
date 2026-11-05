//go:build windows

package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsUseLocalAppData(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	got, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(local, "Kisetsu", "data"); got.Data != want {
		t.Errorf("Data = %q, want %q", got.Data, want)
	}
	if err := got.Ensure(); err != nil {
		t.Errorf("Ensure: %v", err)
	}
}
