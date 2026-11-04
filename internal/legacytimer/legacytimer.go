// Package legacytimer removes the systemd user timer with which Kisetsu
// 0.1.0 rotated the wallpaper (ADR 0025). The background agent replaced it
// (ADR 0041); this package only finds the timer and removes it.
//
// Only unit files Kisetsu created (marked on their first line) are
// removed; files of the same name written by someone else are left alone.
package legacytimer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	ServiceName = "kisetsu-rotate.service"
	TimerName   = "kisetsu-rotate.timer"

	// marker is the first line of every unit file Kisetsu wrote.
	marker = "# Created by Kisetsu."
)

// ErrForeignUnit means a unit file with Kisetsu's name exists but was not
// created by Kisetsu.
var ErrForeignUnit = errors.New("a unit file with this name was not created by Kisetsu")

// Runner runs commands; tests replace it so no real systemctl is called.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs real commands.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Timer is the 0.1.0 rotation timer of the current user.
type Timer struct {
	UnitDir string // ~/.config/systemd/user
	Run     Runner
}

// New returns the Timer of the current user.
func New() (*Timer, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Timer{UnitDir: filepath.Join(config, "systemd", "user"), Run: ExecRunner{}}, nil
}

// Installed reports whether Kisetsu's unit files (either of them) exist.
func (t *Timer) Installed() (bool, error) {
	found := false
	for _, name := range []string{ServiceName, TimerName} {
		owned, err := t.owned(name)
		if errors.Is(err, ErrForeignUnit) {
			continue
		}
		if err != nil {
			return false, err
		}
		found = found || owned
	}
	return found, nil
}

// Disable stops the timer and removes both unit files. Missing files are
// not an error; files Kisetsu did not create are left alone.
func (t *Timer) Disable(ctx context.Context) error {
	for _, name := range []string{ServiceName, TimerName} {
		if _, err := t.owned(name); err != nil {
			return err
		}
	}
	// The timer may not be loaded (already removed); that is fine.
	t.Run.Run(ctx, "systemctl", "--user", "disable", "--now", TimerName)

	removed := false
	for _, name := range []string{TimerName, ServiceName} {
		err := os.Remove(filepath.Join(t.UnitDir, name))
		switch {
		case err == nil:
			removed = true
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	if removed {
		if _, err := t.Run.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
	}
	return nil
}

// owned reports whether name exists with Kisetsu's marker; a file without
// it gives ErrForeignUnit.
func (t *Timer) owned(name string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(t.UnitDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(string(data), marker) {
		return false, fmt.Errorf("%w: %s", ErrForeignUnit, filepath.Join(t.UnitDir, name))
	}
	return true, nil
}
