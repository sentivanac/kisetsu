package main

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 64, 36)), nil); err != nil {
		t.Fatal(err)
	}
}

// appWithFolder returns an app with a local folder of two images, read
// once, and lets the app reach its files.
func appWithFolder(t *testing.T) (*App, string, int64) {
	t.Helper()
	app := newTestApp(t, &stubProvider{})
	dir := filepath.Join(t.TempDir(), "Pictures")
	writeTestJPEG(t, filepath.Join(dir, "a.jpg"))
	writeTestJPEG(t, filepath.Join(dir, "Trips", "b.jpg"))
	f, err := app.library.AddFolder(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	app.files.SetFolders(func(id int64) (string, error) {
		got, err := app.library.Folder(context.Background(), id)
		return got.Path, err
	}, nil)
	if _, err := app.SyncFolder(f.ID); err != nil {
		t.Fatal(err)
	}
	return app, dir, f.ID
}

func TestFolderInTheWindow(t *testing.T) {
	app, dir, id := appWithFolder(t)

	list, err := app.ListFolders()
	if err != nil || len(list) != 1 || list[0].Images != 2 || list[0].Name != "Pictures" || list[0].Missing {
		t.Fatalf("folders: %+v, %v", list, err)
	}
	page, err := app.ListFolder(id, 0, 10)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("folder page: %d items, %v", len(page.Items), err)
	}
	// The thumbnail is made on request, under thumbs/folders/.
	if thumb := page.Items[0].Image.ThumbnailURL; filepath.Dir(filepath.Dir(thumb)) != "/library/thumbs/folders" {
		t.Errorf("thumbnail URL %q", thumb)
	}
	// Folder images are not in the user's library.
	if lib, _ := app.ListLibrary(0, 0, 10); len(lib.Items) != 0 {
		t.Errorf("%d folder images in the library", len(lib.Items))
	}

	// Never deleted from the window.
	err = app.DeleteFromLibrary(page.Items[0].ID)
	if err == nil || apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("deleting a folder image: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.jpg")); err != nil {
		t.Errorf("the file is gone: %v", err)
	}

	// Set as wallpaper, like any image.
	if _, err := app.SetWallpaper(page.Items[0].ID); err != nil {
		t.Errorf("set a folder image as wallpaper: %v", err)
	}
}

func TestMissingFolderKeepsItsImages(t *testing.T) {
	app, dir, id := appWithFolder(t)
	if err := os.Rename(dir, dir+"-away"); err != nil {
		t.Fatal(err)
	}
	res, err := app.SyncFolder(id)
	if err != nil || !res.Folder.Missing || res.Folder.Images != 2 {
		t.Errorf("missing folder: %+v, %v", res, err)
	}
}

func TestRemoveFolderKeepsTheFiles(t *testing.T) {
	app, dir, id := appWithFolder(t)
	if err := app.RemoveFolder(id); err != nil {
		t.Fatal(err)
	}
	if list, _ := app.ListFolders(); len(list) != 0 {
		t.Errorf("folders after removing: %+v", list)
	}
	for _, f := range []string{"a.jpg", filepath.Join("Trips", "b.jpg")} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
