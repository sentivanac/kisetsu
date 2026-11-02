// Package schedule installs the systemd user timer that runs
// `kisetsu rotate --scheduled`.
//
// It only ever writes, changes or removes its own two unit files, and
// refuses to touch files of the same name that it did not create.
package schedule

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	ServiceName = "kisetsu-rotate.service"
	TimerName   = "kisetsu-rotate.timer"

	// marker is the first line of every unit file Kisetsu writes.
	marker = "# Created by Kisetsu."

	// systemdTime is how systemctl show prints times, e.g.
	// "Mon 2026-11-02 09:21:29 CET".
	systemdTime = "Mon 2006-01-02 15:04:05 MST"
)

var (
	// ErrForeignUnit means a unit file with Kisetsu's name exists but was
	// not created by Kisetsu; it is left alone.
	ErrForeignUnit = errors.New("a unit file with this name was not created by Kisetsu")
	// ErrTemporaryExecutable means Kisetsu runs from a temporary location,
	// which the timer could not rely on.
	ErrTemporaryExecutable = errors.New("Kisetsu is running from a temporary location")
)

// Runner runs commands; tests replace it so no real systemctl is called.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs real commands.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Kisetsu parses dates in systemctl output; keep them independent of
	// the user's language.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Manager installs and removes the rotation timer.
type Manager struct {
	UnitDir    string // ~/.config/systemd/user
	Executable string // absolute path of the kisetsu binary
	Run        Runner
	Getenv     func(string) string
	TempDir    string // builds under this directory are refused
}

// New returns a Manager for the current user and the running binary.
func New() (*Manager, error) {
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
	return &Manager{
		UnitDir:    filepath.Join(config, "systemd", "user"),
		Executable: exe,
		Run:        ExecRunner{},
		Getenv:     os.Getenv,
		TempDir:    os.TempDir(),
	}, nil
}

