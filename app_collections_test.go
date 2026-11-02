package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

func TestCollectionsWorkflow(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	ids := downloadN(t, app, 2)

	mountains, err := app.CreateCollection("  Mountains ")
	if err != nil {
		t.Fatal(err)
	}
	if mountains.Name != "Mountains" || mountains.Images != 0 {
		t.Errorf("created %+v", mountains)
	}
	lakes, err := app.CreateCollection("Lakes")
	if err != nil {
		t.Fatal(err)
	}

	for _, img := range ids {
		if err := app.AddToCollection(mountains.ID, img); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.AddToCollection(lakes.ID, ids[1]); err != nil {
		t.Fatal(err)
	}

	list, err := app.ListCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Lakes" || list[0].Images != 1 || list[1].Images != 2 {
		t.Errorf("ListCollections = %+v", list)
	}

	inMountains, err := app.ListLibrary(mountains.ID, 0, 10)
	if err != nil || len(inMountains.Items) != 2 {
		t.Errorf("Mountains page = %+v, %v", inMountains, err)
	}
	member, err := app.ImageCollections(ids[1])
	if err != nil || !slices.Equal(member, []int64{mountains.ID, lakes.ID}) {
		t.Errorf("ImageCollections = %v, %v", member, err)
	}

	if err := app.RenameCollection(lakes.ID, "Lakes and rivers"); err != nil {
		t.Fatal(err)
	}
	if err := app.RemoveFromCollection(mountains.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteCollection(lakes.ID); err != nil {
		t.Fatal(err)
	}

	list, _ = app.ListCollections()
	if len(list) != 1 || list[0].ID != mountains.ID || list[0].Images != 1 {
		t.Errorf("after changes = %+v", list)
	}
	if st, _ := app.GetLibraryStats(); st.Images != 2 {
		t.Errorf("deleting a collection removed images: %+v", st)
	}
	none, err := app.ImageCollections(ids[0])
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("ImageCollections for an image in no collection = %#v, %v (want empty, not nil)", none, err)
	}
}

func TestCollectionErrors(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	c, err := app.CreateCollection("Autumn")
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		err error
		msg string
	}{
		"duplicate name": {func() error { _, err := app.CreateCollection("AUTUMN"); return err }(), "already exists"},
		"empty name":     {func() error { _, err := app.CreateCollection("   "); return err }(), "1 to 100"},
		"too long":       {app.RenameCollection(c.ID, strings.Repeat("x", 101)), "1 to 100"},
		"missing image":  {app.AddToCollection(c.ID, 999), "no longer exists"},
		"missing coll.":  {app.DeleteCollection(999), "no longer exists"},
	}
	for name, tt := range tests {
		pub := apperr.ToPublic(tt.err)
		if pub.Code != apperr.InvalidInput || !strings.Contains(pub.Message, tt.msg) {
			t.Errorf("%s: %q %q, want invalid_input containing %q", name, pub.Code, pub.Message, tt.msg)
		}
	}
}
