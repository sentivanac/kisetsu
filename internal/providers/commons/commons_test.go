package commons

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

// serveFixture answers with testdata/search.json: four real results from a
// live search plus four crafted ones (a risky category, a risky
// description, a TIFF file and a file without license metadata), shuffled
// to check that results are sorted by search rank.
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

func byID(page providers.Page, id string) images.Image {
	for _, img := range page.Images {
		if img.ProviderID == id {
			return img
		}
	}
	return images.Image{}
}

func TestNew(t *testing.T) {
	p := New(http.DefaultClient, testUserAgent)

	if p.ID() != "wikimedia-commons" || p.Name() != "Wikimedia Commons" {
		t.Errorf("ID, Name = %q, %q", p.ID(), p.Name())
	}
	var _ providers.Provider = p
	if w, ok := any(p).(providers.ContentWarner); !ok || w.ContentWarning() == "" {
		t.Error("Commons must warn that its content filter is best effort")
	}
}

func TestSearchNormalizesResults(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Text: "mountains", Page: 1})
	if err != nil {
		t.Fatal(err)
	}

	// Sorted by rank; the risky, TIFF results are dropped.
	want := []string{"125216708", "125747465", "113030211", "134682226", "900004"}
	if !slices.Equal(ids(page), want) {
		t.Errorf("IDs = %v, want %v", ids(page), want)
	}
	if page.Page != 1 || !page.HasMore {
		t.Errorf("Page, HasMore = %d, %v; want 1, true", page.Page, page.HasMore)
	}

	got := page.Images[0]
	thumbBase := "https://thumb.wikimedia.org/wikipedia/commons/thumb/"
	checks := map[string][2]string{
		"PageURL":     {got.PageURL, "https://commons.wikimedia.org/wiki/File:Himalayas,_Ama_Dablam,_Nepal.jpg"},
		"AuthorName":  {got.Author.Name, "Vyacheslav Argenberg"},
		"AuthorURL":   {got.Author.URL, "https://commons.wikimedia.org/wiki/User:Argenberg"},
		"LicenseName": {got.License.Name, "CC BY 4.0"},
		"LicenseURL":  {got.License.URL, "https://creativecommons.org/licenses/by/4.0"},
		"MIMEType":    {got.MIMEType, "image/jpeg"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if got.License.Status != images.LicenseKnown || !strings.Contains(got.License.Notice, "Attribution required") {
		t.Errorf("License = %+v", got.License)
	}
	if !strings.HasPrefix(got.ThumbnailURL, thumbBase) || !strings.Contains(got.ThumbnailURL, "/500px-") {
		t.Errorf("ThumbnailURL = %q", got.ThumbnailURL)
	}
	if !strings.Contains(got.PreviewURL, "/1280px-") {
		t.Errorf("PreviewURL = %q", got.PreviewURL)
	}
	if !strings.HasPrefix(got.DownloadURL, "https://upload.wikimedia.org/") {
		t.Errorf("DownloadURL = %q", got.DownloadURL)
	}
	if got.Width != 4032 || got.Height != 2688 || got.FileSize == 0 {
		t.Errorf("size = %dx%d, %d bytes", got.Width, got.Height, got.FileSize)
	}

	for _, img := range page.Images {
		if err := img.Validate(); err != nil {
			t.Errorf("returned an invalid image: %v", err)
		}
	}
}

func TestSearchConvertsAuthorHTML(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Page: 1})
	if err != nil {
		t.Fatal(err)
	}

	derivative := byID(page, "113030211").Author
	if derivative.Name != "Florian Kriechbaumer" || derivative.Credit != "Original: Florian Kriechbaumer Derivative work: Aristeas" {
		t.Errorf("derivative author = %+v", derivative)
	}
	// The original author's user page does not exist (redlink): no URL
	// rather than a link to someone else.
	if derivative.URL != "" {
		t.Errorf("derivative author URL = %q", derivative.URL)
	}

	unlabelled := byID(page, "900004")
	if unlabelled.Author.Name != "Some & Body" || unlabelled.Author.URL != "" {
		t.Errorf("author = %+v, want name with decoded entity and no URL", unlabelled.Author)
	}
	if unlabelled.License.Status != images.LicenseUnknown || unlabelled.License.Notice == "" {
		t.Errorf("license = %+v, want unknown with a notice", unlabelled.License)
	}
}

func TestSearchDropsUnsuitableResults(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"900001", "900002", "900003"} {
		if byID(page, id).ProviderID != "" {
			t.Errorf("result %s should have been dropped", id)
		}
	}
}

func TestSearchAppliesOrientationLocally(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))

	page, err := p.Search(context.Background(), providers.Query{Page: 1, Orientation: images.Portrait})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Images) != 0 {
		t.Errorf("portrait filter returned %v", ids(page))
	}
}

