package rotation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// fakeFiles maps stored paths to absolute ones; missing paths fail.
type fakeFiles struct{ missing map[string]bool }

func (f fakeFiles) OriginalPath(rel string) (string, error) {
	if f.missing[rel] {
		return "", errors.New("file missing")
	}
	return "/library/" + rel, nil
}

// fakeSetter records paths instead of changing the desktop.
type fakeSetter struct{ paths []string }

func (f *fakeSetter) Set(ctx context.Context, path string) (wallpaper.Method, error) {
	f.paths = append(f.paths, path)
	return wallpaper.MATE, nil
}

type env struct {
	r      *Rotator
	lib    *library.Store
	set    *settings.Store
	setter *fakeSetter
	files  fakeFiles
}

func newEnv(t *testing.T, images int) (*env, []int64) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	e := &env{lib: library.NewStore(db), set: settings.NewStore(db), setter: &fakeSetter{},
		files: fakeFiles{missing: map[string]bool{}}}
	var ids []int64
	for n := range images {
		id, err := e.lib.Add(context.Background(), entry(n))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	e.r = &Rotator{Library: e.lib, Settings: e.set, History: NewHistory(db), Files: e.files,
		Setter: e.setter, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Rand: seeded()}
	return e, ids
}

func entry(n int) library.Entry {
	return library.Entry{
		Image: images.Image{
			Provider: "wallhaven", ProviderID: fmt.Sprint(n), PageURL: "https://x/" + fmt.Sprint(n),
			DownloadURL: "https://x/d", Width: 1920, Height: 1080, MIMEType: "image/jpeg", FileSize: 1,
			License: images.License{Status: images.LicenseUnknown},
		},
		FilePath: fmt.Sprintf("images/wallhaven/%d.jpg", n),
		SHA256:   fmt.Sprintf("%064d", n),
	}
}

func TestNextShowsEveryImageBeforeRepeating(t *testing.T) {
	e, ids := newEnv(t, 4)
	ctx := context.Background()

	seen := map[int64]int{}
	for range 4 {
		res, err := e.r.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		seen[res.ImageID]++
	}
	for _, id := range ids {
		if seen[id] != 1 {
			t.Errorf("image %d shown %d times in the first cycle", id, seen[id])
		}
	}
	if len(e.setter.paths) != 4 || e.setter.paths[0][:len("/library/images/")] != "/library/images/" {
		t.Errorf("setter got %q", e.setter.paths)
	}
}

func TestNextUsesTheCollectionAndFallsBack(t *testing.T) {
	e, ids := newEnv(t, 3)
	ctx := context.Background()
	coll, _ := e.lib.CreateCollection(ctx, "One")
	e.lib.AddToCollection(ctx, coll.ID, ids[1])
	if err := SaveSettings(ctx, e.set, Settings{Interval: Hourly, Order: Random, CollectionID: coll.ID}); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if res, err := e.r.Next(ctx); err != nil || res.ImageID != ids[1] {
			t.Fatalf("collection rotation picked %d, %v; want %d", res.ImageID, err, ids[1])
		}
	}

	e.lib.DeleteCollection(ctx, coll.ID)
	if res, err := e.r.Next(ctx); err != nil || res.ImageID == ids[1] {
		t.Errorf("after deleting the collection: %d, %v; want another library image", res.ImageID, err)
	}
}

func TestNextSkipsMissingFiles(t *testing.T) {
	e, ids := newEnv(t, 3)
	ctx := context.Background()
	e.files.missing["images/wallhaven/0.jpg"] = true
	e.files.missing["images/wallhaven/2.jpg"] = true

	for range 3 {
		if res, err := e.r.Next(ctx); err != nil || res.ImageID != ids[1] {
			t.Fatalf("picked %d, %v; want the only image with a file (%d)", res.ImageID, err, ids[1])
		}
	}

	e.files.missing["images/wallhaven/1.jpg"] = true
	if _, err := e.r.Next(ctx); !errors.Is(err, ErrNoImages) {
		t.Errorf("all files missing: err = %v", err)
	}
}

func TestNextWithEmptyLibrary(t *testing.T) {
	e, _ := newEnv(t, 0)
	if _, err := e.r.Next(context.Background()); !errors.Is(err, ErrNoImages) {
		t.Errorf("err = %v, want ErrNoImages", err)
	}
	if len(e.setter.paths) != 0 {
		t.Error("setter called for an empty library")
	}
}

func TestManualWallpaperIsNotRepeatedByRotation(t *testing.T) {
	e, ids := newEnv(t, 2)
	ctx := context.Background()

	if err := e.r.RecordManual(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		res, err := e.r.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// With two images, rotation alternates and never repeats the one on screen.
		if err := e.r.RecordManual(ctx, res.ImageID); err != nil {
			t.Fatal(err)
		}
	}
	st, _ := e.r.History.State(ctx)
	if st.Current == 0 {
		t.Error("history has no current image")
	}

	prev := ids[0]
	e.r.RecordManual(ctx, prev)
	res, _ := e.r.Next(ctx)
	if res.ImageID == prev {
		t.Errorf("rotation repeated the image the user just set (%d)", prev)
	}
}

func TestSettings(t *testing.T) {
	e, _ := newEnv(t, 0)
	ctx := context.Background()

	if got, err := LoadSettings(ctx, e.set); err != nil || got != DefaultSettings {
		t.Errorf("defaults = %+v, %v", got, err)
	}
	want := Settings{Enabled: true, Interval: Every15Minutes, Order: Sequential, CollectionID: 4}
	if err := SaveSettings(ctx, e.set, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadSettings(ctx, e.set); got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	for _, bad := range []Settings{
		{Interval: "5s", Order: Random},
		{Interval: Hourly, Order: "shuffle"},
		{Interval: Hourly, Order: Random, CollectionID: -1},
	} {
		if err := SaveSettings(ctx, e.set, bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}

	// A saved value that is no longer valid yields the defaults.
	e.set.Save(ctx, settingsKey, map[string]string{"interval": "5s", "order": "random"})
	if got, err := LoadSettings(ctx, e.set); err != nil || got != DefaultSettings {
		t.Errorf("invalid saved settings = %+v, %v", got, err)
	}

	if d, err := Daily.Duration(); err != nil || d.Hours() != 24 {
		t.Errorf("Daily = %v, %v", d, err)
	}
}

func TestHistoryLast(t *testing.T) {
	e, ids := newEnv(t, 2)
	ctx := context.Background()

	if _, ok, err := e.r.History.Last(ctx); ok || err != nil {
		t.Fatalf("empty history: %v, %v", ok, err)
	}
	e.r.RecordManual(ctx, ids[1])
	lc, ok, err := e.r.History.Last(ctx)
	if err != nil || !ok || lc.ImageID != ids[1] || lc.Reason != ByUser || time.Since(lc.At) > time.Minute {
		t.Errorf("Last = %+v, %v, %v", lc, ok, err)
	}
}
