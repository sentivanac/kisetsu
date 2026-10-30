package search

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

func imgs(provider string, ids ...string) []images.Image {
	out := make([]images.Image, len(ids))
	for i, id := range ids {
		out[i] = images.Image{Provider: provider, ProviderID: id}
	}
	return out
}

func resultIDs(r MultiResult) []string {
	var out []string
	for _, img := range r.Images {
		out = append(out, img.Provider+"/"+img.ProviderID)
	}
	return out
}

func TestInterleave(t *testing.T) {
	pages := []providers.Page{
		{Images: imgs("a", "1", "2", "3")},
		{Images: nil},
		{Images: imgs("c", "1")},
	}
	got := interleave(pages)

	var ids []string
	for _, img := range got {
		ids = append(ids, img.Provider+"/"+img.ProviderID)
	}
	if want := []string{"a/1", "c/1", "a/2", "a/3"}; !slices.Equal(ids, want) {
		t.Errorf("interleave = %v, want %v", ids, want)
	}
}

func TestSearchManyCombinesSources(t *testing.T) {
	a := &fakeProvider{id: "a", name: "A", page: providers.Page{Images: imgs("a", "1", "2"), HasMore: true}}
	b := &fakeProvider{id: "b", name: "B", page: providers.Page{Images: imgs("b", "1")}}
	s, _ := New(discardLogger(), a, b)

	got, err := s.SearchMany(context.Background(),
		[]SourcePage{{ProviderID: "a", Page: 2}, {ProviderID: "b", Page: 1}},
		providers.Query{Text: "lake -snow"})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"a/1", "b/1", "a/2"}; !slices.Equal(resultIDs(got), want) {
		t.Errorf("images = %v, want %v", resultIDs(got), want)
	}
	wantOutcomes := []SourceOutcome{
		{ProviderID: "a", Page: 2, HasMore: true, Count: 2},
		{ProviderID: "b", Page: 1, HasMore: false, Count: 1},
	}
	if !slices.Equal(got.Sources, wantOutcomes) {
		t.Errorf("sources = %+v, want %+v", got.Sources, wantOutcomes)
	}
	// Each provider gets its own page and the normalized query.
	if a.lastQ.Page != 2 || b.lastQ.Page != 1 || a.lastQ.Text != "lake" || !slices.Equal(a.lastQ.Exclude, []string{"snow"}) {
		t.Errorf("queries: a=%+v b=%+v", a.lastQ, b.lastQ)
	}
}

func TestSearchManyKeepsWorkingSources(t *testing.T) {
	failure := &providers.Error{Provider: "broken", Kind: providers.ErrUnavailable}
	ok := &fakeProvider{id: "ok", name: "OK", page: providers.Page{Images: imgs("ok", "1"), HasMore: true}}
	broken := &fakeProvider{id: "broken", name: "Broken", err: failure, page: providers.Page{HasMore: true}}
	s, _ := New(discardLogger(), ok, broken)

	got, err := s.SearchMany(context.Background(),
		[]SourcePage{{ProviderID: "broken", Page: 1}, {ProviderID: "ok", Page: 1}}, providers.Query{})
	if err != nil {
		t.Fatalf("a failing source must not fail the search: %v", err)
	}

	if want := []string{"ok/1"}; !slices.Equal(resultIDs(got), want) {
		t.Errorf("images = %v, want %v", resultIDs(got), want)
	}
	if out := got.Sources[0]; !errors.Is(out.Err, providers.ErrUnavailable) || out.HasMore || out.Count != 0 {
		t.Errorf("broken outcome = %+v", out)
	}
	if out := got.Sources[1]; out.Err != nil || !out.HasMore {
		t.Errorf("ok outcome = %+v", out)
	}
}

func TestSearchManyRejectsInvalidRequests(t *testing.T) {
	tests := map[string]struct {
		sources []SourcePage
		query   providers.Query
		want    error
	}{
		"no sources":       {nil, providers.Query{}, ErrNoSources},
		"unknown provider": {[]SourcePage{{ProviderID: "nope", Page: 1}}, providers.Query{}, ErrUnknownProvider},
		"repeated source":  {[]SourcePage{{ProviderID: "a", Page: 1}, {ProviderID: "a", Page: 2}}, providers.Query{}, ErrDuplicateSource},
		"invalid page":     {[]SourcePage{{ProviderID: "a", Page: 1}, {ProviderID: "b", Page: 0}}, providers.Query{}, providers.ErrInvalidQuery},
		"invalid query": {
			[]SourcePage{{ProviderID: "a", Page: 1}}, providers.Query{Orientation: "diagonal"}, providers.ErrInvalidQuery,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			a := &fakeProvider{id: "a", name: "A"}
			b := &fakeProvider{id: "b", name: "B"}
			s, _ := New(discardLogger(), a, b)

			_, err := s.SearchMany(context.Background(), tt.sources, tt.query)

			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if a.calls+b.calls != 0 {
				t.Error("a provider was called for an invalid request")
			}
		})
	}
}

// meetingProvider blocks until every other meetingProvider has started,
// so a test only finishes if the providers run concurrently.
type meetingProvider struct {
	id      string
	arrived chan struct{}
	all     []chan struct{}
}

func (m *meetingProvider) ID() string   { return m.id }
func (m *meetingProvider) Name() string { return m.id }

func (m *meetingProvider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	close(m.arrived)
	for _, ch := range m.all {
		select {
		case <-ch:
		case <-ctx.Done():
			return providers.Page{}, &providers.Error{Provider: m.id, Kind: providers.ErrUnavailable, Err: ctx.Err()}
		}
	}
	return providers.Page{Images: imgs(m.id, "1")}, nil
}

func TestSearchManyRunsSourcesConcurrently(t *testing.T) {
	chans := []chan struct{}{make(chan struct{}), make(chan struct{})}
	a := &meetingProvider{id: "a", arrived: chans[0], all: chans}
	b := &meetingProvider{id: "b", arrived: chans[1], all: chans}
	s, _ := New(discardLogger(), a, b)
	s.timeout = 2 * time.Second

	got, err := s.SearchMany(context.Background(),
		[]SourcePage{{ProviderID: "a", Page: 1}, {ProviderID: "b", Page: 1}}, providers.Query{})
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range got.Sources {
		if out.Err != nil {
			t.Errorf("%s: %v (sources did not run concurrently)", out.ProviderID, out.Err)
		}
	}
}
