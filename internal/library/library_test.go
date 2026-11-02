package library

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

// entry returns a complete entry; n makes its identifiers unique.
func entry(n int) Entry {
	return Entry{
		Image: images.Image{
			Provider:     "wikimedia-commons",
			ProviderID:   fmt.Sprint(1000 + n),
			PageURL:      fmt.Sprintf("https://commons.wikimedia.org/wiki/File:Image_%d.jpg", n),
			ThumbnailURL: "https://example.org/thumb.jpg",
			PreviewURL:   "https://example.org/preview.jpg",
			DownloadURL:  fmt.Sprintf("https://upload.wikimedia.org/image_%d.jpg", n),
			Width:        4032,
			Height:       2688,
			MIMEType:     "image/jpeg",
			FileSize:     int64(1_000_000 + n),
			Author: images.Author{
				Name:   "Vyacheslav Argenberg",
				URL:    "https://commons.wikimedia.org/wiki/User:Argenberg",
				Credit: "Photo by Vyacheslav Argenberg. Please credit the author.",
			},
			SourceURL: "https://www.example.org/original",
			License: images.License{
				Status: images.LicenseKnown,
				Name:   "CC BY 4.0",
				URL:    "https://creativecommons.org/licenses/by/4.0",
				Notice: "Attribution required.",
			},
		},
		FilePath:      fmt.Sprintf("images/wikimedia-commons/%d.jpg", 1000+n),
		ThumbnailPath: fmt.Sprintf("thumbs/wikimedia-commons/%d.jpg", 1000+n),
		SHA256:        fmt.Sprintf("%064x", n),
		DHash:         0xF0F0_F0F0_0F0F_0F0F + uint64(n),
	}
}

func mustAdd(t *testing.T, s *Store, e Entry) int64 {
	t.Helper()
	id, err := s.Add(context.Background(), e)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAddAndGetKeepAllAttribution(t *testing.T) {
	s := newTestStore(t)
	want := entry(1)
	want.DHash = 0xFFFF_FFFF_FFFF_FFFF // highest bit set: stored as a negative int64

	id := mustAdd(t, s, want)
	got, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != id || time.Since(got.AddedAt) > time.Minute {
		t.Errorf("ID, AddedAt = %d, %v", got.ID, got.AddedAt)
	}
	// Thumbnail and preview URLs of the provider are not stored: the
	// library has its own thumbnail.
	want.Image.ThumbnailURL, want.Image.PreviewURL = "", ""
	got.ID, got.AddedAt = 0, time.Time{}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestAddRejectsDuplicates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	mustAdd(t, s, entry(1))

	sameSource := entry(2)
	sameSource.Image.Provider, sameSource.Image.ProviderID = entry(1).Image.Provider, entry(1).Image.ProviderID
	if _, err := s.Add(ctx, sameSource); !errors.Is(err, ErrAlreadyInLibrary) {
		t.Errorf("same source: err = %v, want ErrAlreadyInLibrary", err)
	}

	sameFile := entry(3)
	sameFile.SHA256 = entry(1).SHA256
	if _, err := s.Add(ctx, sameFile); !errors.Is(err, ErrDuplicateFile) {
		t.Errorf("same file: err = %v, want ErrDuplicateFile", err)
	}

	if st, _ := s.Stats(ctx); st.Images != 1 {
		t.Errorf("library has %d images after rejected duplicates, want 1", st.Images)
	}
}

func TestAddRejectsInvalidRecords(t *testing.T) {
	s := newTestStore(t)

	bad := entry(1)
	bad.Image.License.Status = "" // the schema only accepts known/unknown
	if _, err := s.Add(context.Background(), bad); err == nil {
		t.Error("expected the database to reject a missing license status")
	}
}

func TestFind(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := mustAdd(t, s, entry(1))

	if e, ok, err := s.FindBySource(ctx, "wikimedia-commons", "1001"); err != nil || !ok || e.ID != id {
		t.Errorf("FindBySource = %d, %v, %v", e.ID, ok, err)
	}
	if e, ok, err := s.FindBySHA256(ctx, entry(1).SHA256); err != nil || !ok || e.ID != id {
		t.Errorf("FindBySHA256 = %d, %v, %v", e.ID, ok, err)
	}
	if _, ok, err := s.FindBySource(ctx, "wallhaven", "1001"); err != nil || ok {
		t.Errorf("FindBySource for a missing image = %v, %v", ok, err)
	}
	if _, err := s.Get(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(999) err = %v, want ErrNotFound", err)
	}
}

func TestListNewestFirstWithPaging(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	var ids []int64
	for n := range 5 {
		ids = append(ids, mustAdd(t, s, entry(n)))
	}

	all, err := s.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, len(all))
	for i, e := range all {
		got[i] = e.ID
	}
	want := slices.Clone(ids)
	slices.Reverse(want)
	if !slices.Equal(got, want) {
		t.Errorf("List order = %v, want %v", got, want)
	}

	page, err := s.List(ctx, ListOptions{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].ID != want[2] || page[1].ID != want[3] {
		t.Errorf("page = %v", page)
	}
}

func TestStatsAndHashes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if st, err := s.Stats(ctx); err != nil || st != (Stats{}) {
		t.Errorf("empty Stats = %+v, %v", st, err)
	}

	a, b := entry(1), entry(2)
	b.DHash = 0x8000_0000_0000_0001
	idA, idB := mustAdd(t, s, a), mustAdd(t, s, b)

	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Images != 2 || st.Bytes != a.Image.FileSize+b.Image.FileSize {
		t.Errorf("Stats = %+v", st)
	}

	hashes, err := s.DHashes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]uint64{idA: a.DHash, idB: b.DHash}
	if len(hashes) != 2 || hashes[0].DHash != want[hashes[0].ID] || hashes[1].DHash != want[hashes[1].ID] {
		t.Errorf("DHashes = %+v, want %v", hashes, want)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := mustAdd(t, s, entry(1))
	c, err := s.CreateCollection(ctx, "Mountains")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddToCollection(ctx, c.ID, id); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete err = %v", err)
	}
	cols, _ := s.Collections(ctx)
	if len(cols) != 1 || cols[0].Images != 0 {
		t.Errorf("collection after deleting its image = %+v", cols)
	}
	if err := s.Delete(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete err = %v, want ErrNotFound", err)
	}
}