func TestSearchRequest(t *testing.T) {
	var got *http.Request
	fixture := serveFixture(t)
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		fixture(w, r)
	})

	q := providers.Query{Text: "misty mountains -snow", Page: 3, MinWidth: 1920, MinHeight: 1080}
	if _, err := p.Search(context.Background(), q); err != nil {
		t.Fatal(err)
	}

	params := got.URL.Query()
	wantSearch := "misty mountains filetype:bitmap " + curated +
		" filew:>1919 fileh:>1079 -snow -nude -naked -nudity -erotic"
	if s := params.Get("gsrsearch"); s != wantSearch {
		t.Errorf("gsrsearch = %q\nwant        %q", s, wantSearch)
	}
	expect := map[string]string{
		"gsroffset": "48", "gsrlimit": "24", "gsrnamespace": "6", "maxlag": "5",
		"generator": "search", "iiurlwidth": "500", "formatversion": "2",
	}
	for k, v := range expect {
		if params.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, params.Get(k), v)
		}
	}
	if ua := got.Header.Get("User-Agent"); ua != testUserAgent {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestSearchWithoutContinueHasNoMore(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"batchcomplete": true, "query": {"pages": []}}`))
	})

	page, err := p.Search(context.Background(), providers.Query{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.HasMore || len(page.Images) != 0 {
		t.Errorf("page = %+v, want empty without more", page)
	}
}

func TestSearchWithoutRequest(t *testing.T) {
	tests := map[string]struct {
		query   providers.Query
		wantErr error
	}{
		"too long for Commons": {providers.Query{Text: strings.Repeat("word ", 50), Page: 1}, providers.ErrInvalidQuery},
		"invalid query":        {providers.Query{Page: 0}, providers.ErrInvalidQuery},
		"beyond deepest page":  {providers.Query{Page: maxOffset/pageSize + 2}, nil},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })

			page, err := p.Search(context.Background(), tt.query)

			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (page.HasMore || len(page.Images) != 0) {
				t.Errorf("page = %+v, want empty", page)
			}
			if calls.Load() != 0 {
				t.Error("a request was sent")
			}
		})
	}
}

func TestSearchErrors(t *testing.T) {
	apiError := func(code string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "5")
			w.Write([]byte(`{"error": {"code": "` + code + `", "info": "details"}}`))
		}
	}
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		kind       error
		retryAfter time.Duration
	}{
		{"maxlag", apiError("maxlag"), providers.ErrUnavailable, 5 * time.Second},
		{"API rate limit", apiError("ratelimited"), providers.ErrRateLimited, 5 * time.Second},
		{"query too long", apiError("cirrussearch-query-too-long-with-exemptions"), providers.ErrInvalidQuery, 0},
		{"other API error", apiError("internal_api_error_DBQueryError"), providers.ErrUnavailable, 0},
		{
			"HTTP 429",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTooManyRequests) },
			providers.ErrRateLimited, defaultRetryAfter,
		},
		{
			"HTTP 503",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) },
			providers.ErrUnavailable, 0,
		},
		{
			"malformed JSON",
			func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"query": `)) },
			providers.ErrUnavailable, 0,
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

