package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTrayIcon(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tray-icons")
	for range 2 { // writing again is harmless
		if err := writeTrayIcon(dir); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, appID+".png"))
	if err != nil || !bytes.Equal(data, trayIcon) {
		t.Errorf("icon file: %d bytes, %v", len(data), err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("%d files in the icon directory, want 1 (no leftovers)", len(entries))
	}
}