func TestIDsBySource(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id1, id2 := mustAdd(t, s, entry(1)), mustAdd(t, s, entry(2))

	refs := []SourceRef{
		{Provider: "wikimedia-commons", ProviderID: "1001"},
		{Provider: "wikimedia-commons", ProviderID: "1002"},
		{Provider: "wikimedia-commons", ProviderID: "9999"}, // not downloaded
		{Provider: "wallhaven", ProviderID: "1001"},         // same ID, other provider
	}
	got, err := s.IDsBySource(ctx, refs)
	if err != nil {
		t.Fatal(err)
	}
	want := map[SourceRef]int64{refs[0]: id1, refs[1]: id2}
	if len(got) != len(want) || got[refs[0]] != id1 || got[refs[1]] != id2 {
		t.Errorf("IDsBySource = %v, want %v", got, want)
	}

	if got, err := s.IDsBySource(ctx, nil); err != nil || len(got) != 0 {
		t.Errorf("no refs: %v, %v", got, err)
	}

	// More references than fit in one query.
	many := make([]SourceRef, 2*maxRefsPerQuery+5)
	for i := range many {
		many[i] = SourceRef{Provider: "wikimedia-commons", ProviderID: fmt.Sprint(5000 + i)}
	}
	many[len(many)-1] = refs[0]
	if got, err := s.IDsBySource(ctx, many); err != nil || got[refs[0]] != id1 || len(got) != 1 {
		t.Errorf("chunked lookup = %v, %v", got, err)
	}
}

func TestIDsAndHasCollection(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a, b, c := mustAdd(t, s, entry(1)), mustAdd(t, s, entry(2)), mustAdd(t, s, entry(3))
	coll, _ := s.CreateCollection(ctx, "Pick")
	s.AddToCollection(ctx, coll.ID, c)
	s.AddToCollection(ctx, coll.ID, a)

	if ids, err := s.IDs(ctx, 0); err != nil || !slices.Equal(ids, []int64{a, b, c}) {
		t.Errorf("IDs(all) = %v, %v; want oldest first", ids, err)
	}
	if ids, err := s.IDs(ctx, coll.ID); err != nil || !slices.Equal(ids, []int64{a, c}) {
		t.Errorf("IDs(collection) = %v, %v", ids, err)
	}
	if ok, err := s.HasCollection(ctx, coll.ID); !ok || err != nil {
		t.Errorf("HasCollection(existing) = %v, %v", ok, err)
	}
	if ok, err := s.HasCollection(ctx, 999); ok || err != nil {
		t.Errorf("HasCollection(missing) = %v, %v", ok, err)
	}
}
