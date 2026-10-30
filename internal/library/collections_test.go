package library

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestCreateCollection(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	c, err := s.CreateCollection(ctx, "  Edo   period  ")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.Name != "Edo period" || c.CreatedAt.IsZero() {
		t.Errorf("collection = %+v", c)
	}

	if _, err := s.CreateCollection(ctx, "EDO PERIOD"); !errors.Is(err, ErrCollectionExists) {
		t.Errorf("same name in other case: err = %v, want ErrCollectionExists", err)
	}
	for _, name := range []string{"", "   ", strings.Repeat("x", MaxCollectionName+1)} {
		if _, err := s.CreateCollection(ctx, name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("name %q: err = %v, want ErrInvalidName", name, err)
		}
	}
	// Characters, not bytes, are counted.
	if _, err := s.CreateCollection(ctx, strings.Repeat("季", MaxCollectionName)); err != nil {
		t.Errorf("100 Japanese characters: %v", err)
	}
}

func TestRenameCollection(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a, _ := s.CreateCollection(ctx, "Autumn")
	s.CreateCollection(ctx, "Winter")

	if err := s.RenameCollection(ctx, a.ID, "autumn"); err != nil {
		t.Errorf("changing only the case of its own name: %v", err)
	}
	if err := s.RenameCollection(ctx, a.ID, "winter"); !errors.Is(err, ErrCollectionExists) {
		t.Errorf("rename to another collection's name: err = %v", err)
	}
	if err := s.RenameCollection(ctx, 999, "Spring"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename missing collection: err = %v", err)
	}

	cols, _ := s.Collections(ctx)
	if len(cols) != 2 || cols[0].Name != "autumn" {
		t.Errorf("collections = %+v", cols)
	}
}

func TestCollectionMembership(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	img1, img2 := mustAdd(t, s, entry(1)), mustAdd(t, s, entry(2))
	mountains, _ := s.CreateCollection(ctx, "Mountains")
	lakes, _ := s.CreateCollection(ctx, "Lakes")

	for _, step := range []struct{ coll, img int64 }{
		{mountains.ID, img1}, {mountains.ID, img1}, // twice is fine
		{mountains.ID, img2}, {lakes.ID, img2},
	} {
		if err := s.AddToCollection(ctx, step.coll, step.img); err != nil {
			t.Fatal(err)
		}
	}

	cols, err := s.Collections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range cols {
		counts[c.Name] = c.Images
	}
	if counts["Mountains"] != 2 || counts["Lakes"] != 1 || cols[0].Name != "Lakes" {
		t.Errorf("collections = %+v", cols)
	}

	inMountains, _ := s.List(ctx, ListOptions{CollectionID: mountains.ID})
	inLakes, _ := s.List(ctx, ListOptions{CollectionID: lakes.ID})
	if len(inMountains) != 2 || len(inLakes) != 1 || inLakes[0].ID != img2 {
		t.Errorf("Mountains %d images, Lakes %v", len(inMountains), inLakes)
	}

	member, _ := s.ImageCollections(ctx, img2)
	if want := []int64{mountains.ID, lakes.ID}; !slices.Equal(member, want) {
		t.Errorf("ImageCollections = %v, want %v", member, want)
	}

	if err := s.RemoveFromCollection(ctx, mountains.ID, img1); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveFromCollection(ctx, mountains.ID, img1); err != nil {
		t.Errorf("removing twice: %v", err)
	}
	if _, err := s.Get(ctx, img1); err != nil {
		t.Errorf("image removed from a collection must stay in the library: %v", err)
	}
}

func TestAddToCollectionChecksBothSides(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	img := mustAdd(t, s, entry(1))
	c, _ := s.CreateCollection(ctx, "Mountains")

	if err := s.AddToCollection(ctx, 999, img); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing collection: err = %v", err)
	}
	if err := s.AddToCollection(ctx, c.ID, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing image: err = %v", err)
	}
}

func TestDeleteCollectionKeepsImages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	img := mustAdd(t, s, entry(1))
	c, _ := s.CreateCollection(ctx, "Mountains")
	s.AddToCollection(ctx, c.ID, img)

	if err := s.DeleteCollection(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, img); err != nil {
		t.Errorf("image deleted with its collection: %v", err)
	}
	if member, _ := s.ImageCollections(ctx, img); len(member) != 0 {
		t.Errorf("membership left behind: %v", member)
	}
	if err := s.DeleteCollection(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: err = %v", err)
	}
}
