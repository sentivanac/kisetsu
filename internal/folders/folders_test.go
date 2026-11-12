package folders

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/library"
)

func newStore(t *testing.T) *library.Store {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return library.NewStore(db)
}

func writeImage(t *testing.T, path string, w, h int, asPNG bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.White)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if asPNG {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func paths(t *testing.T, s *library.Store, folderID int64) []string {
	t.Helper()
	files, err := s.FolderFiles(context.Background(), folderID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for p := range files {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func TestSync(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	dir := t.TempDir()
	writeImage(t, filepath.Join(dir, "a.jpg"), 40, 30, false)
	writeImage(t, filepath.Join(dir, "Trips", "b.PNG"), 20, 10, true)
	writeImage(t, filepath.Join(dir, ".cache", "hidden.jpg"), 10, 10, false)
	writeImage(t, filepath.Join(dir, ".dot.jpg"), 10, 10, false)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not an image"), 0o644)
	os.WriteFile(filepath.Join(dir, "broken.jpg"), []byte("not a JPEG"), 0o644)
	if err := os.Symlink(filepath.Join(dir, "a.jpg"), filepath.Join(dir, "link.jpg")); err != nil {
		t.Log("no symlinks:", err)
	}

	f, err := s.AddFolder(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Sync(ctx, s, f)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Skipped != 1 {
		t.Errorf("first sync: %+v, want 2 added and the broken file skipped", res)
	}
	want := []string{FilePath(f.ID, "Trips/b.PNG"), FilePath(f.ID, "a.jpg")}
	if got := paths(t, s, f.ID); !slices.Equal(got, want) {
		t.Errorf("files = %q, want %q", got, want)
	}

	// The entry is a full library entry.
	entries, err := s.List(ctx, library.ListOptions{Origin: library.FromFolder})
	if err != nil || len(entries) != 2 {
		t.Fatalf("listed %d folder images, %v", len(entries), err)
	}
	for _, e := range entries {
		if e.FolderID != f.ID || e.SHA256 != "" || e.Image.Provider != Provider {
			t.Errorf("entry %+v", e)
		}
		if e.FilePath == FilePath(f.ID, "Trips/b.PNG") && (e.Image.Width != 20 || e.Image.MIMEType != "image/png") {
			t.Errorf("b.PNG read as %dx%d %s", e.Image.Width, e.Image.Height, e.Image.MIMEType)
		}
	}
	// Folder images stay out of the user's library.
	if lib, _ := s.List(ctx, library.ListOptions{}); len(lib) != 0 {
		t.Errorf("%d folder images in the library view", len(lib))
	}

	// Nothing changed: nothing to do.
	if res, err := Sync(ctx, s, f); err != nil || res.Added+res.Updated+res.Removed != 0 {
		t.Errorf("second sync: %+v, %v", res, err)
	}

	// A new file, a changed one and a removed one.
	writeImage(t, filepath.Join(dir, "c.jpg"), 50, 50, false)
	writeImage(t, filepath.Join(dir, "a.jpg"), 80, 60, false)
	later := time.Now().Add(time.Minute)
	os.Chtimes(filepath.Join(dir, "a.jpg"), later, later)
	os.Remove(filepath.Join(dir, "Trips", "b.PNG"))
	res, err = Sync(ctx, s, f)
	if err != nil || res.Added != 1 || res.Updated != 1 || res.Removed != 1 {
		t.Errorf("third sync: %+v, %v", res, err)
	}
	files, _ := s.FolderFiles(ctx, f.ID)
	a, _ := s.Get(ctx, files[FilePath(f.ID, "a.jpg")].ID)
	if a.Image.Width != 80 {
		t.Errorf("changed a.jpg is %d wide, want 80", a.Image.Width)
	}
}

func TestSyncKeepsImagesOfAMissingFolder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	dir := filepath.Join(t.TempDir(), "drive")
	writeImage(t, filepath.Join(dir, "a.jpg"), 40, 30, false)
	f, _ := s.AddFolder(ctx, dir)
	if _, err := Sync(ctx, s, f); err != nil {
		t.Fatal(err)
	}
	// The drive is not connected.
	if err := os.Rename(dir, dir+"-away"); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(ctx, s, f); !errors.Is(err, ErrMissing) {
		t.Errorf("err = %v, want ErrMissing", err)
	}
	if got := paths(t, s, f.ID); len(got) != 1 {
		t.Errorf("images of a missing folder were forgotten: %q", got)
	}
}

func TestSyncNeverWritesToTheFolder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	dir := t.TempDir()
	writeImage(t, filepath.Join(dir, "a.jpg"), 40, 30, false)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	f, _ := s.AddFolder(ctx, dir)
	if res, err := Sync(ctx, s, f); err != nil || res.Added != 1 {
		t.Errorf("read-only folder: %+v, %v", res, err)
	}
}

func TestRemoveFolderForgetsOnlyItsImages(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	one, two := t.TempDir(), t.TempDir()
	writeImage(t, filepath.Join(one, "a.jpg"), 40, 30, false)
	writeImage(t, filepath.Join(two, "a.jpg"), 40, 30, false)
	f1, _ := s.AddFolder(ctx, one)
	f2, _ := s.AddFolder(ctx, two)
	Sync(ctx, s, f1)
	Sync(ctx, s, f2)
	if _, err := s.RemoveFolder(ctx, f1.ID); err != nil {
		t.Fatal(err)
	}
	if len(paths(t, s, f1.ID)) != 0 || len(paths(t, s, f2.ID)) != 1 {
		t.Error("removing one folder touched the other")
	}
	if _, err := os.Stat(filepath.Join(one, "a.jpg")); err != nil {
		t.Errorf("the file itself is gone: %v", err)
	}
	if _, err := s.AddFolder(ctx, two); !errors.Is(err, library.ErrFolderExists) {
		t.Errorf("adding a folder twice: %v", err)
	}
}

func TestListByFolder(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	one, two := t.TempDir(), t.TempDir()
	writeImage(t, filepath.Join(one, "a.jpg"), 40, 30, false)
	writeImage(t, filepath.Join(two, "b.jpg"), 40, 30, false)
	writeImage(t, filepath.Join(two, "c.jpg"), 40, 30, false)
	f1, _ := s.AddFolder(ctx, one)
	f2, _ := s.AddFolder(ctx, two)
	Sync(ctx, s, f1)
	Sync(ctx, s, f2)
	got, err := s.List(ctx, library.ListOptions{Origin: library.FromFolder, FolderID: f2.ID})
	if err != nil || len(got) != 2 {
		t.Errorf("folder 2 lists %d images, %v", len(got), err)
	}
	ids, err := s.FolderIDs(ctx, f1.ID)
	if err != nil || len(ids) != 1 {
		t.Errorf("folder 1 IDs %v, %v", ids, err)
	}
}

func TestThumbnails(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	dir := t.TempDir()
	writeImage(t, filepath.Join(dir, "big.jpg"), 1200, 800, false)
	f, _ := s.AddFolder(ctx, dir)
	if _, err := Sync(ctx, s, f); err != nil {
		t.Fatal(err)
	}
	entries, _ := s.List(ctx, library.ListOptions{Origin: library.FromFolder})
	e := entries[0]
	data := t.TempDir()
	thumbs := &Thumbnails{Store: s, Dir: data, Original: func(rel string) (string, error) {
		return filepath.Join(dir, strings.TrimPrefix(rel, FilePath(f.ID, ""))), nil
	}}

	rel := ThumbnailPath(e)
	if err := thumbs.Make(rel); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(data, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(file)
	file.Close()
	if err != nil || cfg.Width != 480 || cfg.Height != 320 {
		t.Errorf("thumbnail %dx%d, %v; want 480x320", cfg.Width, cfg.Height, err)
	}
	if got, _ := s.Get(ctx, e.ID); got.ThumbnailPath != rel {
		t.Errorf("recorded thumbnail %q, want %q", got.ThumbnailPath, rel)
	}

	// An old modification time in the name: refused.
	old := e
	old.FileModTime--
	if err := thumbs.Make(ThumbnailPath(old)); err == nil {
		t.Error("made a thumbnail for an out-of-date path")
	}
	for _, bad := range []string{"thumbs/folders/x/1-a.jpg", "thumbs/wallhaven/a.jpg", "thumbs/folders/1/2.jpg", "thumbs/folders/1/a/2-a.jpg"} {
		if err := thumbs.Make(bad); err == nil {
			t.Errorf("made %q", bad)
		}
	}
}
