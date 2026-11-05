//go:build !windows

package config

import (
	"fmt"
	"os"
)

// DefaultPaths resolves Paths from the current environment.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return ResolvePaths(os.Getenv, home)
}
