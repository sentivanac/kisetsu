// Package config resolves where Kisetsu stores its files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const appDirName = "kisetsu"

// dirPerm keeps Kisetsu directories private: they will hold search
// history, collections and, later, provider API keys.
const dirPerm = 0o700

// Paths are the per-user directories used by Kisetsu, following the
// XDG Base Directory Specification.
type Paths struct {
	Config string // settings, e.g. ~/.config/kisetsu
	Data   string // database and downloaded images, e.g. ~/.local/share/kisetsu
	Cache  string // re-creatable files such as thumbnails, e.g. ~/.cache/kisetsu
	State  string // logs and other runtime state, e.g. ~/.local/state/kisetsu
}

// DefaultPaths resolves Paths from the current environment.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return ResolvePaths(os.Getenv, home)
}

// ResolvePaths resolves Paths using getenv to read XDG variables and
// home as the fallback base directory. Relative XDG values are ignored,
// as the specification requires.
func ResolvePaths(getenv func(string) string, home string) (Paths, error) {
	if !filepath.IsAbs(home) {
		return Paths{}, errors.New("home directory must be an absolute path")
	}

	base := func(env, fallback string) string {
		if dir := getenv(env); filepath.IsAbs(dir) {
			return filepath.Join(dir, appDirName)
		}
		return filepath.Join(home, fallback, appDirName)
	}

	return Paths{
		Config: base("XDG_CONFIG_HOME", ".config"),
		Data:   base("XDG_DATA_HOME", filepath.Join(".local", "share")),
		Cache:  base("XDG_CACHE_HOME", ".cache"),
		State:  base("XDG_STATE_HOME", filepath.Join(".local", "state")),
	}, nil
}

// Ensure creates any missing directories. Existing directories and
// their contents are left untouched.
func (p Paths) Ensure() error {
	for _, dir := range []string{p.Config, p.Data, p.Cache, p.State} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}
