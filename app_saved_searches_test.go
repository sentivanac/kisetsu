package main

import (
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/searches"
)

func TestSavedSearchesWorkflow(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	q := searches.Query{Text: "misty mountains -snow", ProviderIDs: []string{"stub"}, MinWidth: 1920, MinHeight: 1080}

	saved, err := app.SaveSearch("Misty mountains", q)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.RenameSavedSearch(saved.ID, "Mountains in fog"); err != nil {
		t.Fatal(err)
	}
	list, err := app.ListSavedSearches()
	if err != nil || len(list) != 1 || list[0].Name != "Mountains in fog" || list[0].Query.Text != q.Text {
		t.Errorf("list = %+v, %v", list, err)
	}
	if err := app.DeleteSavedSearch(saved.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := app.ListSavedSearches(); len(list) != 0 {
		t.Errorf("after delete: %+v", list)
	}
}

func TestSavedSearchErrors(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	q := searches.Query{ProviderIDs: []string{"stub"}}
	app.SaveSearch("Lakes", q)

	tests := map[string]struct {
		err error
		msg string
	}{
		"taken":      {func() error { _, err := app.SaveSearch("LAKES", q); return err }(), "already exists"},
		"no sources": {func() error { _, err := app.SaveSearch("X", searches.Query{Text: "x"}); return err }(), "at least one source"},
		"empty name": {func() error { _, err := app.SaveSearch(" ", q); return err }(), "1 to 100"},
		"missing":    {app.DeleteSavedSearch(999), "no longer exists"},
	}
	for name, tt := range tests {
		pub := apperr.ToPublic(tt.err)
		if pub.Code != apperr.InvalidInput || !strings.Contains(pub.Message, tt.msg) {
			t.Errorf("%s: %q %q, want message containing %q", name, pub.Code, pub.Message, tt.msg)
		}
	}
}
