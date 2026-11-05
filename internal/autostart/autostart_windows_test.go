//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// newEntry uses a key of its own, never the real Run key.
func newEntry(t *testing.T, exe string) *Entry {
	t.Helper()
	parent := `Software\Kisetsu-test`
	key := fmt.Sprintf(`%s\%d`, parent, time.Now().UnixNano())
	t.Cleanup(func() {
		registry.DeleteKey(registry.CURRENT_USER, key)
		registry.DeleteKey(registry.CURRENT_USER, parent) // only if empty
	})
	return &Entry{Key: key, Executable: exe, TempDir: `C:\Temp`}
}

func TestRunValue(t *testing.T) {
	exe := `C:\Users\Ana Šarić\AppData\Local\Programs\Kisetsu\kisetsu.exe`
	e := newEntry(t, exe)
	if st, err := e.Status(); err != nil || st.Installed {
		t.Fatalf("before: %+v, %v", st, err)
	}
	if err := e.Disable(); err != nil {
		t.Errorf("disable without a value: %v", err)
	}
	if err := e.Enable(); err != nil {
		t.Fatal(err)
	}
	k, _ := registry.OpenKey(registry.CURRENT_USER, e.Key, registry.QUERY_VALUE)
	v, _, err := k.GetStringValue(ValueName)
	k.Close()
	if want := `"` + exe + `" agent`; err != nil || v != want {
		t.Errorf("value = %q, %v; want %q", v, err, want)
	}
	st, err := e.Status()
	if err != nil || !st.Installed || st.Executable != exe || st.Outdated(exe) {
		t.Errorf("status = %+v, %v", st, err)
	}
	if !st.Outdated(`D:\elsewhere\kisetsu.exe`) {
		t.Error("an entry for another binary is not outdated")
	}
	if err := e.Disable(); err != nil {
		t.Fatal(err)
	}
	if st, _ := e.Status(); st.Installed {
		t.Error("still installed")
	}
}

func TestRefusesTemporaryBuilds(t *testing.T) {
	e := newEntry(t, `C:\Temp\go-build123\b001\exe\kisetsu.exe`)
	if err := e.Enable(); !errors.Is(err, ErrTemporaryExecutable) {
		t.Errorf("err = %v, want ErrTemporaryExecutable", err)
	}
}

func TestExecutableOf(t *testing.T) {
	for v, want := range map[string]string{
		`"C:\a b\kisetsu.exe" agent`: `C:\a b\kisetsu.exe`,
		`C:\k\kisetsu.exe agent`:     `C:\k\kisetsu.exe`,
	} {
		if got := executableOf(v); got != want {
			t.Errorf("executableOf(%q) = %q, want %q", v, got, want)
		}
	}
}
