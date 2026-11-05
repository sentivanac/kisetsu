// Package autostart starts the Kisetsu agent when the user logs in: on
// Linux with an XDG autostart entry (~/.config/autostart/*.desktop), which
// GNOME, KDE Plasma, Xfce, MATE and Cinnamon read (window managers such as
// i3 and sway do not); on Windows with a value in the user's Run key.
//
// Kisetsu only changes or removes an entry it created itself, so one the
// user wrote under the same name is never touched.
package autostart

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrForeign means an entry with Kisetsu's name exists that Kisetsu
	// did not create.
	ErrForeign = errors.New("autostart entry was not created by Kisetsu")
	// ErrTemporaryExecutable means Kisetsu runs from a temporary build
	// (for example `go run` or `wails dev`), which would be gone at the
	// next login.
	ErrTemporaryExecutable = errors.New("kisetsu runs from a temporary directory")
)

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

// checkExecutable refuses a binary under tempDir, which would be gone at
// the next login.
func checkExecutable(exe, tempDir string) error {
	if rel, err := filepath.Rel(tempDir, exe); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", ErrTemporaryExecutable, exe)
	}
	return nil
}
