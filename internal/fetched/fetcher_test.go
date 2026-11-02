package fetched

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// fakeSearch returns 3 images per page; a source has `pages` pages.
type fakeSearch struct {
	pages int
	fail  bool
	asked [][]search.SourcePage
}

func (f *fakeSearch) Providers() []search.ProviderInfo {
	return []search.ProviderInfo{{ID: "w", Name: "W"}}
}

func (f *fakeSearch) SearchMany(ctx context.Context, sources []search.SourcePage, q providers.Query) (search.MultiResult, error) {
	f.asked = append(f.asked, sources)
	var res search.MultiResult
	for _, src := range sources {
		if f.fail {
			res.Sources = append(res.Sources, search.SourceOutcome{ProviderID: src.ProviderID, Page: src.Page,
				Err: errors.New("offline")})
			continue
		}
		for i := range 3 {
			res.Images = append(res.Images, images.Image{Provider: src.ProviderID, ProviderID: fmt.Sprintf("p%d-%d", src.Page, i)})
		}
		res.Sources = append(res.Sources, search.SourceOutcome{ProviderID: src.ProviderID, Page: src.Page,
			Count: 3, HasMore: src.Page < f.pages})
	}
	return res, nil
}

// fakeDownloader stores fetched entries directly, without network.
type fakeDownloader struct {
	e   *env
	fit []downloads.FetchOptions
	n   int
}

func (d *fakeDownloader) FetchNow(ctx context.Context, img images.Image, opts downloads.FetchOptions) downloads.Progress {
	key := img.Provider + "/" + img.ProviderID
	if _, found, _ := d.e.lib.FindBySource(ctx, img.Provider, img.ProviderID); found {
		return downloads.Progress{Key: key, State: downloads.Duplicate}
	}
	d.fit = append(d.fit, opts)
	d.n++
	id, err := d.e.lib.Add(ctx, library.Entry{
		Image: images.Image{
			Provider: img.Provider, ProviderID: img.ProviderID, PageURL: "https://x", DownloadURL: "https://x",
			Width: 1, Height: 1, MIMEType: "image/jpeg", FileSize: 100,
			License: images.License{Status: images.LicenseUnknown},
		},
		FilePath: "fetched/images/w/" + img.ProviderID + ".jpg", ThumbnailPath: "fetched/thumbs/w/" + img.ProviderID + ".jpg",
		SHA256: fmt.Sprintf("%064d", 1000+d.n), Origin: library.Fetched, SavedSearchID: opts.SavedSearchID,
	})
	if err != nil {
		return downloads.Progress{Key: key, State: downloads.Failed, Err: err}
	}
	return downloads.Progress{Key: key, State: downloads.Done, EntryID: id}
}

func newFetcher(t *testing.T, pages int) (*Fetcher, *fakeSearch, *fakeDownloader, int64) {
	t.Helper()
	e := newEnv(t)
	sqlDB := openDB(t, e)
	store := searches.NewStore(sqlDB)
	saved, err := store.Create(context.Background(), "Lakes", searches.Query{Text: "lake", ProviderIDs: []string{"w", "gone"}})
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeSearch{pages: pages}
	dl := &fakeDownloader{e: e}
	return &Fetcher{
		DB: sqlDB, Searches: store, Settings: settings.NewStore(sqlDB), Search: fs, Downloader: dl,
		Service: e.svc, Log: e.svc.Log,
	}, fs, dl, saved.ID
}

func TestFetchContinuesWhereItStopped(t *testing.T) {
	f, fs, dl, id := newFetcher(t, 5)
	ctx := context.Background()

	res, err := f.Fetch(ctx, id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 2 {
		t.Fatalf("fetched %d, want 2", res.Fetched)
	}
	// The source "gone" is not registered and is skipped.
	if want := []search.SourcePage{{ProviderID: "w", Page: 1}}; !slices.Equal(fs.asked[0], want) {
		t.Errorf("first search = %v", fs.asked[0])
	}

	// The next fetch starts on page 2; the third image of page 1 was not
	// taken, which is fine: pages, not images, are the unit.
	if _, err := f.Fetch(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	if got := fs.asked[1][0].Page; got != 2 {
		t.Errorf("second fetch searched page %d, want 2", got)
	}
	if dl.fit[0].Fit.Width != 1920 || dl.fit[0].SavedSearchID != id {
		t.Errorf("fetch options = %+v (default display size, saved search)", dl.fit[0])
	}
}

func TestFetchSkipsKnownImagesAndUsesSeveralRounds(t *testing.T) {
	f, fs, _, id := newFetcher(t, 5)
	ctx := context.Background()
	// Page 1 is already fully known.
	f.Fetch(ctx, id, 3)
	f.DB.Exec(`UPDATE fetch_cursors SET next_page = 1`)

	res, err := f.Fetch(ctx, id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 2 || len(fs.asked) != 3 || fs.asked[2][0].Page != 2 {
		t.Errorf("fetched %d with searches %v; want 2 new images from page 2", res.Fetched, fs.asked)
	}
}

func TestFetchStartsOverAfterTheLastPage(t *testing.T) {
	f, fs, _, id := newFetcher(t, 1) // a single page
	ctx := context.Background()

	f.Fetch(ctx, id, 3)
	f.Fetch(ctx, id, 1) // nothing new on page 1
	if got := fs.asked[len(fs.asked)-1][0].Page; got != 1 {
		t.Errorf("after the last page the search asked page %d, want 1", got)
	}
}

func TestFetchToleratesFailingSources(t *testing.T) {
	f, fs, _, id := newFetcher(t, 5)
	fs.fail = true

	res, err := f.Fetch(context.Background(), id, 3)
	if err != nil || res.Fetched != 0 {
		t.Errorf("offline: %+v, %v; want no images and no error", res, err)
	}
	if len(fs.asked) != 1 {
		t.Errorf("searched %d times while offline, want 1", len(fs.asked))
	}
}

// Just fetched images are never pruned, so one fetch may go over the limit
// by its own size; the next fetch brings it back by removing older images.
func TestFetchPrunesButNotTheNewImages(t *testing.T) {
	f, _, _, id := newFetcher(t, 5)
	f.Service.MaxBytes = 250 // room for 2 images of 100 bytes
	ctx := context.Background()

	first, _ := f.Fetch(ctx, id, 3)
	if first.Pruned != 0 {
		t.Errorf("first fetch pruned %d of its own images", first.Pruned)
	}
	second, _ := f.Fetch(ctx, id, 2)
	if second.Pruned != 3 {
		t.Errorf("second fetch pruned %d, want the 3 older images", second.Pruned)
	}
	for _, newID := range second.IDs {
		if _, err := f.Service.Library.Get(ctx, newID); err != nil {
			t.Errorf("a just fetched image was pruned: %d", newID)
		}
	}
	if st, _ := f.Service.Library.FetchedStats(ctx); st.Bytes > 250 {
		t.Errorf("%d bytes fetched after the second fetch, limit 250", st.Bytes)
	}
}
