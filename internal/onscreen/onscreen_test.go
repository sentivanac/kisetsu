package onscreen

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/overlay"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// fakeSetter records paths instead of changing the desktop.
type fakeSetter struct {
	paths   []string
	variety bool
}

func (f *fakeSetter) Set(ctx context.Context, path string) (wallpaper.Method, error) {
	f.paths = append(f.paths, path)
	if f.variety {
		return wallpaper.Variety, nil
	}
	return wallpaper.MATE, nil
}

func (f *fakeSetter) ViaVariety() bool { return f.variety }

type env struct {
	s      *Service
	store  *settings.Store
	setter *fakeSetter
	image  string // a JPEG in the library
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := settings.NewStore(db)
	if err := store.SaveDisplay(ctx, settings.DisplaySize{Width: 320, Height: 180}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	writeJPEG(t, path, 640, 400)

	e := &env{store: store, setter: &fakeSetter{}, image: path}
	e.s = &Service{
		Settings: store, Setter: e.setter, Dir: filepath.Join(dir, "overlay"),
		ProviderName: func(id string) string {
			return map[string]string{"commons": "Wikimedia Commons", "wallhaven": "Wallhaven"}[id]
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return e
}

func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func commonsEntry() library.Entry {
	return library.Entry{ID: 7, Image: images.Image{
		Provider: "commons",
		Author:   images.Author{Name: "Ana Ćirić"},
		License:  images.License{Status: images.LicenseKnown, Name: "CC BY-SA 4.0"},
	}}
}

func (e *env) turnOn(t *testing.T) {
	t.Helper()
	e.save(t, func(s *Settings) { s.Credit, s.CreditCorner = true, overlay.BottomRight })
}

// save stores the defaults changed by change.
func (e *env) save(t *testing.T, change func(*Settings)) {
	t.Helper()
	s := DefaultSettings
	change(&s)
	if err := SaveSettings(context.Background(), e.store, s); err != nil {
		t.Fatal(err)
	}
}

func TestShowsTheOriginalByDefault(t *testing.T) {
	e := newEnv(t)
	method, err := e.s.Show(context.Background(), commonsEntry(), e.image)
	if err != nil || method != wallpaper.MATE {
		t.Fatalf("method %q, err %v", method, err)
	}
	if len(e.setter.paths) != 1 || e.setter.paths[0] != e.image {
		t.Errorf("set %q, want the original", e.setter.paths)
	}
	if _, err := os.Stat(e.s.Dir); !os.IsNotExist(err) {
		t.Errorf("composed directory created while off: %v", err)
	}
}

func TestShowsAComposedImageWithTheCredit(t *testing.T) {
	e := newEnv(t)
	e.turnOn(t)
	if _, err := e.s.Show(context.Background(), commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	got := e.setter.paths[0]
	if filepath.Dir(got) != e.s.Dir {
		t.Fatalf("set %q, want a file in %q", got, e.s.Dir)
	}
	f, err := os.Open(got)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 320 || cfg.Height != 180 {
		t.Errorf("composed image is %dx%d, want the screen size 320x180", cfg.Width, cfg.Height)
	}
	if fi, err := os.Stat(e.image); err != nil || fi.Size() == 0 {
		t.Errorf("the original changed: %v", err)
	}
}

func TestAlternatesBetweenTwoFiles(t *testing.T) {
	e := newEnv(t)
	e.turnOn(t)
	for range 4 {
		if _, err := e.s.Show(context.Background(), commonsEntry(), e.image); err != nil {
			t.Fatal(err)
		}
	}
	p := e.setter.paths
	if p[0] == p[1] || p[1] == p[2] || p[0] != p[2] || p[1] != p[3] {
		t.Errorf("paths %q do not alternate", p)
	}
	entries, _ := os.ReadDir(e.s.Dir)
	if len(entries) != 2 {
		t.Errorf("%d files in %s, want 2 (no temporary files left)", len(entries), e.s.Dir)
	}
}

func TestLeavesTextToVariety(t *testing.T) {
	e := newEnv(t)
	e.turnOn(t)
	e.setter.variety = true
	if _, err := e.s.Show(context.Background(), commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if e.setter.paths[0] != e.image {
		t.Errorf("set %q through Variety, want the original", e.setter.paths[0])
	}
}

func TestShowsTheOriginalWhenComposingFails(t *testing.T) {
	e := newEnv(t)
	e.turnOn(t)
	broken := filepath.Join(t.TempDir(), "broken.jpg")
	if err := os.WriteFile(broken, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Show(context.Background(), commonsEntry(), broken); err != nil {
		t.Fatal(err)
	}
	if e.setter.paths[0] != broken {
		t.Errorf("set %q, want the original", e.setter.paths[0])
	}
}

func TestCredit(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct {
		name string
		img  images.Image
		want string
	}{
		{"all known", commonsEntry().Image, "Ana Ćirić · CC BY-SA 4.0 · Wikimedia Commons"},
		{"unknown author and license", images.Image{
			Provider: "wallhaven", License: images.License{Status: images.LicenseUnknown},
		}, "Wallhaven"},
		{"name the font cannot draw", images.Image{
			Provider: "commons", Author: images.Author{Name: "山田太郎"},
			License: images.License{Status: images.LicenseKnown, Name: "CC0"},
		}, "CC0 · Wikimedia Commons"},
		{"license name with unknown status", images.Image{
			Provider: "commons", Author: images.Author{Name: " Ana "},
			License: images.License{Status: images.LicenseUnknown, Name: "maybe"},
		}, "Ana · Wikimedia Commons"},
	} {
		if got := e.s.credit(tc.img); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSettings(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	got, err := LoadSettings(ctx, e.store)
	if err != nil || got != DefaultSettings || got.Any() {
		t.Fatalf("defaults: %+v, %v", got, err)
	}
	for _, bad := range []func(*Settings){
		func(s *Settings) { s.CreditCorner = "middle" },
		func(s *Settings) { s.ClockCorner = "" },
		func(s *Settings) { s.ClockSize = "huge" },
	} {
		s := DefaultSettings
		bad(&s)
		if err := SaveSettings(ctx, e.store, s); err == nil {
			t.Errorf("%+v was saved", s)
		}
	}
	want := DefaultSettings
	want.Credit, want.CreditCorner, want.Clock, want.Clock12h = true, overlay.TopLeft, true, true
	if err := SaveSettings(ctx, e.store, want); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadSettings(ctx, e.store); err != nil || got != want {
		t.Errorf("loaded %+v, %v; want %+v", got, err, want)
	}
	// A stored value from a newer or broken version gives the defaults.
	if err := e.store.Save(ctx, settingsKey, map[string]any{"credit": true, "creditCorner": "nowhere"}); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadSettings(ctx, e.store); err != nil || got != DefaultSettings {
		t.Errorf("unusable stored value: %+v, %v", got, err)
	}
}

func TestSettingsFromBeforeTheClock(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	// As saved by the first version with the credit only.
	if err := e.store.Save(ctx, settingsKey, map[string]any{"credit": true, "creditCorner": "top-left"}); err != nil {
		t.Fatal(err)
	}
	want := DefaultSettings
	want.Credit, want.CreditCorner = true, overlay.TopLeft
	if got, err := LoadSettings(ctx, e.store); err != nil || got != want {
		t.Errorf("loaded %+v, %v; want %+v", got, err, want)
	}
}

func TestClockLines(t *testing.T) {
	at := time.Date(2026, 11, 5, 14, 7, 30, 0, time.Local)
	for _, tc := range []struct {
		change func(*Settings)
		want   []string
	}{
		{func(s *Settings) {}, []string{"14:07", "Thursday, 5 November"}},
		{func(s *Settings) { s.Clock12h = true }, []string{"2:07 PM", "Thursday, 5 November"}},
		{func(s *Settings) { s.ClockDate = false }, []string{"14:07"}},
	} {
		s := DefaultSettings
		tc.change(&s)
		if got := clockLines(at, s); !slices.Equal(got, tc.want) {
			t.Errorf("%+v: %q, want %q", s, got, tc.want)
		}
	}
}

func TestClockDue(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 5, 14, 7, 30, 0, time.Local)
	e.s.Now = func() time.Time { return now }

	if e.s.ClockDue(ctx) {
		t.Error("due while the clock is off")
	}
	e.save(t, func(s *Settings) { s.Clock = true })
	if !e.s.ClockDue(ctx) {
		t.Error("not due before the first drawing")
	}
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if e.s.ClockDue(ctx) {
		t.Error("due again within the same minute")
	}
	now = now.Add(20 * time.Second) // 14:07:50
	if e.s.ClockDue(ctx) {
		t.Error("due within the same minute")
	}
	now = now.Add(15 * time.Second) // 14:08:05
	if !e.s.ClockDue(ctx) {
		t.Error("not due in a new minute")
	}
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	e.save(t, func(s *Settings) { s.Clock, s.Clock12h = true, true })
	if !e.s.ClockDue(ctx) {
		t.Error("not due after the format changed")
	}
	now = now.Add(time.Minute)
	e.s.SkipClock(ctx)
	if e.s.ClockDue(ctx) {
		t.Error("due after SkipClock in the same minute")
	}
	e.setter.variety = true
	now = now.Add(time.Minute)
	if e.s.ClockDue(ctx) {
		t.Error("due while Variety shows the images")
	}
}

func TestClockKeepsTheBase(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.save(t, func(s *Settings) { s.Clock = true })
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if e.s.base != nil {
		t.Fatal("base kept without KeepBase (the window)")
	}
	e.s.KeepBase = true
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	first := e.s.base
	if first == nil {
		t.Fatal("base not kept while the clock is on")
	}
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if e.s.base != first {
		t.Error("base computed again for the same image")
	}

	// Another image gets its own base.
	other := filepath.Join(t.TempDir(), "other.jpg")
	writeJPEG(t, other, 300, 300)
	if _, err := e.s.Show(ctx, commonsEntry(), other); err != nil {
		t.Fatal(err)
	}
	if e.s.base == first || e.s.baseKey.path != other {
		t.Error("base of the previous image used for another one")
	}

	// Without the clock nothing is kept.
	e.save(t, func(s *Settings) { s.Credit = true })
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if e.s.base != nil {
		t.Error("base kept while the clock is off")
	}
}

func TestClockAndCreditDrawnTogether(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.s.Now = func() time.Time { return time.Date(2026, 11, 5, 14, 7, 0, 0, time.Local) }
	e.save(t, func(s *Settings) { s.Clock, s.Credit, s.ClockCorner = true, true, s.CreditCorner })
	if _, err := e.s.Show(ctx, commonsEntry(), e.image); err != nil {
		t.Fatal(err)
	}
	if got := e.setter.paths[0]; filepath.Dir(got) != e.s.Dir {
		t.Errorf("set %q, want a composed image", got)
	}
}
