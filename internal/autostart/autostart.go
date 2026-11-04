// Package autostart starts the Kisetsu agent when the user logs in, with
// an XDG autostart entry (~/.config/autostart/*.desktop). GNOME, KDE
// Plasma, Xfce, MATE and Cinnamon read these entries; window managers such
// as i3 and sway do not.
//
// Kisetsu only changes or removes an entry it created itself (marked by
// a comment line), so a file the user wrote under the same name is never
// touched.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the entry's file name, after the application ID.
const FileName = "io.github.sentivanac.Kisetsu.Agent.desktop"

// marker identifies entries written by Kisetsu.
const marker = "# Created by Kisetsu."

var (
	// ErrForeign means an entry with Kisetsu's name exists that Kisetsu
	// did not create.
	ErrForeign = errors.New("autostart entry was not created by Kisetsu")
	// ErrTemporaryExecutable means Kisetsu runs from a temporary build
	// (for example `go run` or `wails dev`), which would be gone at the
	// next login.
	ErrTemporaryExecutable = errors.New("kisetsu runs from a temporary directory")
)

// Entry is the autostart entry for the running Kisetsu binary.
type Entry struct {
	Dir        string // ~/.config/autostart
	Executable string // absolute path of the kisetsu binary
	TempDir    string // builds under this directory are refused
}

// New returns the Entry for the current user and the running binary.
func New() (*Entry, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Entry{Dir: filepath.Join(config, "autostart"), Executable: exe, TempDir: os.TempDir()}, nil
}

// Status describes the installed entry.
type Status struct {
	Installed bool
	// Executable is the binary the entry starts ("" if none).
	Executable string
}

// Outdated reports whether the entry starts another binary than exe (after
// Kisetsu was moved, or rebuilt elsewhere).
func (s Status) Outdated(exe string) bool {
	return s.Installed && s.Executable != exe
}

func (e *Entry) path() string { return filepath.Join(e.Dir, FileName) }

// Enable writes the entry for e.Executable, replacing an older one.
func (e *Entry) Enable() error {
	if rel, err := filepath.Rel(e.TempDir, e.Executable); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", ErrTemporaryExecutable, e.Executable)
	}
	if _, err := e.read(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(e.Dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", e.Dir, err)
	}
	return writeFile(e.path(), e.content())
}

// Disable removes the entry. A missing entry is not an error.
func (e *Entry) Disable() error {
	if _, err := e.read(); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := os.Remove(e.path()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Status reads the installed entry.
func (e *Entry) Status() (Status, error) {
	content, err := e.read()
	if errors.Is(err, os.ErrNotExist) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	st := Status{Installed: true}
	for line := range strings.SplitSeq(content, "\n") {
		if v, ok := strings.CutPrefix(line, "TryExec="); ok {
			st.Executable = unescapeValue(v)
		}
	}
	return st, nil
}

// read returns the entry if Kisetsu wrote it, ErrForeign if not, or an
// error wrapping os.ErrNotExist if there is none.
func (e *Entry) read() (string, error) {
	data, err := os.ReadFile(e.path())
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(data), "\n"+marker+"\n") && !strings.HasPrefix(string(data), marker+"\n") {
		return "", fmt.Errorf("%w: %s", ErrForeign, e.path())
	}
	return string(data), nil
}

// content is the desktop entry. TryExec makes the desktop skip the entry
// when the binary is gone (Kisetsu uninstalled), instead of failing.
func (e *Entry) content() string {
	return marker + "\n" +
		"# Starts the Kisetsu agent, which changes the wallpaper. Turn rotation\n" +
		"# off in Kisetsu to remove this file.\n" +
		"[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Kisetsu agent\n" +
		"Comment=Changes the wallpaper in the background\n" +
		"Icon=io.github.sentivanac.Kisetsu\n" +
		"TryExec=" + escapeValue(e.Executable) + "\n" +
		"Exec=" + escapeValue(quoteExecArg(e.Executable)) + " agent\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// quoteExecArg quotes one Exec argument as the Desktop Entry
// Specification requires: inside double quotes, `"`, "`", `$` and `\` are
// escaped with a backslash, and a literal `%` is written `%%`.
func quoteExecArg(arg string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`)
	return `"` + r.Replace(arg) + `"`
}

// escapeValue escapes a string value of a desktop entry: backslashes and
// line breaks.
func escapeValue(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return r.Replace(s)
}

func unescapeValue(s string) string {
	r := strings.NewReplacer(`\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t", `\s`, " ")
	return r.Replace(s)
}

// writeFile replaces path atomically, so the desktop never reads half a
// file.
func writeFile(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kisetsu-autostart-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
