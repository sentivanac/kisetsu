package wallhaven

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

const testUserAgent = "Kisetsu/test"

// newTestProvider returns a provider talking to a local server instead of
// wallhaven.cc. Tests never use the network.
func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p := New(srv.Client(), testUserAgent)
	p.baseURL = srv.URL
	p.limiter = rate.NewLimiter(rate.Inf, 1)
	return p
}

// serveFixture answers every request with testdata/search.json, a trimmed
// real response with three extra entries: a sketchy one, one without a
// download URL and one whose source is not a URL.
func serveFixture(t *testing.T) http.HandlerFunc {
	t.Helper()
	data, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func ids(page providers.Page) []string {
	var out []string
	for _, img := range page.Images {
		out = append(out, img.ProviderID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNew(t *testing.T) {
	p := New(http.DefaultClient, testUserAgent)

	if p.ID() != "wallhaven" || p.Name() != "Wallhaven" {
		t.Errorf("ID, Name = %q, %q", p.ID(), p.Name())
	}
	if p.baseURL != "https://wallhaven.cc/api/v1" {
		t.Errorf("baseURL = %q", p.baseURL)
	}
	var _ providers.Provider = p
	var c providers.Categorizer = p
	if !c.HasCategories() {
		t.Error("HasCategories = false")
	}
}

func TestSearchNormalizesResults(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Text: "japan village", Page: 1})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"g751xq", "j3wlqw", "l3dz22", "badsrc1"}; !equal(ids(page), want) {
		t.Errorf("IDs = %v, want %v", ids(page), want)
	}
	if page.Page != 1 || !page.HasMore {
		t.Errorf("Page, HasMore = %d, %v; want 1, true", page.Page, page.HasMore)
	}

	got := page.Images[0]
	want := images.Image{
		Provider:     "wallhaven",
		ProviderID:   "g751xq",
		PageURL:      "https://wallhaven.cc/w/g751xq",
		ThumbnailURL: "https://th.wallhaven.cc/small/g7/g751xq.jpg",
		PreviewURL:   "https://th.wallhaven.cc/orig/g7/g751xq.jpg",
		DownloadURL:  "https://w.wallhaven.cc/full/g7/wallhaven-g751xq.jpg",
		Width:        5070,
		Height:       3380,
		MIMEType:     "image/jpeg",
		FileSize:     1208033,
		License:      images.License{Status: images.LicenseUnknown, Notice: licenseNotice},
	}
	if got != want {
		t.Errorf("image =\n%+v\nwant\n%+v", got, want)
	}

	for _, img := range page.Images {
		if err := img.Validate(); err != nil {
			t.Errorf("returned an invalid image: %v", err)
		}
	}
}

func TestSearchKeepsOnlyURLSources(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, img := range page.Images {
		sources[img.ProviderID] = img.SourceURL
	}
	if got := sources["l3dz22"]; got != "https://www.artstation.com/artwork/g8g29Q" {
		t.Errorf("source URL = %q", got)
	}
	if got := sources["badsrc1"]; got != "" {
		t.Errorf("non-URL source was kept: %q", got)
	}
}

func TestSearchNeverReturnsNonSFW(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, img := range page.Images {
		if img.ProviderID == "sketch1" {
			t.Fatal("a sketchy result was returned")
		}
	}
}

func TestSearchAppliesFiltersLocally(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))
	ctx := context.Background()

	page, err := p.Search(ctx, providers.Query{Page: 1, MinWidth: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"g751xq", "badsrc1"}; !equal(ids(page), want) {
		t.Errorf("MinWidth 3000: IDs = %v, want %v", ids(page), want)
	}

	page, err = p.Search(ctx, providers.Query{Page: 1, Orientation: images.Square})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Images) != 0 {
		t.Errorf("square filter returned %v", ids(page))
	}
}

