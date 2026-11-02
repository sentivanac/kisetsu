package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

// downloadN downloads n different images and returns their entry IDs in
// download order.
func downloadN(t *testing.T, app *App, n int) []int64 {
	t.Helper()
	data := jpegBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Each path gets different bytes, so no download is an exact duplicate.
		w.Write(append(append([]byte{}, data...), r.URL.Path...))
	}))
	t.Cleanup(srv.Close)
	ev := captureEvents(app)

	var ids []int64
	for i := range n {
		id := string(rune('a' + i))
		if _, err := app.Download(downloadable(srv, id, "/"+id+".jpg")); err != nil {
			t.Fatal(err)
		}
		done := ev.wait(t)
		if done.EntryID == 0 {
			t.Fatalf("download %s: %+v", id, done)
		}
		ids = append(ids, done.EntryID)
	}
	return ids
}

func TestListLibraryPages(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 3)

	first, err := app.ListLibrary(0, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || !first.HasMore || first.Items[0].ID != ids[2] {
		t.Errorf("first page = %+v", first)
	}
	item := first.Items[0]
	if !strings.HasPrefix(item.Image.ThumbnailURL, "/library/thumbs/") || !strings.HasPrefix(item.FileURL, "/library/images/") {
		t.Errorf("URLs = %q, %q", item.Image.ThumbnailURL, item.FileURL)
	}
	if !strings.HasPrefix(item.Image.PageURL, "https://example.org/") || item.AddedAt == "" {
		t.Errorf("attribution or date missing: %+v", item)
	}

	last, err := app.ListLibrary(0, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.HasMore || last.Items[0].ID != ids[0] {
		t.Errorf("last page = %+v", last)
	}

	for _, bad := range [][3]int{{0, -1, 10}, {0, 0, 0}, {0, 0, maxLibraryPage + 1}, {-1, 0, 10}} {
		if _, err := app.ListLibrary(int64(bad[0]), bad[1], bad[2]); apperr.ToPublic(err).Code != apperr.InvalidInput {
			t.Errorf("ListLibrary%v err = %v", bad, err)
		}
	}
}

func TestDeleteFromLibraryRemovesRecordAndFiles(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 2)
	root := filepath.Dir(app.dbPath)
	entry, err := app.library.Get(app.ctx, ids[0])
	if err != nil {
		t.Fatal(err)
	}

	before, _ := app.GetLibraryStats()
	if err := app.DeleteFromLibrary(ids[0]); err != nil {
		t.Fatal(err)
	}
	after, _ := app.GetLibraryStats()

	if before.Images != 2 || after.Images != 1 || after.Bytes >= before.Bytes {
		t.Errorf("stats before %+v, after %+v", before, after)
	}
	for _, rel := range []string{entry.FilePath, entry.ThumbnailPath} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Errorf("%s still exists (err %v)", rel, err)
		}
	}
	if err := app.DeleteFromLibrary(ids[0]); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("second delete: err = %v", err)
	}

	// The other image is untouched.
	other, err := app.library.Get(app.ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, other.FilePath)); err != nil {
		t.Errorf("other image file missing: %v", err)
	}
}

func TestDeleteFromLibraryNeverRemovesFilesOutsideTheLibrary(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 1)
	root := filepath.Dir(app.dbPath)
	// A damaged record pointing at the database.
	if _, err := app.db.Exec(`UPDATE images SET file_path = 'kisetsu.db', thumbnail_path = '../outside.txt' WHERE id = ?`, ids[0]); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	os.WriteFile(outside, []byte("keep"), 0o600)
	t.Cleanup(func() { os.Remove(outside) })

	if err := app.DeleteFromLibrary(ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app.dbPath); err != nil {
		t.Errorf("database file removed: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("file outside the library removed: %v", err)
	}
}
