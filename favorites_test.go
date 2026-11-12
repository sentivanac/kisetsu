package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/searches"
)

func TestFavoritesFromAFolder(t *testing.T) {
	app, _, folder := appWithFolder(t)
	page, _ := app.ListFolder(folder, 0, 10)
	img := page.Items[0].ID

	// The first favorite makes the collection.
	if err := app.AddToFavorites(img); err != nil {
		t.Fatal(err)
	}
	cols, _ := app.ListCollections()
	if len(cols) != 1 || cols[0].Name != "Favorites" || cols[0].Images != 1 {
		t.Fatalf("collections: %+v", cols)
	}
	page, _ = app.ListFolder(folder, 0, 10)
	for _, it := range page.Items {
		if it.Favorite != (it.ID == img) {
			t.Errorf("item %d marked favorite %v", it.ID, it.Favorite)
		}
	}
	// Renamed, it is still the favorites.
	if err := app.RenameCollection(cols[0].ID, "Best"); err != nil {
		t.Fatal(err)
	}
	if err := app.AddToFavorites(page.Items[1].ID); err != nil {
		t.Fatal(err)
	}
	if cols, _ := app.ListCollections(); len(cols) != 1 || cols[0].Images != 2 {
		t.Errorf("after renaming: %+v", cols)
	}
	if err := app.RemoveFromFavorites(img); err != nil {
		t.Fatal(err)
	}
	if cols, _ := app.ListCollections(); cols[0].Images != 1 {
		t.Errorf("after removing: %+v", cols)
	}
}

func TestFavoritesUseAnExistingCollection(t *testing.T) {
	app, _, folder := appWithFolder(t)
	if _, err := app.CreateCollection("Favourites"); err != nil {
		t.Fatal(err)
	}
	page, _ := app.ListFolder(folder, 0, 10)
	if err := app.AddToFavorites(page.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	cols, _ := app.ListCollections()
	if len(cols) != 1 || cols[0].Name != "Favourites" || cols[0].Images != 1 {
		t.Errorf("collections: %+v; want the existing Favourites used", cols)
	}
}

func TestFavoriteFetchedImageIsKept(t *testing.T) {
	data := jpegBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	app := newTestApp(t, &stubProvider{})
	ev := captureEvents(app)
	saved, err := app.SaveSearch("Test", searches.Query{ProviderIDs: []string{"stub"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.downloads.EnqueueFetched(app.ctx, downloadable(srv, "auto", "/a.jpg"), downloads.FetchOptions{
		SavedSearchID: saved.ID, Fit: imaging.Size{Width: 160, Height: 90},
	}); err != nil {
		t.Fatal(err)
	}
	ev.wait(t)
	page, _ := app.ListFetched(0, 10)
	id := page.Items[0].ID

	if err := app.AddToFavorites(id); err != nil {
		t.Fatal(err)
	}
	lib, _ := app.ListLibrary(0, 0, 10)
	if len(lib.Items) != 1 || !strings.HasPrefix(lib.Items[0].FileURL, "/library/images/") || !lib.Items[0].Favorite {
		t.Errorf("the favorite fetched image was not kept: %+v", lib.Items)
	}
	if fetched, _ := app.ListFetched(0, 10); len(fetched.Items) != 0 {
		t.Errorf("still among the fetched images: %+v", fetched.Items)
	}
}

func TestFavoriteCurrentWallpaper(t *testing.T) {
	app, _, folder := appWithFolder(t)
	if _, err := app.FavoriteCurrentWallpaper(); err == nil {
		t.Error("no wallpaper yet, but no error")
	}
	page, _ := app.ListFolder(folder, 0, 10)
	if _, err := app.SetWallpaper(page.Items[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FavoriteCurrentWallpaper(); err != nil {
		t.Fatal(err)
	}
	page, _ = app.ListFolder(folder, 0, 10)
	if !page.Items[1].Favorite || page.Items[0].Favorite {
		t.Errorf("favorites: %v %v; want only the wallpaper", page.Items[0].Favorite, page.Items[1].Favorite)
	}
}
