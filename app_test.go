package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// stubProvider returns a fixed page or error.
type stubProvider struct {
	page providers.Page
	err  error
}

func (s *stubProvider) ID() string   { return "stub" }
func (s *stubProvider) Name() string { return "Stub Images" }

func (s *stubProvider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	return s.page, s.err
}

func newTestApp(t *testing.T, provider *stubProvider) *App {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	path := filepath.Join(t.TempDir(), database.FileName)
	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	svc, err := search.New(log, provider)
	if err != nil {
		t.Fatal(err)
	}
	return NewApp(Deps{Log: log, DB: db, DBPath: path, Search: svc, Settings: settings.NewStore(db)})
}

func TestGetStatus(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	status, err := app.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.App != appinfo.Get() {
		t.Errorf("App = %+v, want %+v", status.App, appinfo.Get())
	}
	if status.Database.Path != app.dbPath {
		t.Errorf("Database.Path = %q, want %q", status.Database.Path, app.dbPath)
	}
}

func TestGetStatusReportsUnavailableDatabase(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.db.Close()

	_, err := app.GetStatus()

	if appErr, ok := errors.AsType[*apperr.Error](err); !ok || appErr.Code != apperr.Unavailable {
		t.Fatalf("err = %v, want an %q apperr.Error", err, apperr.Unavailable)
	}
}

func TestBoundMethodRecoversPanic(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.db = nil // makes GetStatus panic inside database/sql

	_, err := app.GetStatus()

	if got := apperr.ToPublic(err).Code; got != apperr.Internal {
		t.Errorf("code = %q, want %q (err: %v)", got, apperr.Internal, err)
	}
}

func TestListProviders(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	list, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != (search.ProviderInfo{ID: "stub", Name: "Stub Images"}) {
		t.Errorf("ListProviders() = %+v", list)
	}
}

func TestSearchReturnsPage(t *testing.T) {
	want := providers.Page{Images: []images.Image{{ProviderID: "abc"}}, Page: 1, HasMore: true}
	app := newTestApp(t, &stubProvider{page: want})

	got, err := app.Search("stub", providers.Query{Text: "lake", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 1 || got.Images[0].ProviderID != "abc" || !got.HasMore {
		t.Errorf("page = %+v", got)
	}
}

func TestSearchErrorsAreUserFriendly(t *testing.T) {
	providerErr := func(kind, cause error, retry time.Duration) error {
		return &providers.Error{Provider: "stub", Kind: kind, Err: cause, RetryAfter: retry}
	}

	tests := []struct {
		name          string
		providerID    string
		query         providers.Query
		err           error
		wantCode      apperr.Code
		wantInMsg     string
		wantRetrySecs int
	}{
		{
			name: "unknown provider", providerID: "missing", query: providers.Query{Page: 1},
			wantCode: apperr.InvalidInput,
		},
		{
			name: "invalid query", providerID: "stub", query: providers.Query{Page: 0},
			wantCode: apperr.InvalidInput,
		},
		{
			name: "rate limited", providerID: "stub", query: providers.Query{Page: 1},
			err:      providerErr(providers.ErrRateLimited, nil, 30*time.Second),
			wantCode: apperr.RateLimited, wantInMsg: "Stub Images", wantRetrySecs: 30,
		},
		{
			name: "timeout", providerID: "stub", query: providers.Query{Page: 1},
			err:      providerErr(providers.ErrUnavailable, context.DeadlineExceeded, 0),
			wantCode: apperr.ProviderUnavailable, wantInMsg: "did not respond in time",
		},
		{
			name: "network failure", providerID: "stub", query: providers.Query{Page: 1},
			err:      providerErr(providers.ErrUnavailable, errors.New("dial tcp: no route to host"), 0),
			wantCode: apperr.ProviderUnavailable, wantInMsg: "internet connection",
		},
		{
			name: "unexpected error", providerID: "stub", query: providers.Query{Page: 1},
			err:      errors.New("something odd"),
			wantCode: apperr.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t, &stubProvider{err: tt.err})

			_, err := app.Search(tt.providerID, tt.query)

			pub := apperr.ToPublic(err)
			if pub.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q (err: %v)", pub.Code, tt.wantCode, err)
			}
			if !strings.Contains(pub.Message, tt.wantInMsg) {
				t.Errorf("message %q does not contain %q", pub.Message, tt.wantInMsg)
			}
			if pub.RetryAfterSeconds != tt.wantRetrySecs {
				t.Errorf("RetryAfterSeconds = %d, want %d", pub.RetryAfterSeconds, tt.wantRetrySecs)
			}
			for _, leak := range []string{"dial tcp", "something odd"} {
				if strings.Contains(pub.Message, leak) {
					t.Errorf("internal detail %q leaked into %q", leak, pub.Message)
				}
			}
		})
	}
}

func TestSearchFiltersRoundTrip(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	defaults, err := app.GetSearchFilters()
	if err != nil {
		t.Fatal(err)
	}
	if defaults != (settings.SearchFilters{}) {
		t.Errorf("defaults = %+v", defaults)
	}

	want := settings.SearchFilters{ProviderID: "stub", MinWidth: 2560, MinHeight: 1440, Orientation: images.Portrait}
	if err := app.SaveSearchFilters(want); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSearchFilters()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveSearchFiltersRejectsInvalid(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	err := app.SaveSearchFilters(settings.SearchFilters{Orientation: "diagonal"})
	if got := apperr.ToPublic(err).Code; got != apperr.InvalidInput {
		t.Errorf("code = %q, want %q", got, apperr.InvalidInput)
	}
}

func TestSearchFiltersReportUnavailableDatabase(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.db.Close()

	_, getErr := app.GetSearchFilters()
	saveErr := app.SaveSearchFilters(settings.SearchFilters{})

	for _, err := range []error{getErr, saveErr} {
		if got := apperr.ToPublic(err).Code; got != apperr.Unavailable {
			t.Errorf("code = %q, want %q (err: %v)", got, apperr.Unavailable, err)
		}
	}
}
