package search

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

// fakeProvider records calls and returns a fixed result.
type fakeProvider struct {
	id, name string
	page     providers.Page
	err      error
	block    bool // wait until ctx is done
	calls    int
	lastQ    providers.Query
}

func (f *fakeProvider) ID() string   { return f.id }
func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	f.calls++
	f.lastQ = q
	if f.block {
		<-ctx.Done()
		return providers.Page{}, &providers.Error{Provider: f.id, Kind: providers.ErrUnavailable, Err: ctx.Err()}
	}
	return f.page, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewRejectsInvalidRegistrations(t *testing.T) {
	if _, err := New(discardLogger()); err == nil {
		t.Error("no providers: expected an error")
	}
	if _, err := New(discardLogger(), &fakeProvider{id: "", name: "Nameless"}); err == nil {
		t.Error("empty ID: expected an error")
	}
	a, b := &fakeProvider{id: "same", name: "A"}, &fakeProvider{id: "same", name: "B"}
	if _, err := New(discardLogger(), a, b); err == nil {
		t.Error("duplicate ID: expected an error")
	}
}

func TestProviders(t *testing.T) {
	s, err := New(discardLogger(), &fakeProvider{id: "b", name: "Bee"}, &fakeProvider{id: "a", name: "Ay"})
	if err != nil {
		t.Fatal(err)
	}

	got := s.Providers()
	if len(got) != 2 || got[0] != (ProviderInfo{ID: "b", Name: "Bee", EnabledByDefault: true}) ||
		got[1] != (ProviderInfo{ID: "a", Name: "Ay", EnabledByDefault: true}) {
		t.Errorf("Providers() = %+v, want registration order", got)
	}
	if s.ProviderName("a") != "Ay" || s.ProviderName("missing") != "missing" {
		t.Errorf("ProviderName: %q, %q", s.ProviderName("a"), s.ProviderName("missing"))
	}
}

// warningProvider is a fakeProvider that cannot guarantee SFW results.
type warningProvider struct{ fakeProvider }

func (w *warningProvider) ContentWarning() string { return "may contain anything" }

func TestProvidersWithContentWarningAreOptIn(t *testing.T) {
	s, err := New(discardLogger(), &fakeProvider{id: "safe", name: "Safe"},
		&warningProvider{fakeProvider{id: "risky", name: "Risky"}})
	if err != nil {
		t.Fatal(err)
	}

	got := s.Providers()
	if !got[0].EnabledByDefault || got[0].ContentWarning != "" {
		t.Errorf("safe provider = %+v", got[0])
	}
	if got[1].EnabledByDefault || got[1].ContentWarning != "may contain anything" {
		t.Errorf("risky provider = %+v", got[1])
	}
}

// categoryProvider is a fakeProvider that applies categories.
type categoryProvider struct{ fakeProvider }

func (c *categoryProvider) HasCategories() bool { return true }

