package settings

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
)

func newTestStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db), db
}

func TestLoadMissingSetting(t *testing.T) {
	store, _ := newTestStore(t)

	v := "unchanged"
	found, err := store.Load(context.Background(), "missing", &v)
	if err != nil {
		t.Fatal(err)
	}
	if found || v != "unchanged" {
		t.Errorf("found = %v, v = %q; want false, unchanged", found, v)
	}
}

func TestSaveAndLoad(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "k", map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, "k", map[string]int{"a": 2}); err != nil { // overwrite
		t.Fatal(err)
	}

	var got map[string]int
	found, err := store.Load(ctx, "k", &got)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got["a"] != 2 {
		t.Errorf("found = %v, got = %v; want true, a=2", found, got)
	}
}

func TestLoadReportsUndecodableValue(t *testing.T) {
	store, db := newTestStore(t)
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('k', '"text"')`); err != nil {
		t.Fatal(err)
	}

	var n int
	if _, err := store.Load(context.Background(), "k", &n); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("err = %v, want ErrInvalidValue", err)
	}
}

func TestDatabaseRejectsInvalidJSON(t *testing.T) {
	_, db := newTestStore(t)

	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES ('k', 'not json')`); err == nil {
		t.Error("expected the json_valid check to reject the value")
	}
}

func TestSearchFiltersDefaults(t *testing.T) {
	store, _ := newTestStore(t)

	got, err := store.SearchFilters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != (SearchFilters{}) {
		t.Errorf("got %+v, want defaults", got)
	}
}

func TestSearchFiltersRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	want := SearchFilters{ProviderID: "wallhaven", MinWidth: 3840, MinHeight: 2160, Orientation: images.Landscape}

	if err := store.SaveSearchFilters(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.SearchFilters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveSearchFiltersRejectsInvalid(t *testing.T) {
	store, _ := newTestStore(t)

	invalid := []SearchFilters{
		{MinWidth: -1},
		{MinHeight: maxMinSide + 1},
		{Orientation: "diagonal"},
	}
	for _, f := range invalid {
		if err := store.SaveSearchFilters(context.Background(), f); err == nil {
			t.Errorf("%+v: expected an error", f)
		}
	}
}

func TestSearchFiltersIgnoresBadStoredValues(t *testing.T) {
	tests := map[string]string{
		"wrong shape":         `{"minWidth": "wide"}`,
		"invalid orientation": `{"orientation": "diagonal"}`,
		"negative size":       `{"minWidth": -5}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			store, db := newTestStore(t)
			if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, searchFiltersKey, raw); err != nil {
				t.Fatal(err)
			}

			got, err := store.SearchFilters(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != (SearchFilters{}) {
				t.Errorf("got %+v, want defaults", got)
			}
		})
	}
}

func TestSearchFiltersReportsDatabaseFailure(t *testing.T) {
	store, db := newTestStore(t)
	db.Close()

	if _, err := store.SearchFilters(context.Background()); err == nil {
		t.Error("expected an error from a closed database")
	}
}
