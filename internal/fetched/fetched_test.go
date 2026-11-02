package fetched

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
)

type env struct {
	root string
	db   *sql.DB
	lib  *library.Store
	svc  *Service
}

func openDB(t *testing.T, e *env) *sql.DB { return e.db }

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"images", "thumbs", "cache/fetched/images", "cache/fetched/thumbs"} {
		os.MkdirAll(filepath.Join(root, d), 0o700)
	}
	db, err := database.Open(context.Background(), filepath.Join(root, database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	files, err := libfiles.New(root, filepath.Join(root, "cache", "fetched"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })
	lib := library.NewStore(db)
	return &env{root: root, db: db, lib: lib, svc: &Service{Library: lib, Files: files, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}}
}

// add stores an image with files on disk; fetched images go to the cache.
func (e *env) add(t *testing.T, n int, origin library.Origin, size int64) int64 {
	t.Helper()
	dir := ""
	if origin == library.Fetched {
		dir = "fetched/"
	}
	file := fmt.Sprintf("%simages/w/%d.jpg", dir, n)
	thumb := fmt.Sprintf("%sthumbs/w/%d.jpg", dir, n)
	for _, rel := range []string{file, thumb} {
		abs := filepath.Join(e.root, rel)
		if origin == library.Fetched {
			abs = filepath.Join(e.root, "cache", rel)
		}
		os.MkdirAll(filepath.Dir(abs), 0o700)
		os.WriteFile(abs, []byte(rel), 0o600)
	}
	id, err := e.lib.Add(context.Background(), library.Entry{
		Image: images.Image{
			Provider: "w", ProviderID: fmt.Sprint(n), PageURL: "https://x", DownloadURL: "https://x",
			Width: 1, Height: 1, MIMEType: "image/jpeg", FileSize: size,
			License: images.License{Status: images.LicenseUnknown},
		},
		FilePath: file, ThumbnailPath: thumb, SHA256: fmt.Sprintf("%064d", n), Origin: origin,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestKeepMovesIntoTheLibrary(t *testing.T) {
	e := newEnv(t)
	id := e.add(t, 1, library.Fetched, 100)

	kept, err := e.svc.Keep(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Origin != library.FromUser || kept.FilePath != "images/w/1.jpg" || kept.ThumbnailPath != "thumbs/w/1.jpg" {
		t.Errorf("kept = %+v", kept)
	}
	if _, err := os.Stat(filepath.Join(e.root, "images", "w", "1.jpg")); err != nil {
		t.Errorf("library copy missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.root, "cache", "fetched", "images", "w", "1.jpg")); !os.IsNotExist(err) {
		t.Errorf("fetched file still in the cache: %v", err)
	}
	if _, err := e.svc.Keep(context.Background(), id); !errors.Is(err, ErrNotFetched) {
		t.Errorf("keeping twice: %v", err)
	}
}

func TestPruneKeepsWithinTheLimit(t *testing.T) {
	e := newEnv(t)
	e.svc.MaxBytes = 250
	ctx := context.Background()
	mine := e.add(t, 0, library.FromUser, 1000) // the user's library never counts or goes
	oldest := e.add(t, 1, library.Fetched, 100)
	onScreen := e.add(t, 2, library.Fetched, 100)
	e.add(t, 3, library.Fetched, 100)
	newest := e.add(t, 4, library.Fetched, 100)

	removed, freed, err := e.svc.Prune(ctx, onScreen)
	if err != nil {
		t.Fatal(err)
	}
	// 400 bytes fetched, limit 250: the two oldest that are not on screen go.
	if removed != 2 || freed != 200 {
		t.Errorf("removed %d, freed %d; want 2, 200", removed, freed)
	}
	for id, want := range map[int64]bool{mine: true, oldest: false, onScreen: true, newest: true} {
		if _, err := e.lib.Get(ctx, id); (err == nil) != want {
			t.Errorf("image %d present = %v, want %v", id, err == nil, want)
		}
	}
	if _, err := os.Stat(filepath.Join(e.root, "cache", "fetched", "images", "w", "1.jpg")); !os.IsNotExist(err) {
		t.Errorf("pruned file still on disk: %v", err)
	}
	if st, _ := e.lib.FetchedStats(ctx); st.Bytes > 250 {
		t.Errorf("still %d bytes fetched", st.Bytes)
	}

	if removed, _, _ := e.svc.Prune(ctx, onScreen); removed != 0 {
		t.Errorf("second prune removed %d", removed)
	}
}
