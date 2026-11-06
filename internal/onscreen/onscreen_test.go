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
	"testing"

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
	if err := SaveSettings(context.Background(), e.store, Settings{Credit: true, CreditCorner: overlay.BottomRight}); err != nil {
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
	if err := SaveSettings(ctx, e.store, Settings{Credit: true, CreditCorner: "middle"}); err == nil {
		t.Error("an unknown corner was saved")
	}
	want := Settings{Credit: true, CreditCorner: overlay.TopLeft}
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
