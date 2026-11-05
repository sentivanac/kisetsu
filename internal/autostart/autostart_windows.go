//go:build windows

// The Run key of the current user (Windows).

package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	// RunKey is the key, under HKEY_CURRENT_USER, whose values Windows
	// starts at sign-in. No administrator rights are needed.
	RunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	// ValueName is Kisetsu's value in it, after the application ID; a
	// value of this name is Kisetsu's own.
	ValueName = "io.github.sentivanac.Kisetsu.Agent"
)

// Location names the entry in messages.
const Location = `HKEY_CURRENT_USER\` + RunKey + ` (` + ValueName + `)`

// Entry is the autostart entry for the running Kisetsu binary.
type Entry struct {
	Key        string // under HKEY_CURRENT_USER; RunKey (tests use their own)
	Executable string // absolute path of kisetsu.exe
	TempDir    string // builds under this directory are refused
}

// New returns the Entry for the current user and the running binary.
func New() (*Entry, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return &Entry{Key: RunKey, Executable: exe, TempDir: os.TempDir()}, nil
}

// Enable writes the value for e.Executable, replacing an older one.
func (e *Entry) Enable() error {
	if err := checkExecutable(e.Executable, e.TempDir); err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, e.Key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(ValueName, command(e.Executable))
}

// Disable removes the value. A missing value is not an error.
func (e *Entry) Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.Key, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(ValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// Status reads the installed value.
func (e *Entry) Status() (Status, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.Key, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(ValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Installed: true, Executable: executableOf(v)}, nil
}

// command is the value: the quoted program and "agent". Windows paths
// cannot contain quotes, so no escaping is needed.
func command(exe string) string {
	return `"` + exe + `" agent`
}

// executableOf reads the program back from a value.
func executableOf(v string) string {
	if rest, ok := strings.CutPrefix(v, `"`); ok {
		exe, _, _ := strings.Cut(rest, `"`)
		return exe
	}
	exe, _, _ := strings.Cut(v, " ")
	return exe
}