func TestSearchHonoursCancellation(t *testing.T) {
	p := newTestProvider(t, serveFixture(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := p.Search(ctx, providers.Query{Page: 1}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestLooksUnsafe(t *testing.T) {
	unsafe := []string{"Category:Nude sculptures", "Topless woman", "NAKED-eye astronomy? no: naked", "erotic art"}
	for _, s := range unsafe {
		if !looksUnsafe(s) {
			t.Errorf("looksUnsafe(%q) = false", s)
		}
	}
	safe := []string{"Essex coastline", "Sussex downs", "Middlesex", "Mountains of Nepal", ""}
	for _, s := range safe {
		if looksUnsafe(s) {
			t.Errorf("looksUnsafe(%q) = true", s)
		}
	}
}

func TestParseHTML(t *testing.T) {
	tests := []struct {
		in    string
		text  string
		links []link
	}{
		{`plain &amp; simple`, "plain & simple", nil},
		{`<a href="//commons.wikimedia.org/wiki/User:A">A</a>`, "A",
			[]link{{href: "https://commons.wikimedia.org/wiki/User:A", text: "A"}}},
		{`<a href="javascript:alert(1)">bad</a>`, "bad", []link{{text: "bad"}}},
		{`<a href="/wiki/Relative">rel</a>`, "rel", []link{{text: "rel"}}},
		{`<ul><li>One</li><li>Two</li></ul>`, "One Two", nil},
		{`<a href="https://x.org/a">Some</a>thing`, "Something", []link{{href: "https://x.org/a", text: "Some"}}},
		{`<a href="https://x.org/w?title=User:B&amp;redlink=1">B</a>`, "B",
			[]link{{href: "https://x.org/w?title=User:B&redlink=1", text: "B", redlink: true}}},
		{`<b>unclosed`, "unclosed", nil},
	}
	for _, tt := range tests {
		text, links := parseHTML(tt.in)
		if text != tt.text || !slices.Equal(links, tt.links) {
			t.Errorf("parseHTML(%q) = %q, %+v; want %q, %+v", tt.in, text, links, tt.text, tt.links)
		}
	}
}

func TestParseArtist(t *testing.T) {
	tests := []struct {
		name                  string
		in                    string
		wantName, wantProfile string
		wantCredit            string
	}{
		{
			name:        "plain user link",
			in:          `<a href="//commons.wikimedia.org/wiki/User:Argenberg" title="User:Argenberg">Vyacheslav Argenberg</a>`,
			wantName:    "Vyacheslav Argenberg",
			wantProfile: "https://commons.wikimedia.org/wiki/User:Argenberg",
		},
		{
			name: "derivative work with a missing user page",
			in: `<ul><li><b>Original: </b> <a href="//commons.wikimedia.org/w/index.php?title=User:Florian_Kriechbaumer&amp;action=edit&amp;redlink=1" class="new">Florian Kriechbaumer</a></li>
<li><b>Derivative work: </b> <a href="//commons.wikimedia.org/wiki/User:Aristeas">Aristeas</a></li></ul>`,
			wantName:   "Florian Kriechbaumer",
			wantCredit: "Original: Florian Kriechbaumer Derivative work: Aristeas",
		},
		{
			name:        "creator page",
			in:          `<a href="//commons.wikimedia.org/wiki/Creator:Katsushika_Hokusai">Katsushika Hokusai</a> (1760–1849)`,
			wantName:    "Katsushika Hokusai",
			wantProfile: "https://commons.wikimedia.org/wiki/Creator:Katsushika_Hokusai",
			wantCredit:  "Katsushika Hokusai (1760–1849)",
		},
		{
			name:        "Wikipedia article, after a file link",
			in:          `<a href="//commons.wikimedia.org/wiki/File:Icon.svg"><img></a> <a href="https://en.wikipedia.org/wiki/Hiroshige">Hiroshige</a>`,
			wantName:    "Hiroshige",
			wantProfile: "https://en.wikipedia.org/wiki/Hiroshige",
		},
		{name: "unknown author as text", in: `Unknown author`, wantName: "Unknown author"},
		{
			name:       "long text without a person link",
			in:         strings.Repeat("Please credit the photographer. ", 5),
			wantCredit: strings.TrimSpace(strings.Repeat("Please credit the photographer. ", 5)),
		},
		{name: "empty", in: ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, profile, credit := parseArtist(tt.in)
			if name != tt.wantName || profile != tt.wantProfile || credit != tt.wantCredit {
				t.Errorf("got name %q, profile %q, credit %q\nwant name %q, profile %q, credit %q",
					name, profile, credit, tt.wantName, tt.wantProfile, tt.wantCredit)
			}
		})
	}
}

// The Artist field of File:Giorgio_Moroder.jpg, saved from the live API:
// a template with icons and usage conditions, where the first link is an
// icon and the second the file itself.
func TestParseArtistTemplateWithConditions(t *testing.T) {
	data, err := os.ReadFile("testdata/artist_moroder.html")
	if err != nil {
		t.Fatal(err)
	}

	name, profile, credit := parseArtist(string(data))

	if name != "Wolfgang Moroder" || profile != "https://commons.wikimedia.org/wiki/User:Moroder" {
		t.Errorf("name %q, profile %q", name, profile)
	}
	for _, part := range []string{"This Photo was taken by Wolfgang Moroder", "social media", "derivative or extract"} {
		if !strings.Contains(credit, part) {
			t.Errorf("credit does not contain %q:\n%s", part, credit)
		}
	}
	if strings.ContainsAny(credit, "<>") {
		t.Errorf("credit contains markup: %s", credit)
	}
}

func TestThumbAtWidth(t *testing.T) {
	thumb := "https://thumb.wikimedia.org/wikipedia/commons/thumb/d/dd/A_%28b%29.jpg/500px-A_%28b%29.jpg?utm_source=x"
	ii := imageInfo{Width: 4000, URL: "https://upload.wikimedia.org/wikipedia/commons/d/dd/A_%28b%29.jpg", ThumbURL: thumb}

	want := "https://thumb.wikimedia.org/wikipedia/commons/thumb/d/dd/A_%28b%29.jpg/1280px-A_%28b%29.jpg?utm_source=x"
	if got := thumbAtWidth(ii, 1280); got != want {
		t.Errorf("thumbAtWidth = %q\nwant          %q", got, want)
	}

	small := ii
	small.Width = 1000
	if got := thumbAtWidth(small, 1280); got != ii.URL {
		t.Errorf("narrow image: got %q, want the original", got)
	}

	noThumb := ii
	noThumb.ThumbURL = "https://example.org/other.jpg"
	if got := thumbAtWidth(noThumb, 1280); got != ii.URL {
		t.Errorf("unexpected thumb URL: got %q, want the original", got)
	}
}
