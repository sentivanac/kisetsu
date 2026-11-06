package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// fakeWallpaper records the path instead of changing the desktop.
type fakeWallpaper struct {
	path    string
	method  wallpaper.Method
	err     error
	variety bool
}

func (f *fakeWallpaper) VarietyRunning() bool { return f.variety }

func (f *fakeWallpaper) ViaVariety() bool { return f.variety }

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

// creditText is the default text settings with the credit set.
func creditText(on bool, corner string) WallpaperText {
	t := wallpaperText(onscreen.DefaultSettings)
	t.Credit, t.CreditCorner = on, corner
	return t
}

// composed reports whether path is an image with text drawn by Kisetsu.
func composed(app *App, path string) bool {
	return filepath.Dir(path) == app.screen.Dir
}

func TestWallpaperTextIsOffByDefault(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	state, err := app.GetRotation()
	if err != nil {
		t.Fatal(err)
	}
	if state.Text != (creditText(false, "bottom-left")) {
		t.Errorf("text = %+v", state.Text)
	}
	if _, err := app.SetWallpaper(ids[0]); err != nil {
		t.Fatal(err)
	}
	if fake := app.wallpaper.(*fakeWallpaper); composed(app, fake.path) {
		t.Errorf("composed image %q set while text is off", fake.path)
	}
}

func TestSetWallpaperDrawsTheCredit(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	if _, err := app.SaveWallpaperText(creditText(true, "top-right")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetWallpaper(ids[0]); err != nil {
		t.Fatal(err)
	}
	if fake := app.wallpaper.(*fakeWallpaper); !composed(app, fake.path) {
		t.Errorf("set %q, want a composed image", fake.path)
	}
	// History keeps the library image.
	if st, err := app.rotation.History.State(app.ctx); err != nil || st.Current != ids[0] {
		t.Errorf("history current = %d, %v", st.Current, err)
	}
}

func TestSaveWallpaperTextShowsTheCurrentWallpaperAgain(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	fake := app.wallpaper.(*fakeWallpaper)
	if _, err := app.SetWallpaper(ids[0]); err != nil {
		t.Fatal(err)
	}
	original := fake.path

	state, err := app.SaveWallpaperText(creditText(true, "top-right"))
	if err != nil {
		t.Fatal(err)
	}
	if state.Text != (creditText(true, "top-right")) {
		t.Errorf("text = %+v", state.Text)
	}
	if !composed(app, fake.path) {
		t.Errorf("after turning on: set %q, want a composed image", fake.path)
	}

	if _, err := app.SaveWallpaperText(creditText(false, "top-right")); err != nil {
		t.Fatal(err)
	}
	if fake.path != original {
		t.Errorf("after turning off: set %q, want the original %q", fake.path, original)
	}

	// Saving the same settings again changes nothing.
	fake.path = ""
	if _, err := app.SaveWallpaperText(creditText(false, "top-right")); err != nil {
		t.Fatal(err)
	}
	if fake.path != "" {
		t.Errorf("unchanged settings set %q", fake.path)
	}
}

func TestSaveWallpaperTextWithoutAWallpaperYet(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	if _, err := app.SaveWallpaperText(creditText(true, "bottom-right")); err != nil {
		t.Fatal(err)
	}
	if fake := app.wallpaper.(*fakeWallpaper); fake.path != "" {
		t.Errorf("set %q although Kisetsu never set a wallpaper", fake.path)
	}
}

func TestSaveWallpaperTextLeavesVarietyAlone(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	fake := app.wallpaper.(*fakeWallpaper)
	fake.variety = true
	if _, err := app.SetWallpaper(ids[0]); err != nil {
		t.Fatal(err)
	}
	if composed(app, fake.path) {
		t.Errorf("composed image %q handed to Variety", fake.path)
	}
	fake.path = ""
	if _, err := app.SaveWallpaperText(creditText(true, "bottom-right")); err != nil {
		t.Fatal(err)
	}
	if fake.path != "" {
		t.Errorf("set %q while Variety runs", fake.path)
	}
}

func TestSaveWallpaperTextRejectsAnUnknownCorner(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	_, err := app.SaveWallpaperText(creditText(true, "middle"))
	if apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("err = %v", err)
	}
}

func TestProviderName(t *testing.T) {
	if got := providerName("wikimedia-commons"); got != "Wikimedia Commons" {
		t.Errorf("wikimedia-commons: %q", got)
	}
	if got := providerName("wallhaven"); got != "Wallhaven" {
		t.Errorf("wallhaven: %q", got)
	}
	if got := providerName("gone"); got != "gone" {
		t.Errorf("unknown source: %q", got)
	}
}

func TestClockStartsTheAgent(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ag := app.agent.(*fakeAgent)
	clock := wallpaperText(onscreen.DefaultSettings)
	clock.Clock = true

	state, err := app.SaveWallpaperText(clock)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Text.Clock || !ag.autostart || !ag.running {
		t.Errorf("clock on: text %+v, agent %+v", state.Text, ag)
	}

	clock.Clock = false
	if _, err := app.SaveWallpaperText(clock); err != nil {
		t.Fatal(err)
	}
	if ag.autostart {
		t.Errorf("clock off with nothing else to do: agent %+v", ag)
	}
}

func TestClockStaysOffWhenTheAgentCannotRun(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.agent.(*fakeAgent).enableErr = autostart.ErrTemporaryExecutable
	clock := wallpaperText(onscreen.DefaultSettings)
	clock.Clock = true

	_, err := app.SaveWallpaperText(clock)
	if !strings.Contains(apperr.ToPublic(err).Message, "temporary build") {
		t.Errorf("err = %v", err)
	}
	if cfg, _ := onscreen.LoadSettings(app.ctx, app.settings); cfg.Clock {
		t.Error("clock saved as on although the agent cannot run")
	}
}

func TestSaveWallpaperTextRejectsAnUnknownClockSize(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	bad := wallpaperText(onscreen.DefaultSettings)
	bad.ClockSize = "huge"
	if _, err := app.SaveWallpaperText(bad); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("err = %v", err)
	}
}
