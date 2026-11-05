//go:build windows

package config

import (
	"errors"
	"os"
	"path/filepath"
)

// DefaultPaths returns the directories under %LocalAppData%.
func DefaultPaths() (Paths, error) {
	local := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(local) {
		return Paths{}, errors.New("%LOCALAPPDATA% is not set to an absolute path")
	}
	return WindowsPaths(local), nil
}
