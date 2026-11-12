package rotation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
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

// fakeScreen records paths instead of changing the desktop.
type fakeScreen struct{ paths []string }

func (f *fakeScreen) Show(ctx context.Context, e library.Entry, path string) (wallpaper.Method, error) {
	f.paths = append(f.paths, path)
	return wallpaper.MATE, nil
}

type env struct {
	r      *Rotator
	lib    *library.Store
	set    *settings.Store
	setter *fakeScreen
	files  fakeFiles
}

func newEnv(t *testing.T, images int) (*env, []int64) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	e := &env{lib: library.NewStore(db), set: settings.NewStore(db), setter: &fakeScreen{},
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
		Screen: e.setter, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Rand: seeded()}
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

// addFetched stores n fetched images for saved search 1.
func addFetched(t *testing.T, e *env, from, n int) []int64 {
	t.Helper()
	var ids []int64
	for i := from; i < from+n; i++ {
		en := entry(100 + i)
		en.Origin, en.SavedSearchID = library.Fetched, 1
		en.FilePath = fmt.Sprintf("fetched/images/w/%d.jpg", i)
		id, err := e.lib.Add(context.Background(), en)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func useSavedSearch(t *testing.T, e *env) {
	t.Helper()
	ctx := context.Background()
	db := e.r.History.db
	if _, err := db.Exec(`INSERT INTO saved_searches (id, name, query) VALUES (1, 'Lakes', '{}')`); err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(ctx, e.set, Settings{Interval: Hourly, Order: Random, SavedSearchID: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestSavedSearchRotatesOnlyItsFetchedImages(t *testing.T) {
	e, library := newEnv(t, 2) // two library images that must not be used
	useSavedSearch(t, e)
	fetched := addFetched(t, e, 0, 4)

	for range 4 {
		res, err := e.r.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(library, res.ImageID) {
			t.Fatalf("rotation used library image %d for a saved search", res.ImageID)
		}
		if !slices.Contains(fetched, res.ImageID) {
			t.Fatalf("rotation used unknown image %d", res.ImageID)
		}
	}
}

func TestSavedSearchTopsUpWhenRunningLow(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	addFetched(t, e, 0, 1)
	calls := 0
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
		calls++
		if id != 1 || want != topUpCount {
			t.Errorf("TopUp(%d, %d)", id, want)
		}
		addFetched(t, e, 10*calls, want)
		return want, nil
	}

	res, err := e.r.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("TopUp called %d times, want 1", calls)
	}
	if res.ImageID == 0 {
		t.Error("no image chosen after the top-up")
	}
	// Plenty unshown now: no fetch on the next change.
	e.r.Next(context.Background())
	if calls != 1 {
		t.Errorf("TopUp called again with enough images: %d", calls)
	}
}

func TestSavedSearchWorksOffline(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	fetched := addFetched(t, e, 0, 2)
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
		return 0, errors.New("network is unreachable")
	}

	res, err := e.r.Next(context.Background())
	if err != nil || !slices.Contains(fetched, res.ImageID) {
		t.Errorf("offline rotation: %+v, %v; want a fetched image", res, err)
	}
}

func TestSavedSearchWithoutImages(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) { return 0, nil }

	if _, err := e.r.Next(context.Background()); !errors.Is(err, ErrNoImages) {
		t.Errorf("err = %v, want ErrNoImages", err)
	}
}

func TestDeletedSavedSearchFallsBackToTheLibrary(t *testing.T) {
	e, libraryIDs := newEnv(t, 2)
	useSavedSearch(t, e)
	e.r.History.db.Exec(`DELETE FROM saved_searches WHERE id = 1`)

	res, err := e.r.Next(context.Background())
	if err != nil || !slices.Contains(libraryIDs, res.ImageID) {
		t.Errorf("after deleting the search: %+v, %v", res, err)
	}
}

func TestSettingsRejectBothSources(t *testing.T) {
	s := Settings{Interval: Hourly, Order: Random, CollectionID: 1, SavedSearchID: 2}
	if err := s.Validate(); err == nil {
		t.Error("a collection and a saved search together were accepted")
	}
}

func TestSavedSearchChangesBeforeFetching(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	addFetched(t, e, 0, 2) // two unshown: enough to change at once
	var order []string
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
		order = append(order, "fetch")
		return 0, nil
	}
	e.r.Fetching = func(bool) { order = append(order, "waiting") }
	shownBefore := len(e.setter.paths)
	if _, err := e.r.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(e.setter.paths) != shownBefore+1 {
		t.Fatal("the wallpaper did not change")
	}
	// Changed first, then fetched (one left, fewer than lowWater), never
	// waiting for it.
	if len(order) != 1 || order[0] != "fetch" {
		t.Errorf("order %v, want only a fetch after the change", order)
	}
}

func TestSavedSearchWaitsOnlyWhenNothingIsNew(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	first := addFetched(t, e, 0, 1)
	e.r.RecordManual(context.Background(), first[0]) // on screen
	st, _ := e.r.History.State(context.Background())
	e.r.History.Record(context.Background(), first[0], st.Cycle, ByRotation) // and shown
	var waiting []bool
	e.r.Fetching = func(on bool) { waiting = append(waiting, on) }
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
		addFetched(t, e, 10, want)
		return want, nil
	}
	res, err := e.r.Next(context.Background())
	if err != nil || res.ImageID == first[0] {
		t.Fatalf("Next: %+v, %v; want a new image", res, err)
	}
	if len(waiting) != 2 || !waiting[0] || waiting[1] {
		t.Errorf("Fetching calls %v, want [true false] around the wait", waiting)
	}
}

func TestSavedSearchFetchesInTheBackground(t *testing.T) {
	e, _ := newEnv(t, 0)
	useSavedSearch(t, e)
	addFetched(t, e, 0, 2)
	release := make(chan struct{})
	done := make(chan struct{})
	e.r.TopUpInBackground = true
	e.r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
		<-release // a slow network
		close(done)
		return 0, nil
	}
	if _, err := e.r.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Next returned while the fetch still waits.
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the background fetch never ran")
	}
}
