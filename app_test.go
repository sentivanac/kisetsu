package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// stubProvider returns a fixed page or error. Its ID defaults to "stub".
// With block set it waits until its context is done; started, if set, is
// closed when Search begins.
type stubProvider struct {
	id      string
	page    providers.Page
	err     error
	block   bool
	started chan struct{}
}

func (s *stubProvider) ID() string {
	if s.id == "" {
		return "stub"
	}
	return s.id
}

func (s *stubProvider) Name() string {
	if s.id == "" {
		return "Stub Images"
	}
	return "Stub " + s.id
}

func (s *stubProvider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	if s.started != nil {
		close(s.started)
	}
	if s.block {
		<-ctx.Done()
		return providers.Page{}, &providers.Error{Provider: s.ID(), Kind: providers.ErrUnavailable, Err: ctx.Err()}
	}
	return s.page, s.err
}

func newTestApp(t *testing.T, stubs ...*stubProvider) *App {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	path := filepath.Join(t.TempDir(), database.FileName)
	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ps := make([]providers.Provider, len(stubs))
	for i, s := range stubs {
		ps[i] = s
	}
	svc, err := search.New(log, ps...)
	if err != nil {
		t.Fatal(err)
	}

	var app *App
	lib := library.NewStore(db)
	mgr, err := downloads.New(downloads.Config{
		Client:    http.DefaultClient,
		UserAgent: "Kisetsu/test",
		Root:      filepath.Dir(path),
		Store:     lib,
		Log:       log,
		Notify:    func(p downloads.Progress) { app.notifyDownload(p) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Close)
	files, err := libfiles.New(filepath.Dir(path)) // after downloads.New created the directories
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { files.Close() })

	app = NewApp(Deps{
		Log: log, DB: db, DBPath: path, Search: svc,
		Settings: settings.NewStore(db), Downloads: mgr, Library: lib, Files: files,
		Wallpaper: &fakeWallpaper{}, // never the real desktop
		Rotation: &rotation.Rotator{
			Library: lib, Settings: settings.NewStore(db), History: rotation.NewHistory(db),
			Files: files, Setter: &fakeWallpaper{}, Log: log,
		},
		Schedule:     &fakeSchedule{}, // never the real systemd
		Executable:   "/opt/kisetsu/kisetsu",
		RotationLock: filepath.Join(filepath.Dir(path), "rotate.lock"),
	})
	return app
}

// events captures what the app emits to the frontend.
type events struct {
	mu   sync.Mutex
	list []DownloadProgress
	done chan DownloadProgress
}

func captureEvents(app *App) *events {
	ev := &events{done: make(chan DownloadProgress, 8)}
	app.emit = func(name string, data any) {
		p, ok := data.(DownloadProgress)
		if name != DownloadProgressEvent || !ok {
			return
		}
		ev.mu.Lock()
		ev.list = append(ev.list, p)
		ev.mu.Unlock()
		if downloads.State(p.State).Finished() {
			ev.done <- p
		}
	}
	return ev
}

func (ev *events) wait(t *testing.T) DownloadProgress {
	t.Helper()
	select {
	case p := <-ev.done:
		return p
	case <-time.After(10 * time.Second):
		t.Fatal("download did not finish")
		return DownloadProgress{}
	}
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
	if len(list) != 1 || list[0] != (search.ProviderInfo{ID: "stub", Name: "Stub Images", EnabledByDefault: true}) {
		t.Errorf("ListProviders() = %+v", list)
	}
}

// Provider failures are reported per source with a user-friendly message.
// Request-level errors are covered by TestSearchSourcesRejectsInvalidRequests.
func TestSourceErrorsAreUserFriendly(t *testing.T) {
	providerErr := func(kind, cause error, retry time.Duration) error {
		return &providers.Error{Provider: "stub", Kind: kind, Err: cause, RetryAfter: retry}
	}

	tests := []struct {
		name          string
		err           error
		wantCode      apperr.Code
		wantInMsg     string
		wantRetrySecs int
	}{
		{
			name:     "rate limited",
			err:      providerErr(providers.ErrRateLimited, nil, 30*time.Second),
			wantCode: apperr.RateLimited, wantInMsg: "Stub Images", wantRetrySecs: 30,
		},
		{
			name:     "timeout",
			err:      providerErr(providers.ErrUnavailable, context.DeadlineExceeded, 0),
			wantCode: apperr.ProviderUnavailable, wantInMsg: "did not respond in time",
		},
		{
			name:     "network failure",
			err:      providerErr(providers.ErrUnavailable, errors.New("dial tcp: no route to host"), 0),
			wantCode: apperr.ProviderUnavailable, wantInMsg: "internet connection",
		},
		{
			name:     "rejected by the provider",
			err:      providerErr(providers.ErrInvalidQuery, errors.New("HTTP 400"), 0),
			wantCode: apperr.InvalidInput, wantInMsg: "not valid",
		},
		{
			name:     "unexpected error",
			err:      errors.New("something odd"),
			wantCode: apperr.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t, &stubProvider{err: tt.err})

			result, err := app.SearchSources([]search.SourcePage{{ProviderID: "stub", Page: 1}}, providers.Query{})
			if err != nil {
				t.Fatalf("a source failure must not fail the call: %v", err)
			}

			pub := result.Sources[0].Error
			if pub == nil {
				t.Fatal("source has no error")
			}
			if pub.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", pub.Code, tt.wantCode)
			}
			if !strings.Contains(pub.Message, tt.wantInMsg) {
				t.Errorf("message %q does not contain %q", pub.Message, tt.wantInMsg)
			}
			if pub.RetryAfterSeconds != tt.wantRetrySecs {
				t.Errorf("RetryAfterSeconds = %d, want %d", pub.RetryAfterSeconds, tt.wantRetrySecs)
			}
			for _, leak := range []string{"dial tcp", "something odd", "HTTP 400"} {
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
	if !reflect.DeepEqual(defaults, settings.SearchFilters{}) {
		t.Errorf("defaults = %+v", defaults)
	}

	want := settings.SearchFilters{ProviderIDs: []string{"stub"}, MinWidth: 2560, MinHeight: 1440, Orientation: images.Portrait}
	if err := app.SaveSearchFilters(want); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSearchFilters()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
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

func TestSearchSourcesReportsEachSource(t *testing.T) {
	good := &stubProvider{id: "good", page: providers.Page{
		Images: []images.Image{{Provider: "good", ProviderID: "1"}}, HasMore: true,
	}}
	limited := &stubProvider{id: "limited", err: &providers.Error{
		Provider: "limited", Kind: providers.ErrRateLimited, RetryAfter: 10 * time.Second,
	}}
	app := newTestApp(t, good, limited)

	result, err := app.SearchSources(
		[]search.SourcePage{{ProviderID: "good", Page: 1}, {ProviderID: "limited", Page: 1}},
		providers.Query{Text: "lake"})
	if err != nil {
		t.Fatalf("a failing source must not fail the call: %v", err)
	}

	if len(result.Images) != 1 || len(result.Sources) != 2 {
		t.Fatalf("result = %+v", result)
	}
	ok := result.Sources[0]
	if ok.Name != "Stub good" || ok.Count != 1 || !ok.HasMore || ok.Error != nil {
		t.Errorf("good source = %+v", ok)
	}
	bad := result.Sources[1]
	if bad.Error == nil || bad.Error.Code != apperr.RateLimited || bad.Error.RetryAfterSeconds != 10 {
		t.Errorf("limited source = %+v (error %+v)", bad, bad.Error)
	}
	if bad.Error != nil && !strings.Contains(bad.Error.Message, "Stub limited") {
		t.Errorf("message %q does not name the source", bad.Error.Message)
	}
}

func TestSearchSourcesRejectsInvalidRequests(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	tests := map[string][]search.SourcePage{
		"no sources":      nil,
		"unknown source":  {{ProviderID: "missing", Page: 1}},
		"repeated source": {{ProviderID: "stub", Page: 1}, {ProviderID: "stub", Page: 2}},
		"invalid page":    {{ProviderID: "stub", Page: 0}},
	}
	for name, sources := range tests {
		_, err := app.SearchSources(sources, providers.Query{})
		if got := apperr.ToPublic(err).Code; got != apperr.InvalidInput {
			t.Errorf("%s: code = %q, want %q (err: %v)", name, got, apperr.InvalidInput, err)
		}
	}
}

// runSearch calls SearchSources in a goroutine and returns its error.
func runSearch(app *App, providerID string) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := app.SearchSources([]search.SourcePage{{ProviderID: providerID, Page: 1}}, providers.Query{})
		done <- err
	}()
	return done
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitErr(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("search did not return")
		return nil
	}
}

func TestCancelSearchStopsRunningSearch(t *testing.T) {
	slow := &stubProvider{id: "slow", block: true, started: make(chan struct{})}
	app := newTestApp(t, slow)

	result := runSearch(app, "slow")
	waitFor(t, slow.started, "the search to start")

	if err := app.CancelSearch(); err != nil {
		t.Fatal(err)
	}

	err := waitErr(t, result)
	if got := apperr.ToPublic(err).Code; got != apperr.Canceled {
		t.Errorf("code = %q, want %q (err: %v)", got, apperr.Canceled, err)
	}
}

func TestNewSearchCancelsPrevious(t *testing.T) {
	slow := &stubProvider{id: "slow", block: true, started: make(chan struct{})}
	fast := &stubProvider{id: "fast", page: providers.Page{Images: []images.Image{{Provider: "fast", ProviderID: "1"}}}}
	app := newTestApp(t, slow, fast)

	first := runSearch(app, "slow")
	waitFor(t, slow.started, "the first search to start")

	second, err := app.SearchSources([]search.SourcePage{{ProviderID: "fast", Page: 1}}, providers.Query{})
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if len(second.Images) != 1 {
		t.Errorf("second search returned %d images", len(second.Images))
	}

	if got := apperr.ToPublic(waitErr(t, first)).Code; got != apperr.Canceled {
		t.Errorf("first search code = %q, want %q", got, apperr.Canceled)
	}

	// The finished search must not leave a cancel function behind that a
	// later CancelSearch could misuse.
	app.searchMu.Lock()
	leftover := app.cancelSearch != nil
	app.searchMu.Unlock()
	if leftover {
		t.Error("cancel function not cleared after the search finished")
	}
}

func TestCancelSearchWithoutSearchIsHarmless(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	if err := app.CancelSearch(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SearchSources([]search.SourcePage{{ProviderID: "stub", Page: 1}}, providers.Query{}); err != nil {
		t.Errorf("search after an idle cancel failed: %v", err)
	}
}
