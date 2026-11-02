// Package wallpaper sets the desktop wallpaper.
//
// When Variety is running it is asked to show the image (`variety --set`),
// so it keeps managing the desktop and applies its effects. Otherwise the
// desktop's own setting is changed with gsettings: MATE and GNOME are
// supported. Commands are run directly, never through a shell.
package wallpaper

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Method says how the wallpaper was set.
type Method string

const (
	Variety Method = "variety"
	MATE    Method = "mate"
	GNOME   Method = "gnome"
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

// Setter sets wallpapers.
type Setter struct {
	Run    Runner
	Getenv func(string) string
	// ProcDir is where running processes are listed ("/proc" on Linux).
	ProcDir string
	// UID is the user whose Variety counts (normally os.Getuid()).
	UID int
	// Direct skips Variety and always uses the desktop setting. Rotation
	// uses it: the user chose that Kisetsu, not Variety, rotates.
	Direct bool
}

// New returns a Setter for the current user and environment.
func New() *Setter {
	return &Setter{Run: ExecRunner{}, Getenv: os.Getenv, ProcDir: "/proc", UID: os.Getuid()}
}

// Set makes the image at path the wallpaper. path must be absolute.
func (s *Setter) Set(ctx context.Context, path string) (Method, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("wallpaper path %q is not absolute", path)
	}
	if !s.Direct && s.varietyRunning() {
		if _, err := s.Run.Run(ctx, "variety", "--set", path); err != nil {
			return Variety, err
		}
		return Variety, nil
	}

	switch desktop := s.desktop(); desktop {
	case MATE:
		return MATE, s.gsettings(ctx, "org.mate.background", "picture-filename", path)
	case GNOME:
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		// GNOME 42+ keeps a separate picture for the dark style.
		for _, key := range []string{"picture-uri", "picture-uri-dark"} {
			if err := s.gsettings(ctx, "org.gnome.desktop.background", key, uri); err != nil {
				return GNOME, err
			}
		}
		return GNOME, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupportedDesktop, s.Getenv("XDG_CURRENT_DESKTOP"))
}

// VarietyRunning reports whether the user runs Variety, which would also
// change the wallpaper.
func (s *Setter) VarietyRunning() bool {
	return s.varietyRunning()
}

// desktop reads XDG_CURRENT_DESKTOP, a colon-separated list such as
// "ubuntu:GNOME".
func (s *Setter) desktop() Method {
	for name := range strings.SplitSeq(s.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		switch strings.ToUpper(strings.TrimSpace(name)) {
		case "MATE":
			return MATE
		case "GNOME":
			return GNOME
		}
	}
	return ""
}

// gsettings sets a string key and reads it back to confirm the change.
func (s *Setter) gsettings(ctx context.Context, schema, key, value string) error {
	if _, err := s.Run.LookPath("gsettings"); err != nil {
		return fmt.Errorf("gsettings not found: %w", err)
	}
	quoted := quoteGVariant(value)
	if _, err := s.Run.Run(ctx, "gsettings", "set", schema, key, quoted); err != nil {
		return err
	}
	out, err := s.Run.Run(ctx, "gsettings", "get", schema, key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != quoted {
		return fmt.Errorf("%w: %s %s is %s", ErrNotApplied, schema, key, strings.TrimSpace(string(out)))
	}
	return nil
}

// quoteGVariant writes s as a GVariant string literal, the format gsettings
// reads and prints: single quotes, with backslashes and quotes escaped.
func quoteGVariant(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

// varietyRunning reports whether the user runs Variety. It only reads
// /proc: calling the variety command would start Variety if it is not
// running.
func (s *Setter) varietyRunning() bool {
	entries, err := os.ReadDir(s.ProcDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil || !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.ProcDir, e.Name())
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "variety" {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == s.UID {
			return true
		}
	}
	return false
}
