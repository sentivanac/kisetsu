package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
)

func TestFetchedImagesWorkflow(t *testing.T) {
	data := jpegBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	app := newTestApp(t, &stubProvider{})
	ev := captureEvents(app)

	saved, err := app.SaveSearch("Test", searches.Query{ProviderIDs: []string{"stub"}})
	if err != nil {
		t.Fatal(err)
	}
	img := downloadable(srv, "auto", "/a.jpg")
	if _, err := app.downloads.EnqueueFetched(app.ctx, img, downloads.FetchOptions{
		SavedSearchID: saved.ID, Fit: imaging.Size{Width: 160, Height: 90},
	}); err != nil {
		t.Fatal(err)
	}
	done := ev.wait(t)
	if done.State != "done" {
		t.Fatalf("fetch = %+v", done)
	}

	page, err := app.ListFetched(0, 10)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListFetched = %+v, %v", page, err)
	}
	item := page.Items[0]
	if !strings.HasPrefix(item.FileURL, "/library/fetched/images/") || item.Image.Width != 160 {
		t.Errorf("fetched item = %+v", item)
	}
	stats, _ := app.GetFetchedStats()
	if stats.Images != 1 || stats.MaxBytes != 200<<20 {
		t.Errorf("stats = %+v", stats)
	}
	if lib, _ := app.ListLibrary(0, 0, 10); len(lib.Items) != 0 {
		t.Errorf("fetched image shown in the library: %+v", lib.Items)
	}

	_, err = app.Download(img)
	if msg := apperr.ToPublic(err).Message; !strings.Contains(msg, "Fetched") || !strings.Contains(msg, "Keep") {
		t.Errorf("manual download message = %q", msg)
	}

	kept, err := app.KeepFetched(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(kept.FileURL, "/library/images/") {
		t.Errorf("kept item = %+v", kept)
	}
	if lib, _ := app.ListLibrary(0, 0, 10); len(lib.Items) != 1 {
		t.Errorf("kept image not in the library")
	}
	if _, err := app.KeepFetched(item.ID); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("keeping twice: %v", err)
	}
}

func TestSaveDisplaySize(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	if err := app.SaveDisplaySize(1920, 1080); err != nil {
		t.Fatal(err)
	}
	if d, _ := app.settings.Display(app.ctx); d != (settings.DisplaySize{Width: 1920, Height: 1080}) {
		t.Errorf("stored %+v", d)
	}
	if err := app.SaveDisplaySize(0, 0); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("invalid size: %v", err)
	}
}
