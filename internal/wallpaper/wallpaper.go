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
	// DataDirs are searched for xsessions/ and wayland-sessions/ (normally
	// $XDG_DATA_DIRS, or /usr/local/share and /usr/share).
	DataDirs []string
}

// FallbackEnv names the variable the rotation timer sets to the desktop
// found when rotation was enabled. It is used only when the desktop cannot
// be found otherwise; it never overrides the running session.
const FallbackEnv = "KISETSU_DESKTOP_FALLBACK"

// New returns a Setter for the current user and environment.
func New() *Setter {
	dirs := filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))
	if len(dirs) == 0 {
		dirs = []string{"/usr/local/share", "/usr/share"}
	}
	return &Setter{Run: ExecRunner{}, Getenv: os.Getenv, ProcDir: "/proc", UID: os.Getuid(), DataDirs: dirs}
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

	names := s.desktopNames(ctx)
	switch desktop := match(names); desktop {
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
	return "", fmt.Errorf("%w: %q", ErrUnsupportedDesktop, strings.Join(names, ":"))
}

// VarietyRunning reports whether the user runs Variety, which would also
// change the wallpaper.
func (s *Setter) VarietyRunning() bool {
	return s.varietyRunning()
}

// desktopNames returns the names of the desktop the user is logged into,
// such as ["ubuntu", "GNOME"], in this order of trust:
//
//  1. the user's active graphical session according to loginctl, and the
//     DesktopNames of its session file — the same source the login
//     manager uses for XDG_CURRENT_DESKTOP. This stays right for the
//     rotation timer, whose environment can be stale: the systemd user
//     manager keeps the variables of the first session, and sessions such
//     as i3 do not update them;
//  2. XDG_CURRENT_DESKTOP;
//  3. FallbackEnv, set by the timer unit.
func (s *Setter) desktopNames(ctx context.Context) []string {
	if names := s.sessionDesktop(ctx); len(names) > 0 {
		return names
	}
	for _, env := range []string{"XDG_CURRENT_DESKTOP", FallbackEnv} {
		if names := splitNames(s.Getenv(env)); len(names) > 0 {
			return names
		}
	}
	return nil
}

// sessionDesktop asks loginctl for the user's graphical session and reads
// DesktopNames from its session file.
func (s *Setter) sessionDesktop(ctx context.Context) []string {
	if _, err := s.Run.LookPath("loginctl"); err != nil {
		return nil
	}
	out, err := s.Run.Run(ctx, "loginctl", "show-user", strconv.Itoa(s.UID), "--property=Display", "--value")
	session := strings.TrimSpace(string(out))
	if err != nil || session == "" {
		return nil
	}
	// Read by name: loginctl prints properties in its own order, not in
	// the order they were asked for.
	out, err = s.Run.Run(ctx, "loginctl", "show-session", session, "--property=Desktop", "--property=Type")
	if err != nil {
		return nil
	}
	var name, kind string
	for line := range strings.SplitSeq(string(out), "\n") {
		if v, ok := strings.CutPrefix(line, "Desktop="); ok {
			name = strings.TrimSpace(v)
		} else if v, ok := strings.CutPrefix(line, "Type="); ok {
			kind = strings.TrimSpace(v)
		}
	}
	// The name becomes part of a file path.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil
	}
	return s.sessionFileNames(name, kind)
}

// sessionFileNames reads DesktopNames from a session file (Wayland
// sessions are looked up first for Wayland). A file without DesktopNames,
// or no file at all, means the session name itself.
func (s *Setter) sessionFileNames(name, kind string) []string {
	subdirs := []string{"xsessions", "wayland-sessions"}
	if kind == "wayland" {
		subdirs = []string{"wayland-sessions", "xsessions"}
	}
	for _, sub := range subdirs {
		for _, dir := range s.DataDirs {
			data, err := os.ReadFile(filepath.Join(dir, sub, name+".desktop"))
			if err != nil {
				continue
			}
			for line := range strings.SplitSeq(string(data), "\n") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(line), "DesktopNames="); ok {
					if names := splitNames(v); len(names) > 0 {
						return names
					}
				}
			}
			return []string{name}
		}
	}
	return []string{name}
}

// splitNames splits a desktop list; session files use ";" and the
// environment ":".
func splitNames(s string) []string {
	var names []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ':' || r == ';' }) {
		if part = strings.TrimSpace(part); part != "" {
			names = append(names, part)
		}
	}
	return names
}

// match returns the supported desktop among names, or "".
func match(names []string) Method {
	for _, name := range names {
		switch strings.ToUpper(name) {
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
