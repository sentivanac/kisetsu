// Package onscreen puts a library image on the desktop: as it is, or with
// text drawn on it (ADR 0047) when the user turned that on. The window,
// rotation and the agent all show images through it; the agent also
// shows the current image again when the clock's text changes.
//
// Text is never a reason for the wallpaper not to change: when the image
// cannot be composed, the original is set and the problem is logged.
package onscreen

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

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

// Every composed image gets a new name, "wallpaper-<nanoseconds>.jpg":
// desktops keep pictures by path, and GNOME showed an earlier picture
// (with an old time) when the same names were written again. The newest
// keepComposed are kept, since a desktop may still read the previous one.
const (
	composedPrefix = "wallpaper-"
	composedSuffix = ".jpg"
	keepComposed   = 2
)

const (
	// creditShare is the credit's font size as a share of the screen
	// height (20 px on a 1080 px screen).
	creditShare = 0.019
	// dateShare is the date's font size as a share of the time's.
	dateShare = 0.32
	// minFontSize keeps small screens legible.
	minFontSize = 12
)

// Service shows library images.
type Service struct {
	Settings *settings.Store
	Setter   Setter
	// Dir holds the composed images (in the cache).
	Dir string
	// ProviderName returns a source's display name ("Wikimedia Commons").
	ProviderName func(id string) string
	Log          *slog.Logger
	Now          func() time.Time // nil means time.Now
	// KeepBase keeps the base of the current image while the clock or the
	// text file is on, so a redraw only draws the text (ADR 0047). The
	// agent sets it; the window, which does not redraw, does not.
	KeepBase bool
	// ReleaseMemory returns memory to the system after each composed
	// image. The agent sets it: it runs all session, and between redraws
	// it needs only the kept base (ADR 0052).
	ReleaseMemory bool

	mu      sync.Mutex
	base    *image.RGBA
	baseKey baseKey
	// shownLive is the live text (clock and text file) last drawn, or
	// tried; RedrawDue compares it with the current one.
	shownLive string
}

// baseKey identifies a base: the same file, unchanged, for the same
// screen, with the same effects.
type baseKey struct {
	path          string
	modTime       int64
	width, height int
	effects       overlay.Effects
}

// Show sets the image of e, whose file is at path, as the wallpaper.
func (s *Service) Show(ctx context.Context, e library.Entry, path string) (wallpaper.Method, error) {
	return s.show(ctx, e, path, false)
}

// ShowWithoutLive sets the image as Show does, but without what the agent
// keeps up to date (the clock and the text file): the agent shows it when
// it quits, so no stale time or text stays on the desktop. The author,
// license and effects stay if they are turned on.
func (s *Service) ShowWithoutLive(ctx context.Context, e library.Entry, path string) (wallpaper.Method, error) {
	return s.show(ctx, e, path, true)
}

