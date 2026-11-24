//go:build bindings

package main

import (
	"errors"
	"os"
	"path/filepath"
)

// isolateBindings points Kisetsu at an empty temporary directory. To
// generate the frontend bindings, Wails builds the program with the
// "bindings" tag and runs it (in every wails build, dev and generate);
// without this, that run would migrate the developer's own database.
func isolateBindings() (cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "kisetsu-bindings-")
	if err != nil {
		return nil, err
	}
	var errs []error
	for name, sub := range map[string]string{
		"XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "data",
		"XDG_CACHE_HOME": "cache", "XDG_STATE_HOME": "state",
		"APPDATA": "config", "LOCALAPPDATA": "data", // Windows
	} {
		errs = append(errs, os.Setenv(name, filepath.Join(dir, sub)))
	}
	cleanup = func() { os.RemoveAll(dir) }
	if err := errors.Join(errs...); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}
