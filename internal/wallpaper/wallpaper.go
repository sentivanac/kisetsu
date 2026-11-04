// Package wallpaper sets the desktop wallpaper.
//
// When Variety is running it is asked to show the image (`variety --set`),
// so it keeps managing the desktop and applies its effects. Otherwise the
// desktop's own setting is changed: with gsettings on MATE, GNOME and
// Cinnamon, with plasma-apply-wallpaperimage on KDE Plasma, and with
// xfconf-query on XFCE. Commands are run directly, never through a shell.
// On Windows, the wallpaper is set through the Windows API.
package wallpaper

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Method says how the wallpaper was set.
type Method string

const (
	Variety  Method = "variety"
	MATE     Method = "mate"
	GNOME    Method = "gnome"
	Cinnamon Method = "cinnamon"
	KDE      Method = "kde"
	XFCE     Method = "xfce"
	Windows  Method = "windows"
)

var (
	// ErrUnsupportedDesktop means neither Variety nor a supported desktop
	// environment was found.
	ErrUnsupportedDesktop = errors.New("unsupported desktop environment")
	// ErrNotApplied means the command succeeded but the setting did not
	// change.
	ErrNotApplied = errors.New("wallpaper setting did not change")
)

// Runner runs commands. Tests replace it, so they never touch the real
// desktop.
type Runner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, args ...string) (stdout []byte, err error)
}

// ExecRunner runs real commands.
type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
