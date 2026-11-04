package legacytimer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeRunner records commands; no real systemctl is ever called.
type fakeRunner struct {
	calls [][]string
	fail  bool
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.fail {
		return nil, errors.New("systemctl failed")
	}
	return nil, nil
}

// newTimer returns a Timer whose unit directory holds the files of 0.1.0.
func newTimer(t *testing.T, files map[string]string) (*Timer, *fakeRunner) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "systemd", "user")
	os.MkdirAll(dir, 0o755)
	for name, content := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	run := &fakeRunner{}
	return &Timer{UnitDir: dir, Run: run}, run
}

var kisetsuUnits = map[string]string{
	ServiceName: marker + "\n[Service]\nType=oneshot\nExecStart=\"/usr/bin/kisetsu\" rotate --scheduled\n",
	TimerName:   marker + "\n[Timer]\nOnUnitActiveSec=3600s\n",
}

func TestDisableRemovesOnlyKisetsuUnits(t *testing.T) {
	tm, run := newTimer(t, kisetsuUnits)
	other := filepath.Join(tm.UnitDir, "screenlayout-resume.service")
	os.WriteFile(other, []byte("[Service]\nExecStart=/bin/true\n"), 0o644)

	if ok, err := tm.Installed(); !ok || err != nil {
		t.Fatalf("Installed = %v, %v", ok, err)
	}
	if err := tm.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{ServiceName, TimerName} {
		if _, err := os.Stat(filepath.Join(tm.UnitDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still exists", name)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("the user's own unit was touched: %v", err)
	}
	if len(run.calls) != 2 || run.calls[0][2] != "disable" || run.calls[1][2] != "daemon-reload" {
		t.Errorf("calls = %q", run.calls)
	}
	if ok, _ := tm.Installed(); ok {
		t.Error("still installed")
	}

	// Disabling again is harmless and does not reload.
	run.calls = nil
	if err := tm.Disable(context.Background()); err != nil {
		t.Errorf("second disable: %v", err)
	}
	if len(run.calls) != 1 {
		t.Errorf("second disable calls = %q", run.calls)
	}
}

func TestForeignUnitFilesAreNeverTouched(t *testing.T) {
	foreign := "[Service]\nExecStart=/usr/bin/my-own-script\n"
	tm, run := newTimer(t, map[string]string{ServiceName: foreign})

	if ok, err := tm.Installed(); ok || err != nil {
		t.Errorf("Installed = %v, %v; want false (not Kisetsu's)", ok, err)
	}
	if err := tm.Disable(context.Background()); !errors.Is(err, ErrForeignUnit) {
		t.Errorf("Disable err = %v, want ErrForeignUnit", err)
	}
	if data, _ := os.ReadFile(filepath.Join(tm.UnitDir, ServiceName)); string(data) != foreign {
		t.Errorf("foreign unit changed:\n%s", data)
	}
	if len(run.calls) != 0 {
		t.Errorf("systemctl called: %q", run.calls)
	}
}

func TestNothingInstalled(t *testing.T) {
	tm, run := newTimer(t, nil)
	if ok, err := tm.Installed(); ok || err != nil {
		t.Errorf("Installed = %v, %v", ok, err)
	}
	if err := tm.Disable(context.Background()); err != nil {
		t.Errorf("Disable: %v", err)
	}
	if len(run.calls) != 1 { // only the harmless "disable --now"
		t.Errorf("calls = %q", run.calls)
	}
}

func TestDisableReportsReloadFailure(t *testing.T) {
	tm, run := newTimer(t, kisetsuUnits)
	run.fail = true
	if err := tm.Disable(context.Background()); err == nil {
		t.Error("a failed daemon-reload was not reported")
	}
}
