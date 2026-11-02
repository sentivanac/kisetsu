package schedule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeSystemctl records calls; it never runs the real systemctl.
type fakeSystemctl struct {
	calls [][]string
	show  string
	list  string // output of list-timers
	fail  string // fail calls whose arguments contain this word
}

func (f *fakeSystemctl) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.fail != "" && slices.Contains(args, f.fail) {
		return nil, errors.New("systemctl failed")
	}
	if slices.Contains(args, "show") {
		return []byte(f.show), nil
	}
	if slices.Contains(args, "list-timers") {
		return []byte(f.list), nil
	}
	return nil, nil
}

func newManager(t *testing.T) (*Manager, *fakeSystemctl) {
	t.Helper()
	run := &fakeSystemctl{}
	return &Manager{
		UnitDir:    filepath.Join(t.TempDir(), "systemd", "user"),
		Executable: "/home/user/apps/kisetsu",
		Run:        run,
		Getenv:     func(k string) string { return map[string]string{"XDG_CURRENT_DESKTOP": "MATE"}[k] },
		TempDir:    "/tmp",
	}, run
}

func readUnit(t *testing.T, m *Manager, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(m.UnitDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEnableWritesUnitsAndStartsTimer(t *testing.T) {
	m, run := newManager(t)

	if err := m.Enable(context.Background(), time.Hour); err != nil {
		t.Fatal(err)
	}

	service := readUnit(t, m, ServiceName)
	for _, want := range []string{
		marker,
		"Type=oneshot",
		`ExecStart="/home/user/apps/kisetsu" rotate --scheduled`,
		`Environment="KISETSU_DESKTOP_FALLBACK=MATE"`,
	} {
		if !strings.Contains(service, want) {
			t.Errorf("service unit lacks %q:\n%s", want, service)
		}
	}
	timer := readUnit(t, m, TimerName)
	for _, want := range []string{marker, "OnActiveSec=3600s", "OnUnitActiveSec=3600s", "WantedBy=timers.target"} {
		if !strings.Contains(timer, want) {
			t.Errorf("timer unit lacks %q:\n%s", want, timer)
		}
	}
	info, _ := os.Stat(filepath.Join(m.UnitDir, TimerName))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("unit permissions %o", info.Mode().Perm())
	}

	want := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", TimerName},
		{"systemctl", "--user", "restart", TimerName},
	}
	if !slices.EqualFunc(run.calls, want, slices.Equal) {
		t.Errorf("calls = %q\nwant    %q", run.calls, want)
	}
}

func TestEnableAgainChangesTheInterval(t *testing.T) {
	m, _ := newManager(t)
	ctx := context.Background()
	m.Enable(ctx, time.Hour)

	if err := m.Enable(ctx, 15*time.Minute); err != nil {
		t.Fatal(err)
	}
	if timer := readUnit(t, m, TimerName); !strings.Contains(timer, "OnUnitActiveSec=900s") {
		t.Errorf("interval not changed:\n%s", timer)
	}
}