func TestProvidersReportCategories(t *testing.T) {
	s, err := New(discardLogger(), &fakeProvider{id: "plain", name: "Plain"},
		&categoryProvider{fakeProvider{id: "cat", name: "Cat"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Providers(); got[0].Categories || !got[1].Categories {
		t.Errorf("Categories = %v, %v; want false, true", got[0].Categories, got[1].Categories)
	}
}

func TestSearchRoutesToProvider(t *testing.T) {
	want := providers.Page{Images: []images.Image{{ProviderID: "x"}}, Page: 1, HasMore: true}
	a := &fakeProvider{id: "a", name: "A"}
	b := &fakeProvider{id: "b", name: "B", page: want}
	s, _ := New(discardLogger(), a, b)

	got, err := s.Search(context.Background(), "b", providers.Query{Text: "lake", Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Images) != 1 || !got.HasMore {
		t.Errorf("page = %+v, want %+v", got, want)
	}
	if a.calls != 0 || b.calls != 1 || b.lastQ.Text != "lake" {
		t.Errorf("calls a=%d b=%d, query %+v", a.calls, b.calls, b.lastQ)
	}
}

func TestSearchNormalizesQuery(t *testing.T) {
	p := &fakeProvider{id: "a", name: "A"}
	s, _ := New(discardLogger(), p)

	if _, err := s.Search(context.Background(), "a", providers.Query{Text: " lake  -snow ", Page: 1}); err != nil {
		t.Fatal(err)
	}
	if p.lastQ.Text != "lake" || len(p.lastQ.Exclude) != 1 || p.lastQ.Exclude[0] != "snow" {
		t.Errorf("provider received %+v, want text \"lake\" and exclude [snow]", p.lastQ)
	}
}

func TestSearchUnknownProvider(t *testing.T) {
	s, _ := New(discardLogger(), &fakeProvider{id: "a", name: "A"})

	_, err := s.Search(context.Background(), "nope", providers.Query{Page: 1})
	if !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("err = %v, want ErrUnknownProvider", err)
	}
}

func TestSearchRejectsInvalidQueryWithoutCallingProvider(t *testing.T) {
	p := &fakeProvider{id: "a", name: "A"}
	s, _ := New(discardLogger(), p)

	_, err := s.Search(context.Background(), "a", providers.Query{Page: 0})
	if !errors.Is(err, providers.ErrInvalidQuery) {
		t.Errorf("err = %v, want ErrInvalidQuery", err)
	}
	if p.calls != 0 {
		t.Error("provider was called with an invalid query")
	}
}

func TestSearchAppliesTimeout(t *testing.T) {
	s, _ := New(discardLogger(), &fakeProvider{id: "slow", name: "Slow", block: true})
	s.timeout = 20 * time.Millisecond

	_, err := s.Search(context.Background(), "slow", providers.Query{Page: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestSearchLogsTextOnlyAtDebug(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	s, _ := New(log, &fakeProvider{id: "a", name: "A"})

	if _, err := s.Search(context.Background(), "a", providers.Query{Text: "private words", Page: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "private words") {
		t.Errorf("search text logged at info level: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "search done") {
		t.Errorf("search not logged: %q", buf.String())
	}
}

func TestSearchAddsHiddenTags(t *testing.T) {
	p := &fakeProvider{id: "w", name: "W"}
	s, err := New(discardLogger(), p)
	if err != nil {
		t.Fatal(err)
	}
	s.HiddenTags = func(context.Context) ([]string, error) { return []string{"barefoot", "long hair"}, nil }

	// Tags sent by the caller are replaced by the settings.
	if _, err := s.Search(context.Background(), "w", providers.Query{Page: 1, HiddenTags: []string{"other"}}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"barefoot", "long hair"}; !slices.Equal(p.lastQ.HiddenTags, want) {
		t.Errorf("HiddenTags = %q, want %q", p.lastQ.HiddenTags, want)
	}

	s.HiddenTags = func(context.Context) ([]string, error) { return nil, errors.New("database is locked") }
	if _, err := s.Search(context.Background(), "w", providers.Query{Page: 1}); err != nil {
		t.Errorf("search failed with unreadable hidden tags: %v", err)
	}
	if p.lastQ.HiddenTags != nil {
		t.Errorf("HiddenTags = %q, want none", p.lastQ.HiddenTags)
	}
}

// taggingProvider is a fakeProvider whose images have tags.
type taggingProvider struct {
	fakeProvider
	tags  []string
	gotID string
}

func (p *taggingProvider) Tags(ctx context.Context, id string) ([]string, error) {
	p.gotID = id
	return p.tags, nil
}

func TestTags(t *testing.T) {
	tagger := &taggingProvider{fakeProvider: fakeProvider{id: "w", name: "W"}, tags: []string{"forest"}}
	s, err := New(discardLogger(), tagger, &fakeProvider{id: "c", name: "C"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if tags, err := s.Tags(ctx, "w", "abc"); err != nil || !slices.Equal(tags, []string{"forest"}) || tagger.gotID != "abc" {
		t.Errorf("tags = %q, %v (id %q)", tags, err, tagger.gotID)
	}
	if tags, err := s.Tags(ctx, "c", "abc"); err != nil || tags != nil {
		t.Errorf("provider without tags: %q, %v", tags, err)
	}
	if info := s.Providers(); !info[0].Tags || info[1].Tags {
		t.Errorf("Tags = %v, %v; want true, false", info[0].Tags, info[1].Tags)
	}
	if _, err := s.Tags(ctx, "nope", "abc"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("unknown provider: err = %v", err)
	}
}
