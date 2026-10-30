//go:build live

// Downloads a real image from Wallhaven through the whole pipeline.
// Excluded from normal runs; use `make test-live`.

package downloads

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/providers/wallhaven"
)

func TestLiveDownload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 60 * time.Second}
	ua := "Kisetsu/live-test (+https://github.com/sentivanac/kisetsu)"

	page, err := wallhaven.New(client, ua).Search(ctx, providers.Query{Text: "mountains", Page: 1, MinWidth: 1920, MinHeight: 1080})
	if err != nil || len(page.Images) == 0 {
		t.Fatalf("search: %v (%d images)", err, len(page.Images))
	}
	img := page.Images[0]

	root := t.TempDir()
	db, err := database.Open(ctx, filepath.Join(root, database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := library.NewStore(db)
	done := make(chan Progress, 1)
	m, err := New(Config{
		Client: client, UserAgent: ua, Root: root, Store: store,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Notify: func(p Progress) {
			if p.State.Finished() {
				done <- p
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	start := time.Now()
	if _, err := m.Enqueue(ctx, img); err != nil {
		t.Fatal(err)
	}
	var p Progress
	select {
	case p = <-done:
	case <-ctx.Done():
		t.Fatal("download did not finish")
	}
	if p.State != Done {
		t.Fatalf("progress = %+v", p)
	}

	e, err := store.Get(ctx, p.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{e.FilePath, e.ThumbnailPath} {
		if info, err := os.Stat(filepath.Join(root, rel)); err != nil || info.Size() == 0 {
			t.Errorf("%s: %v", rel, err)
		}
	}
	t.Logf("%s: %dx%d %s, %d KB in %s", img.PageURL, e.Image.Width, e.Image.Height, e.Image.MIMEType,
		e.Image.FileSize/1024, time.Since(start).Round(time.Millisecond))
}