func TestDisableRemovesOnlyKisetsuUnits(t *testing.T) {
	m, run := newManager(t)
	ctx := context.Background()
	m.Enable(ctx, time.Hour)
	other := filepath.Join(m.UnitDir, "screenlayout-resume.service")
	os.WriteFile(other, []byte("[Service]\nExecStart=/bin/true\n"), 0o644)
	run.calls = nil

	if err := m.Disable(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ServiceName, TimerName} {
		if _, err := os.Stat(filepath.Join(m.UnitDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still exists", name)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("the user's own unit was touched: %v", err)
	}
	if run.calls[0][2] != "disable" || run.calls[len(run.calls)-1][2] != "daemon-reload" {
		t.Errorf("calls = %q", run.calls)
	}

	// Disabling again is harmless and does not reload.
	run.calls = nil
	if err := m.Disable(ctx); err != nil {
		t.Errorf("second disable: %v", err)
	}
	if len(run.calls) != 1 {
		t.Errorf("second disable calls = %q", run.calls)
	}
}

func TestForeignUnitFilesAreNeverTouched(t *testing.T) {
	m, run := newManager(t)
	os.MkdirAll(m.UnitDir, 0o755)
	foreign := "[Service]\nExecStart=/usr/bin/my-own-script\n"
	os.WriteFile(filepath.Join(m.UnitDir, ServiceName), []byte(foreign), 0o644)

	if err := m.Enable(context.Background(), time.Hour); !errors.Is(err, ErrForeignUnit) {
		t.Errorf("Enable err = %v, want ErrForeignUnit", err)
	}
	if err := m.Disable(context.Background()); !errors.Is(err, ErrForeignUnit) {
		t.Errorf("Disable err = %v, want ErrForeignUnit", err)
	}
	if got := readUnit(t, m, ServiceName); got != foreign {
		t.Errorf("foreign unit changed:\n%s", got)
	}
	if len(run.calls) != 0 {
		t.Errorf("systemctl called: %q", run.calls)
	}
}

func TestEnableRefusesTemporaryBuilds(t *testing.T) {
	m, run := newManager(t)
	m.Executable = "/tmp/go-build123/b001/exe/kisetsu"

	if err := m.Enable(context.Background(), time.Hour); !errors.Is(err, ErrTemporaryExecutable) {
		t.Errorf("err = %v, want ErrTemporaryExecutable", err)
	}
	if len(run.calls) != 0 {
		t.Error("systemctl called for a temporary build")
	}
	if err := m.Enable(context.Background(), 10*time.Second); err == nil {
		t.Error("a 10 s interval was accepted")
	}
}

func TestEnableReportsSystemctlFailure(t *testing.T) {
	m, run := newManager(t)
	run.fail = "enable"

	if err := m.Enable(context.Background(), time.Hour); err == nil {
		t.Error("a failing systemctl was ignored")
	}
}

func TestStatus(t *testing.T) {
	m, run := newManager(t)
	ctx := context.Background()

	st, err := m.Status(ctx)
	if err != nil || st.Installed {
		t.Fatalf("before enabling: %+v, %v", st, err)
	}

	m.Enable(ctx, time.Hour)
	run.show = "ActiveState=active\nNextElapseUSecRealtime=Mon 2026-11-02 09:21:29 CET\nLastTriggerUSec=n/a\n"
	st, err = m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || !st.Active || st.Executable != m.Executable || !st.LastRun.IsZero() {
		t.Errorf("status = %+v", st)
	}
	if st.NextRun.IsZero() || st.NextRun.Hour() != 9 || st.NextRun.Minute() != 21 {
		t.Errorf("NextRun = %v", st.NextRun)
	}
	if st.Outdated(m.Executable) || !st.Outdated("/elsewhere/kisetsu") {
		t.Error("Outdated is wrong")
	}
}

func TestExecPathQuotingRoundTrips(t *testing.T) {
	for _, path := range []string{
		"/home/user/apps/kisetsu",
		"/home/user/My Apps/kisetsu",
		`/home/user/100% "real"/$HOME\x/kisetsu`,
	} {
		quoted := quoteExecPath(path)
		if got := unquoteExecPath(quoted + " rotate --scheduled"); got != path {
			t.Errorf("round trip of %q: quoted %s, got %q", path, quoted, got)
		}
	}
	if got := quoteSystemd(`a%b$c"d\e`); got != `"a%%b$$c\"d\\e"` {
		t.Errorf("quoteSystemd = %s", got)
	}
}

// Real output for Kisetsu's monotonic timer (systemd 249, 2026-11-02): the
// realtime property is empty, so the time comes from list-timers.
func TestStatusOfMonotonicTimer(t *testing.T) {
	m, run := newManager(t)
	ctx := context.Background()
	m.Enable(ctx, 15*time.Minute)
	run.show = "NextElapseUSecRealtime=\nNextElapseUSecMonotonic=1d 3h 17min 57.470100s\nLastTriggerUSec=\nActiveState=active\n"
	run.list = "Mon 2026-11-02 07:17:20 CET 11min left n/a n/a kisetsu-rotate.timer kisetsu-rotate.service\n"

	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.NextRun.IsZero() || st.NextRun.Hour() != 7 || st.NextRun.Minute() != 17 {
		t.Errorf("NextRun = %v, want 07:17", st.NextRun)
	}
	if !st.LastRun.IsZero() {
		t.Errorf("LastRun = %v, want zero before the first run", st.LastRun)
	}
}

func TestParseListTimersNext(t *testing.T) {
	if got := parseListTimersNext("n/a n/a n/a n/a kisetsu-rotate.timer kisetsu-rotate.service"); !got.IsZero() {
		t.Errorf("n/a = %v", got)
	}
	if got := parseListTimersNext(""); !got.IsZero() {
		t.Errorf("empty = %v", got)
	}
}

func TestRestart(t *testing.T) {
	m, run := newManager(t)
	ctx := context.Background()

	if err := m.Restart(ctx); err != nil || len(run.calls) != 0 {
		t.Errorf("without a timer: err %v, calls %q", err, run.calls)
	}

	m.Enable(ctx, time.Hour)
	run.calls = nil
	if err := m.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"systemctl", "--user", "restart", TimerName}}; !slices.EqualFunc(run.calls, want, slices.Equal) {
		t.Errorf("calls = %q", run.calls)
	}
}
