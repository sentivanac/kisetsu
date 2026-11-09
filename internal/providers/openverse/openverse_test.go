package openverse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

const testUserAgent = "Kisetsu/test"

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p := New(srv.Client(), testUserAgent)
	p.baseURL = srv.URL
	p.limiter = rate.NewLimiter(rate.Inf, 1)
	return p
}

// serveFixture answers with testdata/search.json: five real results from a
// live search ("harbour" in the four collections) and five crafted ones
// (no size, a TIFF file, a risky tag, SMK with the size of the original,
// a javascript: URL).
func serveFixture(t *testing.T, got *url.Values) http.HandlerFunc {
	t.Helper()
	data, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			*got = r.URL.Query()
		}
		if r.URL.Path != "/images/" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") != testUserAgent {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

func ids(page providers.Page) []string {
	var out []string
	for _, img := range page.Images {
		out = append(out, img.ProviderID[:8])
	}
	return out
}

func byID(page providers.Page, prefix string) images.Image {
	for _, img := range page.Images {
		if strings.HasPrefix(img.ProviderID, prefix) {
			return img
		}
	}
	return images.Image{}
}

func TestNew(t *testing.T) {
	p := New(http.DefaultClient, testUserAgent)
	if p.ID() != "openverse" || p.Name() != "Openverse" {
		t.Errorf("ID, Name = %q, %q", p.ID(), p.Name())
	}
	var _ providers.Provider = p
	if w, ok := any(p).(providers.ContentWarner); !ok || w.ContentWarning() == "" {
		t.Error("Openverse must warn that its content filter is best effort")
	}
}

func TestSearchNormalizesResults(t *testing.T) {
	p := newTestProvider(t, serveFixture(t, nil))
	page, err := p.Search(context.Background(), providers.Query{Text: "harbour", Page: 1})
	if err != nil {
		t.Fatal(err)
	}

	// The crafted results are dropped, except SMK with a capped size.
	want := []string{"c6704c10", "192b5e21", "8b651297", "0799dec3", "c989a558", "00000000"}
	if got := ids(page); !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
	if page.Page != 1 || !page.HasMore {
		t.Errorf("Page, HasMore = %d, %v", page.Page, page.HasMore)
	}

	got := byID(page, "c6704c10")
	want0 := images.Image{
		Provider:     "openverse",
		ProviderID:   "c6704c10-ee44-4081-9be0-3c9851d5f279",
		PageURL:      "https://openverse.org/image/c6704c10-ee44-4081-9be0-3c9851d5f279",
		SourceURL:    "https://wordpress.org/photos/photo/831639dcb9/",
		ThumbnailURL: "https://api.openverse.org/v1/images/c6704c10-ee44-4081-9be0-3c9851d5f279/thumb/",
		PreviewURL:   "https://api.openverse.org/v1/images/c6704c10-ee44-4081-9be0-3c9851d5f279/thumb/",
		DownloadURL:  "https://pd.w.org/2022/12/831639dcb94869b07.47706358-1683x2048.jpg",
		Width:        1683,
		Height:       2048,
		MIMEType:     "image/jpeg",
		FileSize:     549600,
		Author:       images.Author{Name: "Jeremy Herve", URL: "https://herve.bzh/@jeremy/"},
		License: images.License{
			Status: images.LicenseKnown,
			Name:   "CC0 1.0",
			URL:    "https://creativecommons.org/publicdomain/zero/1.0/",
		},
	}
	if got != want0 {
		t.Errorf("first result:\n got %+v\nwant %+v", got, want0)
	}

	// No creator: the author stays empty. CC BY asks for attribution.
	mv := byID(page, "192b5e21")
	if mv.Author != (images.Author{}) {
		t.Errorf("author = %+v, want empty", mv.Author)
	}
	if mv.License.Name != "CC BY 4.0" || mv.License.Notice == "" {
		t.Errorf("license = %+v", mv.License)
	}
	if pd := byID(page, "8b651297").License; pd.Name != "Public Domain Mark 1.0" || pd.Notice != "" {
		t.Errorf("public domain license = %+v", pd)
	}

	// SMK serves at most 2048 px: 3755×2254 becomes 2048×1229.
	if smk := byID(page, "00000000"); smk.Width != 2048 || smk.Height != 1229 {
		t.Errorf("SMK size = %dx%d, want 2048x1229", smk.Width, smk.Height)
	}
}

