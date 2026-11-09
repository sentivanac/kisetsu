package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/appinfo"
)

func TestGetAbout(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	about, err := app.GetAbout()
	if err != nil {
		t.Fatal(err)
	}
	if about.Name != appinfo.Name || about.Version != appinfo.Version || about.Author != "Miroslav Sentivanac" {
		t.Errorf("about = %+v", about)
	}
	if about.License != "MIT" || about.LicenseURL != "https://github.com/sentivanac/kisetsu/blob/main/LICENSE" {
		t.Errorf("license = %q, %q", about.License, about.LicenseURL)
	}
	if about.AuthorURL != "https://github.com/sentivanac" {
		t.Errorf("author URL = %q", about.AuthorURL)
	}
	if about.IssuesURL != "https://github.com/sentivanac/kisetsu/issues" {
		t.Errorf("issues = %q", about.IssuesURL)
	}
}

func TestTakeAboutRequest(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	if show, _ := app.TakeAboutRequest(); show {
		t.Error("About requested without --about")
	}
	app.aboutRequested.Store(true)
	if show, _ := app.TakeAboutRequest(); !show {
		t.Error("About not shown after --about")
	}
	if show, _ := app.TakeAboutRequest(); show {
		t.Error("About shown twice")
	}
}

func TestThirdPartyLicenses(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "kisetsu.exe")
	next := filepath.Join(dir, "THIRD_PARTY_LICENSES.txt")

	// Without the file next to the program, only the .deb location is
	// left, which exists only where the package is installed.
	if got := thirdPartyLicenses(exe); got != "" && got != "/usr/share/doc/kisetsu/THIRD_PARTY_LICENSES.gz" {
		t.Errorf("without the file: %q", got)
	}
	if err := os.WriteFile(next, []byte("licenses"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := thirdPartyLicenses(exe); got != next {
		t.Errorf("got %q, want %q", got, next)
	}
}

func TestReadThirdPartyLicenses(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "THIRD_PARTY_LICENSES.txt")
	if err := os.WriteFile(plain, []byte("MIT License"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := readThirdPartyLicenses(plain); err != nil || got != "MIT License" {
		t.Errorf("plain: %q, %v", got, err)
	}

	packed := filepath.Join(dir, "THIRD_PARTY_LICENSES.gz")
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	z.Write([]byte("BSD License"))
	z.Close()
	if err := os.WriteFile(packed, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := readThirdPartyLicenses(packed); err != nil || got != "BSD License" {
		t.Errorf("gzipped: %q, %v", got, err)
	}

	if _, err := readThirdPartyLicenses(""); err == nil {
		t.Error("no error without a file")
	}
}
