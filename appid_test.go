package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The window's program name must equal the .desktop file's name, or
// desktops on Wayland show a generic icon instead of Kisetsu's.
func TestWindowNameMatchesDesktopFile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("build", "linux", appID+".desktop"))
	if err != nil {
		t.Fatalf("no .desktop file named after appID: %v", err)
	}
	if !strings.Contains(string(data), "\nStartupWMClass="+appID+"\n") {
		t.Errorf("StartupWMClass is not %s", appID)
	}
	if !strings.Contains(string(data), "\nIcon="+appID+"\n") {
		t.Errorf("Icon is not %s", appID)
	}
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "\nAPP_ID := "+appID+"\n") {
		t.Errorf("APP_ID in the Makefile is not %s", appID)
	}
}

// The Windows file version (wails.json) must follow VERSION in the
// Makefile; the release steps change both.
func TestWindowsVersionMatchesMakefile(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	var version string
	for line := range strings.SplitSeq(string(makefile), "\n") {
		if v, ok := strings.CutPrefix(line, "VERSION := "); ok {
			version = strings.TrimSpace(v)
		}
	}
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if version == "" || cfg.Info.ProductVersion != version {
		t.Errorf("wails.json productVersion = %q, Makefile VERSION = %q", cfg.Info.ProductVersion, version)
	}
}
