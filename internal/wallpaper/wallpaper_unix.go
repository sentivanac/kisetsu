//go:build !windows

package wallpaper

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

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
		uri := fileURI(path)
		// GNOME 42+ keeps a separate picture for the dark style.
		for _, key := range []string{"picture-uri", "picture-uri-dark"} {
			if err := s.gsettings(ctx, "org.gnome.desktop.background", key, uri); err != nil {
				return GNOME, err
			}
		}
		return GNOME, nil
	case Cinnamon:
		// Cinnamon has its own schema (no dark variant). Linux Mint also
		// keeps GNOME's keys, but Cinnamon does not read them.
		return Cinnamon, s.gsettings(ctx, "org.cinnamon.desktop.background", "picture-uri", fileURI(path))
	case KDE:
		return KDE, s.plasma(ctx, path)
	case XFCE:
		return XFCE, s.xfce(ctx, path)
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupportedDesktop, strings.Join(names, ":"))
}

// VarietyRunning reports whether the user runs Variety, which would also
// change the wallpaper.
func (s *Setter) VarietyRunning() bool {
	return s.varietyRunning()
}

// ViaVariety reports whether Set would hand the image to Variety, which
// then shows it with its own effects; Kisetsu draws no text on it.
func (s *Setter) ViaVariety() bool {
	return !s.Direct && s.varietyRunning()
}

// desktopNames returns the names of the desktop the user is logged into,
// such as ["ubuntu", "GNOME"], in this order of trust:
//
//  1. the user's active graphical session according to loginctl, and the
//     DesktopNames of its session file — the same source the login
//     manager uses for XDG_CURRENT_DESKTOP. This stays right even where
//     the environment is stale (as it was for the systemd timer of 0.1.0,
//     ADR 0031);
//  2. XDG_CURRENT_DESKTOP, which the agent, started inside the session,
//     has right.
func (s *Setter) desktopNames(ctx context.Context) []string {
	if names := s.sessionDesktop(ctx); len(names) > 0 {
		return names
	}
	return splitNames(s.Getenv("XDG_CURRENT_DESKTOP"))
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

// fileURI returns path as a percent-encoded file:// URI.
func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// match returns the supported desktop among names, or "".
func match(names []string) Method {
	for _, name := range names {
		switch strings.TrimPrefix(strings.ToUpper(name), "X-") {
		case "MATE":
			return MATE
		case "GNOME":
			return GNOME
		case "CINNAMON", "CINNAMON2D": // X-Cinnamon in the environment; cinnamon2d without 3D
			return Cinnamon
		case "KDE":
			return KDE
		case "XFCE":
			return XFCE
		}
	}
	return ""
}

// plasmaTool is Plasma's own command for setting the wallpaper (Plasma
// 5.24 and newer). It asks the running plasmashell over D-Bus to show the
// image on every screen, on Wayland and X11 alike.
const plasmaTool = "plasma-apply-wallpaperimage"

// plasma sets the wallpaper on KDE Plasma. Plasma saves the setting on its
// own schedule, so there is nothing reliable to read back; a failure shows
// in the command's exit status (for example when plasmashell is not
// running).
func (s *Setter) plasma(ctx context.Context, path string) error {
	if _, err := s.Run.LookPath(plasmaTool); err != nil {
		return fmt.Errorf("%s not found (Plasma 5.24 or newer is needed): %w", plasmaTool, err)
	}
	_, err := s.Run.Run(ctx, plasmaTool, path)
	return err
}

// xfceChannel holds xfdesktop's settings. Each monitor and workspace has
// its own picture, at /backdrop/screen0/monitor<NAME>/workspace<N>/last-image;
// xfdesktop shows a change at once.
const xfceChannel = "xfce4-desktop"

// xfce sets the picture of every monitor and workspace on XFCE (X11).
// xfdesktop stores only pictures the user has changed, so the properties
// that exist are combined with the ones for the current monitors (xrandr)
// and workspaces (xfwm4), and missing ones are created. Each is read back.
func (s *Setter) xfce(ctx context.Context, path string) error {
	if _, err := s.Run.LookPath("xfconf-query"); err != nil {
		return fmt.Errorf("xfconf-query not found: %w", err)
	}
	props, err := s.xfceProperties(ctx)
	if err != nil {
		return err
	}
	for _, prop := range props {
		if _, err := s.Run.Run(ctx, "xfconf-query", "-c", xfceChannel, "-p", prop, "--create", "-t", "string", "-s", path); err != nil {
			return err
		}
		out, err := s.Run.Run(ctx, "xfconf-query", "-c", xfceChannel, "-p", prop)
		if err != nil {
			return err
		}
		if got := strings.TrimSpace(string(out)); got != path {
			return fmt.Errorf("%w: %s %s is %s", ErrNotApplied, xfceChannel, prop, got)
		}
	}
	return nil
}

// xfceProperties lists the last-image properties to set, sorted.
func (s *Setter) xfceProperties(ctx context.Context) ([]string, error) {
	found := map[string]bool{}
	out, err := s.Run.Run(ctx, "xfconf-query", "-c", xfceChannel, "-l")
	if err == nil { // an empty channel is an error too
		for line := range strings.SplitSeq(string(out), "\n") {
			if p := strings.TrimSpace(line); strings.HasPrefix(p, "/backdrop/") && strings.HasSuffix(p, "/last-image") {
				found[p] = true
			}
		}
	}
	workspaces := 1
	if out, err := s.Run.Run(ctx, "xfconf-query", "-c", "xfwm4", "-p", "/general/workspace_count"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && n > 0 && n <= 64 {
			workspaces = n
		}
	}
	for _, monitor := range s.monitors(ctx) {
		for i := range workspaces {
			found[fmt.Sprintf("/backdrop/screen0/monitor%s/workspace%d/last-image", monitor, i)] = true
		}
	}
	if len(found) == 0 {
		return nil, errors.New("no XFCE monitors found (xrandr gave none and no picture is set yet)")
	}
	return slices.Sorted(maps.Keys(found)), nil
}

// monitors returns the names of the connected monitors, such as
// "Virtual-1" or "HDMI-1", which xfdesktop uses in its property names.
func (s *Setter) monitors(ctx context.Context) []string {
	if _, err := s.Run.LookPath("xrandr"); err != nil {
		return nil
	}
	out, err := s.Run.Run(ctx, "xrandr", "--listmonitors")
	if err != nil {
		return nil
	}
	// "Monitors: 1", then lines like " 0: +*Virtual-1 1280/339x800/212+0+0  Virtual-1".
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSuffix(fields[0], ":")); err != nil || !strings.HasSuffix(fields[0], ":") {
			continue // the "Monitors: 1" header
		}
		name := fields[len(fields)-1]
		if !strings.ContainsAny(name, "/ ") {
			names = append(names, name)
		}
	}
	return names
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
