package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
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
	if !reflect.DeepEqual(got, SearchFilters{}) {
		t.Errorf("got %+v, want defaults", got)
	}
}

func TestSearchFiltersRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	want := SearchFilters{ProviderIDs: []string{"wallhaven", "wikimedia-commons"}, MinWidth: 3840, MinHeight: 2160, Orientation: images.Landscape,
		Categories: []images.Category{images.General, images.People}}

	if err := store.SaveSearchFilters(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.SearchFilters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveSearchFiltersRejectsInvalid(t *testing.T) {
	store, _ := newTestStore(t)

	invalid := []SearchFilters{
		{ProviderIDs: []string{"a", "a"}},
		{ProviderIDs: []string{""}},
		{ProviderIDs: make([]string, maxProviders+1)},
		{MinWidth: -1},
		{MinHeight: maxMinSide + 1},
		{Orientation: "diagonal"},
		{Categories: []images.Category{"manga"}},
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
		"repeated source":     `{"providerIds": ["a", "a"]}`,
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
			if !reflect.DeepEqual(got, SearchFilters{}) {
				t.Errorf("got %+v, want defaults", got)
			}
		})
	}
}

func TestSearchFiltersReadsSingleSourceFormat(t *testing.T) {
	store, db := newTestStore(t)
	// Format saved before multi-source search.
	legacy := `{"providerId": "wallhaven", "minWidth": 1920, "minHeight": 1080, "orientation": "landscape"}`
	if _, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, searchFiltersKey, legacy); err != nil {
		t.Fatal(err)
	}

	got, err := store.SearchFilters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := SearchFilters{ProviderIDs: []string{"wallhaven"}, MinWidth: 1920, MinHeight: 1080, Orientation: images.Landscape}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveSearchFiltersDropsLegacyField(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()
	if err := store.SaveSearchFilters(ctx, SearchFilters{ProviderIDs: []string{"wallhaven"}}); err != nil {
		t.Fatal(err)
	}

	var raw string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, searchFiltersKey).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, `"providerId"`) {
		t.Errorf("saved value still has the legacy field: %s", raw)
	}
}

func TestSearchFiltersReportsDatabaseFailure(t *testing.T) {
	store, db := newTestStore(t)
	db.Close()

	if _, err := store.SearchFilters(context.Background()); err == nil {
		t.Error("expected an error from a closed database")
	}
}

func TestDisplaySize(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if d, err := store.Display(ctx); err != nil || d != DefaultDisplaySize {
		t.Errorf("default = %+v, %v", d, err)
	}
	if err := store.SaveDisplay(ctx, DisplaySize{Width: 3840, Height: 2160}); err != nil {
		t.Fatal(err)
	}
	if d, _ := store.Display(ctx); d != (DisplaySize{Width: 3840, Height: 2160}) {
		t.Errorf("saved = %+v", d)
	}
	for _, bad := range []DisplaySize{{0, 1080}, {1920, -1}, {20000, 1080}} {
		if err := store.SaveDisplay(ctx, bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestHiddenTags(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if tags, err := store.HiddenTags(ctx); err != nil || tags != nil {
		t.Fatalf("defaults: %q, %v", tags, err)
	}
	store.SetTagHidden(ctx, "barefoot", true)
	store.SetTagHidden(ctx, " long  hair ", true)
	tags, err := store.SetTagHidden(ctx, "BAREFOOT", true) // already hidden
	if err != nil || !slices.Equal(tags, []string{"barefoot", "long hair"}) {
		t.Errorf("after hiding: %q, %v", tags, err)
	}
	tags, err = store.SetTagHidden(ctx, "Barefoot", false)
	if err != nil || !slices.Equal(tags, []string{"long hair"}) {
		t.Errorf("after showing: %q, %v", tags, err)
	}
	if got, _ := store.HiddenTags(ctx); !slices.Equal(got, []string{"long hair"}) {
		t.Errorf("stored: %q", got)
	}

	if _, err := store.SetTagHidden(ctx, `a "quoted" tag`, true); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("quoted tag: err = %v, want ErrInvalidTag", err)
	}
}

func TestHiddenTagsLimit(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for i := range providers.MaxHiddenTags {
		if _, err := store.SetTagHidden(ctx, fmt.Sprintf("tag%d", i), true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.SetTagHidden(ctx, "one more", true); !errors.Is(err, ErrInvalidTag) {
		t.Errorf("over the limit: err = %v, want ErrInvalidTag", err)
	}
	// Showing a tag again always works, even at the limit.
	if tags, err := store.SetTagHidden(ctx, "tag0", false); err != nil || len(tags) != providers.MaxHiddenTags-1 {
		t.Errorf("showing at the limit: %d tags, %v", len(tags), err)
	}
}
