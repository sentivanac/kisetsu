package searches

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

var mountains = Query{
	Text: "japanese mountains -snow", ProviderIDs: []string{"wallhaven", "wikimedia-commons"},
	MinWidth: 1920, MinHeight: 1080, Orientation: images.Landscape,
	Categories: []images.Category{images.General},
}

func TestCreateGetList(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	saved, err := s.Create(ctx, "  Japanese   mountains ", mountains)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.Name != "Japanese mountains" || saved.CreatedAt.IsZero() {
		t.Errorf("created %+v", saved)
	}
	got, err := s.Get(ctx, saved.ID)
	if err != nil || !reflect.DeepEqual(got.Query, mountains) {
		t.Errorf("Get = %+v, %v", got, err)
	}

	s.Create(ctx, "autumn lakes", Query{Text: "lake autumn", ProviderIDs: []string{"wallhaven"}})
	list, err := s.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "autumn lakes" {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestQueryProviderNormalizes(t *testing.T) {
	q := mountains.Provider(3)
	if q.Text != "japanese mountains" || len(q.Exclude) != 1 || q.Exclude[0] != "snow" || q.Page != 3 ||
		!slices.Equal(q.Categories, []images.Category{images.General}) {
		t.Errorf("Provider(3) = %+v", q)
	}
}

func TestCreateRejectsInvalid(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Create(ctx, "Mountains", mountains)

	tests := map[string]struct {
		name  string
		query Query
		want  error
	}{
		"empty name":      {"  ", mountains, ErrInvalidName},
		"long name":       {strings.Repeat("x", MaxName+1), mountains, ErrInvalidName},
		"name taken":      {"MOUNTAINS", mountains, ErrNameTaken},
		"no sources":      {"A", Query{Text: "x"}, ErrInvalidQuery},
		"repeated source": {"B", Query{ProviderIDs: []string{"w", "w"}}, ErrInvalidQuery},
		"bad orientation": {"C", Query{ProviderIDs: []string{"w"}, Orientation: "diagonal"}, ErrInvalidQuery},
		"bad category":    {"E", Query{ProviderIDs: []string{"w"}, Categories: []images.Category{"manga"}}, ErrInvalidQuery},
		"text too long":   {"D", Query{ProviderIDs: []string{"w"}, Text: strings.Repeat("a", 300)}, ErrInvalidQuery},
	}
	for name, tt := range tests {
		if _, err := s.Create(ctx, tt.name, tt.query); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}
}

func TestRenameAndDelete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a, _ := s.Create(ctx, "Mountains", mountains)
	s.Create(ctx, "Lakes", Query{ProviderIDs: []string{"wallhaven"}})

	if err := s.Rename(ctx, a.ID, "mountains"); err != nil {
		t.Errorf("own name in other case: %v", err)
	}
	if err := s.Rename(ctx, a.ID, "lakes"); !errors.Is(err, ErrNameTaken) {
		t.Errorf("taken: %v", err)
	}
	if err := s.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if err := s.Delete(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if err := s.Rename(ctx, 999, "X"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename missing: %v", err)
	}
}

func TestListSkipsUnreadableQueries(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Create(ctx, "Good", mountains)
	// A query saved by another version in a shape this one cannot read.
	if _, err := s.db.Exec(`INSERT INTO saved_searches (name, query) VALUES ('Odd', '{"minWidth": "wide"}')`); err != nil {
		t.Fatal(err)
	}

	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Good" {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestFindByName(t *testing.T) {
	s := newStore(t)
	saved, _ := s.Create(context.Background(), "Japanese mountains", mountains)

	got, err := s.FindByName(context.Background(), "  japanese   MOUNTAINS ")
	if err != nil || got.ID != saved.ID {
		t.Errorf("FindByName = %+v, %v", got, err)
	}
	if _, err := s.FindByName(context.Background(), "Lakes"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
}
