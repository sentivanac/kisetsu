package config

import (
	"path/filepath"
	"testing"
)

func TestWindowsPaths(t *testing.T) {
	local := filepath.Join(t.TempDir(), "AppData", "Local")
	got := WindowsPaths(local)
	base := filepath.Join(local, "Kisetsu")
	want := Paths{
		Config: filepath.Join(base, "config"),
		Data:   filepath.Join(base, "data"),
		Cache:  filepath.Join(base, "cache"),
		State:  filepath.Join(base, "state"),
	}
	if got != want {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}
