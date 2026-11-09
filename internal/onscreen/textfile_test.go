package onscreen

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func writeText(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func problemOf(t *testing.T, err error) string {
	t.Helper()
	var p *textFileProblem
	if !errors.As(err, &p) {
		t.Fatalf("err = %v, want a problem shown on the wallpaper", err)
	}
	return p.msg
}

func TestReadTextFile(t *testing.T) {
	path := writeText(t, "\n\nFilesystem  Size\n/dev/sda1   100G\t(root)\r\n\n", 0o644)
	got, err := readTextFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Filesystem  Size", "/dev/sda1   100G    (root)"}
	if !slices.Equal(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

func TestReadTextFileThroughASymlink(t *testing.T) {
	path := writeText(t, "hello", 0o600)
	link := filepath.Join(t.TempDir(), "link.txt")
	if err := os.Symlink(path, link); err != nil {
		t.Skip("no symlinks:", err)
	}
	if got, err := readTextFile(link); err != nil || !slices.Equal(got, []string{"hello"}) {
		t.Errorf("%q, %v", got, err)
	}
}

func TestReadTextFileRefuses(t *testing.T) {
	dir := t.TempDir()
	if msg := problemOf(t, func() error { _, err := readTextFile(""); return err }()); msg != "No text file chosen" {
		t.Errorf("no path: %q", msg)
	}
	if msg := problemOf(t, func() error { _, err := readTextFile(filepath.Join(dir, "gone.txt")); return err }()); msg != "Text file not found" {
		t.Errorf("missing: %q", msg)
	}
	if msg := problemOf(t, func() error { _, err := readTextFile(dir); return err }()); msg != "Not a text file" {
		t.Errorf("directory: %q", msg)
	}
	binary := writeText(t, "\xff\xfe\x00garbage", 0o600)
	if msg := problemOf(t, func() error { _, err := readTextFile(binary); return err }()); msg != "The text file is not UTF-8 text" {
		t.Errorf("binary: %q", msg)
	}
}

func TestReadTextFileRefusesFilesOthersCanWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses access lists")
	}
	path := writeText(t, "hello", 0o666)
	if msg := problemOf(t, func() error { _, err := readTextFile(path); return err }()); msg != "Others can change the text file" {
		t.Errorf("writable by others: %q", msg)
	}
	path = writeText(t, "hello", 0o620)
	if msg := problemOf(t, func() error { _, err := readTextFile(path); return err }()); msg != "Others can change the text file" {
		t.Errorf("writable by the group: %q", msg)
	}
}

func TestReadTextFileLimits(t *testing.T) {
	// More than 10 lines: the first 10.
	var many []string
	for i := range 15 {
		many = append(many, strings.Repeat("x", i+1))
	}
	got, err := readTextFile(writeText(t, strings.Join(many, "\n"), 0o600))
	if err != nil || len(got) != maxTextLines || got[0] != "x" {
		t.Errorf("15 lines: %d lines, %v", len(got), err)
	}

	// A huge file: only its start, ending at a whole line.
	line := strings.Repeat("é", 30) + "\n" // 61 bytes
	got, err = readTextFile(writeText(t, strings.Repeat(line, 1000), 0o600))
	if err != nil || len(got) != maxTextLines || got[maxTextLines-1] != strings.Repeat("é", 30) {
		t.Errorf("huge file: %q, %v", got, err)
	}
}

func TestClean(t *testing.T) {
	tests := map[string]string{
		"\x1b[1;31mred\x1b[0m text":    "red text",
		"bell\a and back\bspace":       "bell and backspace",
		"user\u202egnp.exe":            "usergnp.exe",
		"\ufeffwith BOM\u200f":         "with BOM",
		"tabs\there":                   "tabs    here",
		"Ćirilica: Ђорђе, Ελληνικά":    "Ćirilica: Ђорђе, Ελληνικά",
		"trailing ESC \x1b":            "trailing ESC ",
		"cursor \x1b7saved\x1b8 moves": "cursor saved moves",
	}
	for in, want := range tests {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}