// Enable writes the units for the given interval and starts the timer.
// Calling it again with another interval replaces the schedule.
func (m *Manager) Enable(ctx context.Context, interval time.Duration) error {
	if interval < time.Minute {
		return fmt.Errorf("interval %s is too short", interval)
	}
	if rel, err := filepath.Rel(m.TempDir, m.Executable); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("%w: %s", ErrTemporaryExecutable, m.Executable)
	}
	for _, name := range []string{ServiceName, TimerName} {
		if err := m.checkOwned(name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(m.UnitDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", m.UnitDir, err)
	}
	if err := writeFile(filepath.Join(m.UnitDir, ServiceName), m.serviceUnit()); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(m.UnitDir, TimerName), timerUnit(interval)); err != nil {
		return err
	}

	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", TimerName},
		// restart (not start) so a changed interval takes effect now.
		{"--user", "restart", TimerName},
	} {
		if _, err := m.Run.Run(ctx, "systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

// Disable stops the timer and removes both unit files. Missing files are
// not an error; files Kisetsu did not create are left alone.
func (m *Manager) Disable(ctx context.Context) error {
	for _, name := range []string{ServiceName, TimerName} {
		if err := m.checkOwned(name); err != nil {
			return err
		}
	}
	// The timer may not be loaded (already removed); that is fine.
	m.Run.Run(ctx, "systemctl", "--user", "disable", "--now", TimerName)

	removed := false
	for _, name := range []string{TimerName, ServiceName} {
		err := os.Remove(filepath.Join(m.UnitDir, name))
		switch {
		case err == nil:
			removed = true
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	if removed {
		if _, err := m.Run.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
	}
	return nil
}

// Restart starts the timer's countdown again, so the next change comes one
// full interval from now. It is used after the user changes the wallpaper
// by hand. Without Kisetsu's timer installed it does nothing.
func (m *Manager) Restart(ctx context.Context) error {
	_, owned, err := m.installedExecutable()
	if err != nil || !owned {
		return err
	}
	_, err = m.Run.Run(ctx, "systemctl", "--user", "restart", TimerName)
	return err
}

// Status describes the installed timer.
type Status struct {
	Installed bool      // Kisetsu's unit files exist
	Active    bool      // the timer is running
	NextRun   time.Time // zero when unknown
	LastRun   time.Time // zero when it has not run yet
	// Executable is the program the installed service runs; it differs
	// from Manager.Executable when Kisetsu was moved or rebuilt elsewhere.
	Executable string
}

// Outdated reports whether the installed service runs another binary
// than the one asking.
func (s Status) Outdated(executable string) bool {
	return s.Installed && s.Executable != executable
}

// Status reads the unit files and asks systemd about the timer.
func (m *Manager) Status(ctx context.Context) (Status, error) {
	exe, owned, err := m.installedExecutable()
	if err != nil || !owned {
		return Status{}, err
	}
	st := Status{Installed: true, Executable: exe}

	out, err := m.Run.Run(ctx, "systemctl", "--user", "show", TimerName,
		"--property=ActiveState,NextElapseUSecRealtime,LastTriggerUSec")
	if err != nil {
		return st, err
	}
	props := parseProperties(string(out))
	st.Active = props["ActiveState"] == "active"
	st.NextRun = parseTime(props["NextElapseUSecRealtime"])
	st.LastRun = parseTime(props["LastTriggerUSec"])

	// Kisetsu's timer is monotonic (OnActiveSec/OnUnitActiveSec), for which
	// NextElapseUSecRealtime stays empty. Its NextElapseUSecMonotonic counts
	// from boot without suspended time, so it cannot be turned into a clock
	// time here; list-timers prints the wall-clock time systemd computes.
	if st.NextRun.IsZero() && st.Active {
		out, err := m.Run.Run(ctx, "systemctl", "--user", "list-timers", "--all", "--no-legend", TimerName)
		if err != nil {
			return st, err
		}
		st.NextRun = parseListTimersNext(string(out))
	}
	return st, nil
}

// parseListTimersNext reads the NEXT column of `systemctl list-timers
// --no-legend`, whose first four words are a time such as
// "Mon 2026-11-02 07:17:20 CET"; "n/a" gives the zero time.
func parseListTimersNext(out string) time.Time {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return time.Time{}
	}
	return parseTime(strings.Join(fields[:4], " "))
}

// checkOwned fails if name exists without Kisetsu's marker.
func (m *Manager) checkOwned(name string) error {
	data, err := os.ReadFile(filepath.Join(m.UnitDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(data), marker) {
		return fmt.Errorf("%w: %s", ErrForeignUnit, filepath.Join(m.UnitDir, name))
	}
	return nil
}

// installedExecutable returns the program of the installed service, and
// whether Kisetsu's service file exists at all.
func (m *Manager) installedExecutable() (string, bool, error) {
	f, err := os.Open(filepath.Join(m.UnitDir, ServiceName))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() || !strings.HasPrefix(sc.Text(), marker) {
		return "", false, nil
	}
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "ExecStart="); ok {
			return unquoteExecPath(rest), true, nil
		}
	}
	return "", true, sc.Err()
}

func (m *Manager) serviceUnit() string {
	var b strings.Builder
	fmt.Fprintln(&b, marker+" Changes are overwritten when rotation settings change.")
	fmt.Fprintln(&b, "[Unit]")
	fmt.Fprintln(&b, "Description=Kisetsu: change the wallpaper")
	fmt.Fprintln(&b, "Documentation=https://github.com/sentivanac/kisetsu")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[Service]")
	fmt.Fprintln(&b, "Type=oneshot")
	fmt.Fprintf(&b, "ExecStart=%s rotate --scheduled\n", quoteExecPath(m.Executable))
	// Fallback for sessions that do not export the desktop to systemd;
	// an exported value is the same desktop.
	if desktop := m.Getenv("XDG_CURRENT_DESKTOP"); desktop != "" {
		fmt.Fprintf(&b, "Environment=%s\n", quoteSystemd("XDG_CURRENT_DESKTOP="+desktop))
	}
	return b.String()
}

func timerUnit(interval time.Duration) string {
	sec := int64(interval / time.Second)
	var b strings.Builder
	fmt.Fprintln(&b, marker+" Changes are overwritten when rotation settings change.")
	fmt.Fprintln(&b, "[Unit]")
	fmt.Fprintln(&b, "Description=Kisetsu wallpaper rotation")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[Timer]")
	// OnUnitActiveSec alone never fires before the service has run once, so
	// OnActiveSec schedules the first change after the timer starts.
	fmt.Fprintf(&b, "OnActiveSec=%ds\n", sec)
	fmt.Fprintf(&b, "OnUnitActiveSec=%ds\n", sec)
	fmt.Fprintln(&b, "AccuracySec=30s")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "[Install]")
	fmt.Fprintln(&b, "WantedBy=timers.target")
	return b.String()
}

// quoteSystemd writes s as a double-quoted systemd value: backslashes and
// quotes are escaped, "%" (specifiers) and "$" (variables) are doubled.
func quoteSystemd(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `$$`)
	return `"` + r.Replace(s) + `"`
}

func quoteExecPath(path string) string { return quoteSystemd(path) }

// unquoteExecPath reverses quoteExecPath for the first word of ExecStart.
func unquoteExecPath(execStart string) string {
	if !strings.HasPrefix(execStart, `"`) {
		path, _, _ := strings.Cut(execStart, " ")
		return path
	}
	var b strings.Builder
	s := execStart[1:]
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case (s[i] == '%' || s[i] == '$') && i+1 < len(s) && s[i+1] == s[i]:
			i++
			b.WriteByte(s[i])
		case s[i] == '"':
			return b.String()
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func parseProperties(out string) map[string]string {
	props := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[k] = strings.TrimSpace(v)
		}
	}
	return props
}

// parseTime reads a systemctl time in the local zone; "", "n/a" and other
// unknown values give the zero time.
func parseTime(s string) time.Time {
	t, err := time.ParseInLocation(systemdTime, s, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

// writeFile replaces path atomically, so systemd never reads a half file.
func writeFile(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kisetsu-unit-*")
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