func (s *Service) show(ctx context.Context, e library.Entry, path string, withoutLive bool) (wallpaper.Method, error) {
	cfg, err := LoadSettings(ctx, s.Settings)
	if err != nil {
		s.Log.Warn("wallpaper text settings unreadable; showing the image without text", "err", err)
		cfg = DefaultSettings
	}
	if withoutLive {
		cfg.Clock, cfg.TextFile = false, false
	}
	if !cfg.Any() || s.Setter.ViaVariety() {
		return s.Setter.Set(ctx, path)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	composed, err := s.compose(ctx, e, path, cfg)
	if s.ReleaseMemory {
		// The decoded file, the scaled copies and the drawn image are
		// garbage now; without this, Go keeps their memory for minutes.
		debug.FreeOSMemory()
	}
	if err != nil {
		s.Log.Warn("text not drawn on the wallpaper; showing the image without it", "entry", e.ID, "err", err)
		return s.Setter.Set(ctx, path)
	}
	return s.Setter.Set(ctx, composed)
}

// RedrawDue reports whether the clock or the text file is on and what
// they show differs from what was last drawn: a new minute, a new day, a
// changed format or a changed file. It is cheap (the file is at most a
// few KB), so the agent asks every few seconds; asking rather than timing
// keeps the clock right after a suspend or a change of the system clock.
func (s *Service) RedrawDue(ctx context.Context) bool {
	cfg, err := LoadSettings(ctx, s.Settings)
	if err != nil || !cfg.Live() || s.Setter.ViaVariety() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live(cfg).key != s.shownLive
}

// SkipRedraw records the current live text as shown without drawing it,
// when there is no wallpaper to draw it on, so RedrawDue waits for a
// change.
func (s *Service) SkipRedraw(ctx context.Context) {
	cfg, err := LoadSettings(ctx, s.Settings)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shownLive = s.live(cfg).key
}

// liveText is what the agent keeps up to date.
type liveText struct {
	clock []string // the time and the date; nil when the clock is off
	file  []string // the text file's lines, or why they are not shown
	// fileErr is a problem reading the file other than the expected ones
	// (it is logged when drawn).
	fileErr error
	key     string // compared by RedrawDue
}

func (s *Service) live(cfg Settings) liveText {
	var t liveText
	if cfg.Clock {
		t.clock = clockLines(s.now(), cfg)
	}
	if cfg.TextFile {
		lines, err := readTextFile(cfg.TextFilePath)
		var p *textFileProblem
		switch {
		case errors.As(err, &p):
			lines = []string{p.msg}
		case err != nil:
			lines, t.fileErr = []string{"The text file could not be read"}, err
		}
		t.file = lines
	}
	t.key = strings.Join(t.clock, "\n") + "\x00" + strings.Join(t.file, "\n")
	return t
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// clockLines returns the time and, if wanted, the date, such as "14:07"
// and "Thursday, 5 November". The names are English, like the rest of
// Kisetsu.
func clockLines(t time.Time, cfg Settings) []string {
	layout := "15:04"
	if cfg.Clock12h {
		layout = "3:04 PM"
	}
	lines := []string{t.Format(layout)}
	if cfg.ClockDate {
		lines = append(lines, t.Format("Monday, 2 January"))
	}
	return lines
}

// compose draws the text on the image and returns the composed file. The
// caller holds s.mu.
func (s *Service) compose(ctx context.Context, e library.Entry, path string, cfg Settings) (string, error) {
	screen, err := s.Settings.Display(ctx)
	if err != nil {
		return "", err
	}
	var blocks []overlay.Block
	live := s.live(cfg)
	// Recorded before drawing, so a failure is not retried every few
	// seconds, only when the text changes.
	s.shownLive = live.key
	if cfg.Clock {
		share, _ := cfg.ClockSize.clockShare()
		size := max(minFontSize, int(math.Round(float64(screen.Height)*share)))
		blocks = append(blocks, overlay.Block{Lines: live.clock[:1], Corner: cfg.ClockCorner, Size: size})
		if len(live.clock) > 1 {
			blocks = append(blocks, overlay.Block{
				Lines: live.clock[1:], Corner: cfg.ClockCorner,
				Size: max(minFontSize, int(math.Round(float64(size)*dateShare))),
			})
		}
	}
	if cfg.TextFile {
		if live.fileErr != nil {
			s.Log.Warn("text file not read", "path", cfg.TextFilePath, "err", live.fileErr)
		}
		textShare, _ := cfg.TextFileSize.textFileShare()
		lines := make([]string, len(live.file))
		for i, l := range live.file {
			lines[i] = drawableText(l)
		}
		blocks = append(blocks, overlay.Block{
			Lines:        lines,
			Corner:       cfg.TextFileCorner,
			Size:         max(minFontSize, int(math.Round(float64(screen.Height)*textShare))),
			Preformatted: true,
		})
	}
	if cfg.Credit {
		blocks = append(blocks, overlay.Block{
			Lines:  []string{s.credit(e.Image)},
			Corner: cfg.CreditCorner,
			Size:   max(minFontSize, int(math.Round(float64(screen.Height)*creditShare))),
		})
	}
	base, err := s.baseOf(path, screen.Width, screen.Height, cfg.effects(), cfg.Live() && s.KeepBase)
	if err != nil {
		return "", err
	}
	out, err := overlay.Draw(base, blocks)
	if err != nil {
		return "", err
	}
	return s.write(out)
}

// drawableText replaces the characters the font cannot draw with "?", so
// none shows as an empty box.
func drawableText(s string) string {
	if overlay.Drawable(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if overlay.Drawable(string(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
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

// baseOf returns the base of the image at path for the screen, with the
// effects applied. With keep, it is kept for the next minute; without, any
// kept base is dropped (it takes 33 MB for a 4K screen).
func (s *Service) baseOf(path string, w, h int, effects overlay.Effects, keep bool) (*image.RGBA, error) {
	key := baseKey{path: path, modTime: modTime(path), width: w, height: h, effects: effects}
	if keep && s.base != nil && s.baseKey == key {
		return s.base, nil
	}
	s.base = nil
	img, err := decode(path)
	if err != nil {
		return nil, err
	}
	base, err := overlay.Base(img, w, h)
	if err != nil {
		return nil, err
	}
	// Effects change the base in place; Base made a new image, so the
	// decoded file is not changed.
	effects.Apply(base)
	if keep {
		s.base, s.baseKey = base, key
	}
	return base, nil
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

// write stores img under a new name and returns its path, removing older
// composed images. The file appears in one step, so a desktop never reads
// half of it.
func (s *Service) write(img image.Image) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
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
	// The name sorts by time; the window and the agent both write here, so
	// a taken name is skipped.
	n := time.Now().UnixNano()
	target := ""
	for ; ; n++ {
		target = filepath.Join(s.Dir, fmt.Sprintf("%s%019d%s", composedPrefix, n, composedSuffix))
		if _, err := os.Lstat(target); errors.Is(err, fs.ErrNotExist) {
			break
		}
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", err
	}
	s.prune()
	return target, nil
}

// isComposed reports whether name is a composed image written by write:
// the prefix, 19 digits and the suffix. Only these sort by time.
func isComposed(name string) bool {
	digits, ok := strings.CutPrefix(name, composedPrefix)
	if !ok {
		return false
	}
	digits, ok = strings.CutSuffix(digits, composedSuffix)
	if !ok || len(digits) != 19 {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// prune removes all but the newest keepComposed composed images. Other
// files in the directory are left alone. Problems are only logged.
func (s *Service) prune() {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		s.Log.Warn("old wallpaper copies not removed", "err", err)
		return
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && isComposed(e.Name()) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	for _, n := range names[:max(0, len(names)-keepComposed)] {
		if err := os.Remove(filepath.Join(s.Dir, n)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.Log.Warn("old wallpaper copy not removed", "file", n, "err", err)
		}
	}
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
