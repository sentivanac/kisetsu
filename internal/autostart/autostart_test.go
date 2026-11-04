package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newEntry(t *testing.T, exe string) *Entry {
	t.Helper()
	return &Entry{Dir: filepath.Join(t.TempDir(), "autostart"), Executable: exe, TempDir: "/tmp"}
}

func TestEnableWritesAnEntry(t *testing.T) {
	e := newEntry(t, "/usr/bin/kisetsu")
	if err := e.Enable(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(e.Dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Created by Kisetsu.\n", "[Desktop Entry]\n", "Type=Application\n",
		"TryExec=/usr/bin/kisetsu\n", `Exec="/usr/bin/kisetsu" agent` + "\n", "NoDisplay=true\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("entry lacks %q:\n%s", want, data)
		}
	}
	info, _ := os.Stat(filepath.Join(e.Dir, FileName))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	st, err := e.Status()
	if err != nil || !st.Installed || st.Executable != "/usr/bin/kisetsu" || st.Outdated("/usr/bin/kisetsu") {
		t.Errorf("status = %+v, %v", st, err)
	}
	if !st.Outdated("/opt/kisetsu/kisetsu") {
		t.Error("an entry for another binary is not outdated")
	}
}

func TestQuotesUnusualPaths(t *testing.T) {
	exe := `/home/a b/100% "x"/$HOME/k\s`
	e := newEntry(t, exe)
	if err := e.Enable(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(e.Dir, FileName))
	// Exec quoting (spec), then string-value escaping of the backslashes.
	if want := `Exec="/home/a b/100%% \\"x\\"/\\$HOME/k\\\\s" agent`; !strings.Contains(string(data), want) {
		t.Errorf("entry lacks %s:\n%s", want, data)
	}
	if st, _ := e.Status(); st.Executable != exe {
		t.Errorf("TryExec reads back as %q, want %q", st.Executable, exe)
	}
}

func TestDisableRemovesOnlyItsOwnEntry(t *testing.T) {
	e := newEntry(t, "/usr/bin/kisetsu")
	if err := e.Disable(); err != nil {
		t.Errorf("disable without an entry: %v", err)
	}
	e.Enable()
	if err := e.Disable(); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Status(); st.Installed {
		t.Error("entry still installed")
	}

	path := filepath.Join(e.Dir, FileName)
	os.WriteFile(path, []byte("[Desktop Entry]\nExec=my-own-script\n"), 0o644)
	if err := e.Disable(); !errors.Is(err, ErrForeign) {
		t.Errorf("disable foreign: err = %v, want ErrForeign", err)
	}
	if err := e.Enable(); !errors.Is(err, ErrForeign) {
		t.Errorf("enable over foreign: err = %v, want ErrForeign", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "[Desktop Entry]\nExec=my-own-script\n" {
		t.Errorf("foreign entry changed: %q", data)
	}
	if _, err := e.Status(); !errors.Is(err, ErrForeign) {
		t.Errorf("status of foreign: err = %v", err)
	}
}

func TestRefusesTemporaryBuilds(t *testing.T) {
	e := newEntry(t, "/tmp/go-build123/b001/exe/kisetsu")
	if err := e.Enable(); !errors.Is(err, ErrTemporaryExecutable) {
		t.Errorf("err = %v, want ErrTemporaryExecutable", err)
	}
}
