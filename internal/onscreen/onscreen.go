// Package onscreen puts a library image on the desktop: as it is, or with
// text drawn on it (ADR 0047) when the user turned that on. The window,
// rotation and the agent all show images through it.
//
// Text is never a reason for the wallpaper not to change: when the image
// cannot be composed, the original is set and the problem is logged.
package onscreen

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/overlay"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// Setter sets the wallpaper; *wallpaper.Setter.
type Setter interface {
	Set(ctx context.Context, path string) (wallpaper.Method, error)
	// ViaVariety reports whether Set would hand the image to Variety.
	ViaVariety() bool
}

// composedNames are the two files the composed image alternates between:
// GNOME and MATE do not reload a picture whose path did not change.
var composedNames = [2]string{"wallpaper-a.jpg", "wallpaper-b.jpg"}

// creditShare is the credit's font size as a share of the screen height
// (20 px on a 1080 px screen).
const creditShare = 0.019

// Service shows library images.
type Service struct {
	Settings *settings.Store
	Setter   Setter
	// Dir holds the composed images (in the cache).
	Dir string
	// ProviderName returns a source's display name ("Wikimedia Commons").
	ProviderName func(id string) string
	Log          *slog.Logger
}

// Show sets the image of e, whose file is at path, as the wallpaper.
func (s *Service) Show(ctx context.Context, e library.Entry, path string) (wallpaper.Method, error) {
	cfg, err := LoadSettings(ctx, s.Settings)
	if err != nil {
		s.Log.Warn("wallpaper text settings unreadable; showing the image without text", "err", err)
		cfg = DefaultSettings
	}
	if !cfg.Any() || s.Setter.ViaVariety() {
		return s.Setter.Set(ctx, path)
	}
	composed, err := s.compose(ctx, e, path, cfg)
	if err != nil {
		s.Log.Warn("text not drawn on the wallpaper; showing the image without it", "entry", e.ID, "err", err)
		return s.Setter.Set(ctx, path)
	}
	return s.Setter.Set(ctx, composed)
}

// compose draws the text on the image and returns the composed file.
func (s *Service) compose(ctx context.Context, e library.Entry, path string, cfg Settings) (string, error) {
	screen, err := s.Settings.Display(ctx)
	if err != nil {
		return "", err
	}
	img, err := decode(path)
	if err != nil {
		return "", err
	}
	base, err := overlay.Base(img, screen.Width, screen.Height)
	if err != nil {
		return "", err
	}
	var blocks []overlay.Block
	if cfg.Credit {
		blocks = append(blocks, overlay.Block{
			Lines:  []string{s.credit(e.Image)},
			Corner: cfg.CreditCorner,
			Size:   max(12, int(math.Round(float64(screen.Height)*creditShare))),
		})
	}
	out, err := overlay.Draw(base, blocks)
	if err != nil {
		return "", err
	}
	return s.write(out)
}

// credit is the author, the license and the source, such as
// "Ana Ćirić · CC BY-SA 4.0 · Wikimedia Commons". What is unknown, or
// cannot be drawn with the font, is left out; the source is always known.
func (s *Service) credit(img images.Image) string {
	var parts []string
	if name := strings.TrimSpace(img.Author.Name); name != "" && overlay.Drawable(name) {
		parts = append(parts, name)
	}
	if lic := strings.TrimSpace(img.License.Name); img.License.Status == images.LicenseKnown && lic != "" && overlay.Drawable(lic) {
		parts = append(parts, lic)
	}
	source := img.Provider
	if s.ProviderName != nil {
		source = s.ProviderName(img.Provider)
	}
	parts = append(parts, source)
	return strings.Join(parts, " · ")
}

// decode reads an image file, refusing images too large to decode safely.
func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	if int64(cfg.Width)*int64(cfg.Height) > imaging.MaxPixels {
		return nil, fmt.Errorf("%w: %dx%d", imaging.ErrTooLarge, cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}
	return img, nil
}

// write stores img in the older of the two composed files and returns its
// path. The window and the agent may both compose, so the choice is made
// from the files, not remembered; the file is replaced in one step, so a
// desktop never reads half of it.
func (s *Service) write(img image.Image) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
	}
	target := filepath.Join(s.Dir, composedNames[0])
	if a, b := modTime(target), modTime(filepath.Join(s.Dir, composedNames[1])); b < a {
		target = filepath.Join(s.Dir, composedNames[1])
	}

	tmp, err := os.CreateTemp(s.Dir, ".wallpaper-*.jpg")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	if err := overlay.Encode(tmp, img); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	return target, nil
}

// modTime returns the file's modification time in nanoseconds, or 0 if it
// does not exist.
func modTime(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}
