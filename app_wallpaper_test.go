package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// fakeWallpaper records the path instead of changing the desktop.
type fakeWallpaper struct {
	path   string
	method wallpaper.Method
	err    error
}

func (f *fakeWallpaper) Set(ctx context.Context, path string) (wallpaper.Method, error) {
	f.path = path
	if f.method == "" {
		f.method = wallpaper.MATE
	}
	return f.method, f.err
}

func TestSetWallpaperUsesTheLibraryOriginal(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	fake := app.wallpaper.(*fakeWallpaper)

	result, err := app.SetWallpaper(ids[0])
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := app.library.Get(app.ctx, ids[0])
	want := filepath.Join(filepath.Dir(app.dbPath), entry.FilePath)
	if fake.path != want || !filepath.IsAbs(fake.path) {
		t.Errorf("setter got %q, want %q", fake.path, want)
	}
	if result.Method != "mate" || result.Message == "" {
		t.Errorf("result = %+v", result)
	}

	st, err := app.rotation.History.State(app.ctx)
	if err != nil || st.Current != ids[0] {
		t.Errorf("history current = %d, %v; want %d", st.Current, err, ids[0])
	}

	fake.method = wallpaper.Variety
	result, _ = app.SetWallpaper(ids[0])
	if !strings.Contains(result.Message, "Variety") {
		t.Errorf("Variety message = %q", result.Message)
	}
}

func TestSetWallpaperErrors(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	fake := app.wallpaper.(*fakeWallpaper)

	if _, err := app.SetWallpaper(999); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("missing entry: %v", err)
	}

	fake.err = wallpaper.ErrUnsupportedDesktop
	if _, err := app.SetWallpaper(ids[0]); apperr.ToPublic(err).Code != Unsupported {
		t.Errorf("unsupported desktop: %v", err)
	}

	fake.err = errors.New("gsettings: exit status 1")
	_, err := app.SetWallpaper(ids[0])
	if pub := apperr.ToPublic(err); pub.Code != apperr.Unavailable || strings.Contains(pub.Message, "gsettings") {
		t.Errorf("failure: %+v", pub)
	}

	// A file removed outside Kisetsu is reported, and the setter is not called.
	fake.err, fake.path = nil, ""
	entry, _ := app.library.Get(app.ctx, ids[0])
	os.Remove(filepath.Join(filepath.Dir(app.dbPath), entry.FilePath))
	if _, err := app.SetWallpaper(ids[0]); !strings.Contains(apperr.ToPublic(err).Message, "missing") {
		t.Errorf("missing file: %v", err)
	}
	if fake.path != "" {
		t.Errorf("setter called with %q for a missing file", fake.path)
	}
}
