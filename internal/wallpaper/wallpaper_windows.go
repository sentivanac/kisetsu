//go:build windows

package wallpaper

import (
	"context"
	"fmt"
	"path/filepath"
)

// Setter sets wallpapers on Windows.
type Setter struct {
	// Direct is accepted for the same calls as on Linux; there is no
	// Variety on Windows.
	Direct bool
}

// New returns a Setter.
func New() *Setter {
	return &Setter{}
}

// Set makes the image at path the wallpaper. path must be absolute.
//
// Not implemented yet (0.3.0, step 3).
func (s *Setter) Set(ctx context.Context, path string) (Method, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("wallpaper path %q is not absolute", path)
	}
	return "", fmt.Errorf("%w: Windows", ErrUnsupportedDesktop)
}

// VarietyRunning is always false: Variety does not run on Windows.
func (s *Setter) VarietyRunning() bool { return false }
