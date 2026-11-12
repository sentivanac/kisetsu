package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/rotation"
)

func TestRotationFromAFolder(t *testing.T) {
	c, r, fake := testCore(t, 2) // two library images, which must not be chosen
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "Pictures")
	writeTestJPEG(t, filepath.Join(dir, "a.jpg"))
	f, err := c.lib.AddFolder(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rotation.DefaultSettings
	cfg.FolderID = f.ID
	if err := rotation.SaveSettings(ctx, r.Settings, cfg); err != nil {
		t.Fatal(err)
	}

	// The folder was never read: rotation reads it first.
	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.path != filepath.Join(dir, "a.jpg") {
		t.Fatalf("set %q, want the folder's image", fake.path)
	}

	// A new file takes part at the next change.
	writeTestJPEG(t, filepath.Join(dir, "b.jpg"))
	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.path != filepath.Join(dir, "b.jpg") {
		t.Errorf("set %q, want the new b.jpg (a.jpg was shown in this cycle)", fake.path)
	}

	// A drive that is not connected: no change, and the images stay known.
	if err := os.Rename(dir, dir+"-away"); err != nil {
		t.Fatal(err)
	}
	fake.path = ""
	if _, err := r.Next(ctx); err == nil || fake.path != "" {
		t.Errorf("missing folder: err %v, set %q", err, fake.path)
	}
	if ids, _ := c.lib.FolderIDs(ctx, f.ID); len(ids) != 2 {
		t.Errorf("%d images known after the folder went away, want 2", len(ids))
	}

	// A removed folder falls back to the whole library.
	if _, err := c.lib.RemoveFolder(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fake.path, filepath.Join(c.paths.Data, "images")) {
		t.Errorf("after removing the folder, set %q, want a library image", fake.path)
	}
}

func TestRotationSettingsTakeOneSource(t *testing.T) {
	for _, s := range []rotation.Settings{
		{Interval: rotation.Hourly, Order: rotation.Random, CollectionID: 1, FolderID: 2},
		{Interval: rotation.Hourly, Order: rotation.Random, SavedSearchID: 1, FolderID: 2},
		{Interval: rotation.Hourly, Order: rotation.Random, FolderID: -1},
	} {
		if err := s.Validate(); err == nil {
			t.Errorf("%+v accepted", s)
		}
	}
}

func TestRotationFromACollectionWithAFolderImage(t *testing.T) {
	c, r, fake := testCore(t, 0)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "Pictures")
	writeTestJPEG(t, filepath.Join(dir, "a.jpg"))
	f, _ := c.lib.AddFolder(ctx, dir)
	if _, err := syncFolder(ctx, c.lib, c.files, f, nil, c.log); err != nil {
		t.Fatal(err)
	}
	ids, _ := c.lib.FolderIDs(ctx, f.ID)
	col, err := c.lib.CreateCollection(ctx, "Favourites")
	if err != nil {
		t.Fatal(err)
	}
	c.lib.AddToCollection(ctx, col.ID, ids[0])

	cfg := rotation.DefaultSettings
	cfg.CollectionID = col.ID
	rotation.SaveSettings(ctx, r.Settings, cfg)
	if _, err := r.Next(ctx); err != nil {
		t.Fatalf("rotation from a collection with only a folder image: %v", err)
	}
	if fake.path != filepath.Join(dir, "a.jpg") {
		t.Errorf("set %q, want the folder image", fake.path)
	}
}