func TestSearchRequest(t *testing.T) {
	var got url.Values
	p := newTestProvider(t, serveFixture(t, &got))
	_, err := p.Search(context.Background(), providers.Query{
		Text: "misty  mountains -snow", Page: 3, Orientation: images.Landscape,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"q":            {"misty mountains -snow -nude -naked -nudity -erotic"},
		"source":       {"rijksmuseum,museumsvictoria,wordpress,smk"},
		"page":         {"3"},
		"page_size":    {"20"},
		"mature":       {"false"},
		"aspect_ratio": {"wide"},
	}
	if got.Encode() != want.Encode() {
		t.Errorf("query:\n got %s\nwant %s", got.Encode(), want.Encode())
	}

	for o, ratio := range map[images.Orientation]string{images.Portrait: "tall", images.Square: "square", "": ""} {
		if _, err := p.Search(context.Background(), providers.Query{Page: 1, Orientation: o}); err != nil {
			t.Fatal(err)
		}
		if got.Get("aspect_ratio") != ratio {
			t.Errorf("%q: aspect_ratio = %q, want %q", o, got.Get("aspect_ratio"), ratio)
		}
	}
}

func TestSearchFiltersLocally(t *testing.T) {
	p := newTestProvider(t, serveFixture(t, nil))
	page, err := p.Search(context.Background(), providers.Query{
		Page: 1, MinWidth: 2000, MinHeight: 1100, Orientation: images.Landscape,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Only the wide WordPress photo (2048×1152) and the capped SMK image
	// (2048×1229) are wide enough and landscape.
	if got, want := ids(page), []string{"0799dec3", "00000000"}; !slices.Equal(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestSearchPages(t *testing.T) {
	var requests atomic.Int32
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte(`{"result_count":181,"page_count":10,"results":[]}`))
	})
	ctx := context.Background()

	for page, more := range map[int]bool{1: true, 9: true, 10: false} {
		got, err := p.Search(ctx, providers.Query{Page: page})
		if err != nil {
			t.Fatal(err)
		}
		if got.HasMore != more {
			t.Errorf("page %d: HasMore = %v, want %v", page, got.HasMore, more)
		}
	}

	// Openverse refuses pages after the 240th result without a key; they
	// are not requested.
	requests.Store(0)
	got, err := p.Search(ctx, providers.Query{Page: maxPage + 1})
	if err != nil || len(got.Images) != 0 || got.HasMore || requests.Load() != 0 {
		t.Errorf("page %d: %+v, %v, %d requests", maxPage+1, got, err, requests.Load())
	}
}

func TestSearchErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		header     string
		body       string
		kind       error
		retryAfter time.Duration
		message    string
	}{
		{"rate limited", http.StatusTooManyRequests, "120", "", providers.ErrRateLimited, 120 * time.Second, ""},
		{"rate limited without header", http.StatusTooManyRequests, "", "", providers.ErrRateLimited, defaultRetryAfter, ""},
		{"refused", http.StatusUnauthorized, "", `{"detail":"pagination depth may not exceed 240 for anonymous requests"}`,
			providers.ErrUnavailable, 0, "pagination depth"},
		{"server error", http.StatusBadGateway, "", "<html>", providers.ErrUnavailable, 0, "HTTP 502"},
		{"broken JSON", http.StatusOK, "", "{", providers.ErrUnavailable, 0, "decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.header != "" {
					w.Header().Set("Retry-After", tt.header)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			})
			_, err := p.Search(context.Background(), providers.Query{Page: 1})
			var pe *providers.Error
			if !errors.As(err, &pe) || !errors.Is(err, tt.kind) {
				t.Fatalf("err = %v, want %v", err, tt.kind)
			}
			if pe.Provider != ID || pe.RetryAfter != tt.retryAfter {
				t.Errorf("Provider, RetryAfter = %q, %v", pe.Provider, pe.RetryAfter)
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Errorf("err = %q, want it to mention %q", err, tt.message)
			}
		})
	}
}

func TestSearchRejectsLongQueries(t *testing.T) {
	var requests atomic.Int32
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1) })
	// With Kisetsu's own excluded words, 180 characters of text are too
	// many for Openverse's 200.
	_, err := p.Search(context.Background(), providers.Query{Text: strings.Repeat("a", 180), Page: 1})
	if !errors.Is(err, providers.ErrInvalidQuery) || requests.Load() != 0 {
		t.Errorf("err = %v after %d requests, want ErrInvalidQuery before any", err, requests.Load())
	}
}

func TestSearchHonoursCancellation(t *testing.T) {
	p := newTestProvider(t, serveFixture(t, nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Search(ctx, providers.Query{Page: 1}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestLicense(t *testing.T) {
	tests := []struct {
		code, version, url string
		want               images.License
	}{
		{"by-nc-sa", "2.0", "https://creativecommons.org/licenses/by-nc-sa/2.0/", images.License{
			Status: images.LicenseKnown, Name: "CC BY-NC-SA 2.0", URL: "https://creativecommons.org/licenses/by-nc-sa/2.0/",
			Notice: "Attribution required: credit the author and link the license when you share this image.",
		}},
		{"cc0", "1.0", "javascript:alert(1)", images.License{Status: images.LicenseKnown, Name: "CC0 1.0"}},
		{"new-license", "", "", images.License{Status: images.LicenseKnown, Name: "NEW-LICENSE"}},
		{"", "", "", images.License{
			Status: images.LicenseUnknown,
			Notice: "Openverse gives no license for this image. Check its page before reusing it.",
		}},
	}
	for _, tt := range tests {
		r := result{License: tt.code, LicenseVersion: tt.version, LicenseURL: tt.url}
		if got := r.license(); got != tt.want {
			t.Errorf("%q %q: %+v, want %+v", tt.code, tt.version, got, tt.want)
		}
	}
}
