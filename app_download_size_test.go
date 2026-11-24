package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// countingServer serves one 320×200 image and counts requests.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	data := jpegBytes(t)
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func useScreen(t *testing.T, app *App, w, h int) {
	t.Helper()
	if err := app.settings.SaveDisplay(app.ctx, settings.DisplaySize{Width: w, Height: h}); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadFollowsTheChosenSize(t *testing.T) {
	for _, c := range []struct {
		setting, as string
		want        library.Stored
	}{
		{"", "", library.StoredOriginal},
		{"screen", "", library.StoredReduced},
		{"screen", "original", library.StoredOriginal},
		{"original", "screen", library.StoredReduced},
	} {
		srv, _ := countingServer(t)
		app := newTestApp(t, &stubProvider{})
		ev := captureEvents(app)
		useScreen(t, app, 160, 90)
		if c.setting != "" {
			if err := app.SaveDownloadSize(c.setting); err != nil {
				t.Fatal(err)
			}
		}
		img := downloadable(srv, "a", "/a.jpg")
		var err error
		if c.as == "" {
			_, err = app.Download(img)
		} else {
			_, err = app.DownloadAs(img, c.as)
		}
		if err != nil {
			t.Fatal(err)
		}
		e, err := app.library.Get(app.ctx, ev.wait(t).EntryID)
		if err != nil || e.Stored() != c.want {
			t.Errorf("%+v: stored %q, %v", c, e.Stored(), err)
		}
	}
	app := newTestApp(t, &stubProvider{})
	if app.SaveDownloadSize("huge") == nil {
		t.Error("unknown size accepted")
	}
}

// A fetched copy that covers the screen is kept without a download.
func TestScreenDownloadUsesTheFetchedCopy(t *testing.T) {
	for _, c := range []struct{ w, h, requests int }{{160, 90, 1}, {240, 135, 2}} {
		srv, n := countingServer(t)
		app := newTestApp(t, &stubProvider{})
		ev := captureEvents(app)
		saved, err := app.SaveSearch("Test", searches.Query{ProviderIDs: []string{"stub"}})
		if err != nil {
			t.Fatal(err)
		}
		img := downloadable(srv, "auto", "/a.jpg")
		p := app.downloads.FetchNow(app.ctx, img, downloads.FetchOptions{
			SavedSearchID: saved.ID, Fit: imaging.Size{Width: 160, Height: 90},
		})
		useScreen(t, app, c.w, c.h)
		if _, err := app.DownloadAs(img, "screen"); err != nil {
			t.Fatal(err)
		}
		done := ev.wait(t)
		e, _ := app.library.Get(app.ctx, p.EntryID)
		if done.State != "done" || done.EntryID != p.EntryID || e.Origin != library.FromUser || e.Image.Width < c.w {
			t.Errorf("%+v: event %+v, entry %+v", c, done, e)
		}
		if int(n.Load()) != c.requests {
			t.Errorf("%+v: %d requests", c, n.Load())
		}
	}
}

func TestGetOriginal(t *testing.T) {
	srv, n := countingServer(t)
	app := newTestApp(t, &stubProvider{})
	ev := captureEvents(app)
	useScreen(t, app, 160, 90)
	if _, err := app.DownloadAs(downloadable(srv, "a", "/a.jpg"), "screen"); err != nil {
		t.Fatal(err)
	}
	id := ev.wait(t).EntryID
	if key, err := app.GetOriginal(id); err != nil || key == "" {
		t.Fatalf("GetOriginal = %q, %v", key, err)
	}
	ev.wait(t)
	if e, _ := app.library.Get(app.ctx, id); e.Stored() != library.StoredOriginal {
		t.Errorf("stored %q", e.Stored())
	}

	// Stored before 0.9.0: whether it is the original is seen from the file.
	if _, err := app.db.Exec(`UPDATE images SET original_width = NULL, original_height = NULL`); err != nil {
		t.Fatal(err)
	}
	before := n.Load()
	if key, err := app.GetOriginal(id); err != nil || key != "" || n.Load() != before {
		t.Errorf("GetOriginal of an original = %q, %v, %d requests", key, err, n.Load()-before)
	}
	if e, _ := app.library.Get(app.ctx, id); e.Stored() != library.StoredOriginal {
		t.Errorf("stored %q after the check", e.Stored())
	}
	if _, err := app.GetOriginal(999); err == nil {
		t.Error("missing image accepted")
	}
}