func TestSearchRequest(t *testing.T) {
	tests := []struct {
		name  string
		query providers.Query
		want  url.Values
	}{
		{
			name:  "text search",
			query: providers.Query{Text: "  misty mountains ", Page: 2},
			want: url.Values{
				"q": {"misty mountains"}, "sorting": {"relevance"}, "page": {"2"},
				"categories": {"111"}, "purity": {"100"},
			},
		},
		{
			name:  "excluded terms from text and field",
			query: providers.Query{Text: "mountains -snow", Exclude: []string{"anime"}, Page: 1},
			want: url.Values{
				"q": {"mountains -anime -snow"}, "sorting": {"relevance"}, "page": {"1"},
				"categories": {"111"}, "purity": {"100"},
			},
		},
		{
			name:  "only excluded terms",
			query: providers.Query{Text: "-snow", Page: 1},
			want: url.Values{
				"q": {"-snow"}, "sorting": {"toplist"}, "page": {"1"},
				"categories": {"111"}, "purity": {"100"},
			},
		},
		{
			name:  "browse without text",
			query: providers.Query{Page: 1},
			want:  url.Values{"sorting": {"toplist"}, "page": {"1"}, "categories": {"111"}, "purity": {"100"}},
		},
		{
			name:  "filters",
			query: providers.Query{Page: 1, MinWidth: 1920, MinHeight: 1080, Orientation: images.Portrait},
			want: url.Values{
				"sorting": {"toplist"}, "page": {"1"}, "categories": {"111"}, "purity": {"100"},
				"atleast": {"1920x1080"}, "ratios": {"portrait"},
			},
		},
		{
			name:  "hidden tags, one quoted",
			query: providers.Query{Text: "forest", Page: 1, HiddenTags: []string{"barefoot", "long hair"}},
			want: url.Values{
				"q": {`forest -barefoot -"long hair"`}, "sorting": {"relevance"}, "page": {"1"},
				"categories": {"111"}, "purity": {"100"},
			},
		},
		{
			name:  "categories",
			query: providers.Query{Page: 1, Categories: []images.Category{images.People, images.General}},
			want:  url.Values{"sorting": {"toplist"}, "page": {"1"}, "categories": {"101"}, "purity": {"100"}},
		},
		{
			name:  "only general",
			query: providers.Query{Page: 1, Categories: []images.Category{images.General}},
			want:  url.Values{"sorting": {"toplist"}, "page": {"1"}, "categories": {"100"}, "purity": {"100"}},
		},
		{
			name:  "square is filtered locally",
			query: providers.Query{Page: 1, Orientation: images.Square},
			want:  url.Values{"sorting": {"toplist"}, "page": {"1"}, "categories": {"111"}, "purity": {"100"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *http.Request
			fixture := serveFixture(t)
			p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				got = r
				fixture(w, r)
			})

			if _, err := p.Search(context.Background(), tt.query); err != nil {
				t.Fatal(err)
			}

			if got.URL.Path != "/search" {
				t.Errorf("path = %q, want /search", got.URL.Path)
			}
			if q := got.URL.Query(); q.Encode() != tt.want.Encode() {
				t.Errorf("query = %s\nwant    %s", q.Encode(), tt.want.Encode())
			}
			if ua := got.Header.Get("User-Agent"); ua != testUserAgent {
				t.Errorf("User-Agent = %q", ua)
			}
		})
	}
}

func TestSearchErrors(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		kind       error
		retryAfter time.Duration
	}{
		{
			name: "rate limited with Retry-After",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			kind:       providers.ErrRateLimited,
			retryAfter: 30 * time.Second,
		},
		{
			name:       "rate limited without Retry-After",
			handler:    func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) },
			kind:       providers.ErrRateLimited,
			retryAfter: time.Minute,
		},
		{
			name:    "server error",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) },
			kind:    providers.ErrUnavailable,
		},
		{
			name:    "bad request",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) },
			kind:    providers.ErrInvalidQuery,
		},
		{
			name:    "malformed JSON",
			handler: func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data": [`)) },
			kind:    providers.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProvider(t, tt.handler)

			_, err := p.Search(context.Background(), providers.Query{Page: 1})

			if !errors.Is(err, tt.kind) {
				t.Fatalf("err = %v, want kind %v", err, tt.kind)
			}
			pe, ok := errors.AsType[*providers.Error](err)
			if !ok || pe.Provider != ID {
				t.Fatalf("err = %#v, want *providers.Error from %s", err, ID)
			}
			if pe.RetryAfter != tt.retryAfter {
				t.Errorf("RetryAfter = %v, want %v", pe.RetryAfter, tt.retryAfter)
			}
		})
	}
}

func TestSearchRejectsInvalidQueryWithoutRequest(t *testing.T) {
	var calls atomic.Int32
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })

	_, err := p.Search(context.Background(), providers.Query{Page: 0})

	if !errors.Is(err, providers.ErrInvalidQuery) {
		t.Errorf("err = %v, want ErrInvalidQuery", err)
	}
	if calls.Load() != 0 {
		t.Error("an invalid query was sent to the server")
	}
}

func TestSearchHonoursCancellation(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Search(ctx, providers.Query{Page: 1})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestSearchWaitsForRateLimiter(t *testing.T) {
	var calls atomic.Int32
	fixture := serveFixture(t)
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fixture(w, r)
	})
	p.limiter = rate.NewLimiter(rate.Every(time.Hour), 1) // one request, then wait

	if _, err := p.Search(context.Background(), providers.Query{Page: 1}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Search(ctx, providers.Query{Page: 2})

	if !errors.Is(err, providers.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable", err)
	}
	if calls.Load() != 1 {
		t.Errorf("server received %d requests, want 1", calls.Load())
	}
}

func TestTags(t *testing.T) {
	var path string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"vp3m23","tags":[{"id":863,"name":"barefoot"},` +
			`{"id":169,"name":"long  hair"},{"id":1,"name":"Barefoot"}]}}`))
	})
	var _ providers.Tagger = p

	tags, err := p.Tags(context.Background(), "vp3m23")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/w/vp3m23" {
		t.Errorf("path = %q", path)
	}
	if want := []string{"barefoot", "long hair"}; !slices.Equal(tags, want) {
		t.Errorf("tags = %q, want %q", tags, want)
	}
}

func TestTagsErrors(t *testing.T) {
	requests := 0
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
	})
	for _, id := range []string{"", "../search", "ABC", "a b"} {
		if _, err := p.Tags(context.Background(), id); !errors.Is(err, providers.ErrInvalidQuery) {
			t.Errorf("%q: err = %v, want ErrInvalidQuery", id, err)
		}
	}
	if requests != 0 {
		t.Errorf("%d requests for invalid IDs", requests)
	}
	if _, err := p.Tags(context.Background(), "abc123"); !errors.Is(err, providers.ErrUnavailable) {
		t.Errorf("HTTP 404: err = %v, want ErrUnavailable", err)
	}
}
