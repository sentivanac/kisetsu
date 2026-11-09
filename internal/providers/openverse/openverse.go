// Package openverse searches Openverse (https://openverse.org), limited to
// the collections whose files are large enough for a wallpaper (ADR 0048).
//
// API documentation: https://api.openverse.org/v1/
// Terms of service: https://docs.openverse.org/terms_of_service.html
package openverse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

const (
	// ID is the stable provider identifier.
	ID = "openverse"

	defaultBaseURL = "https://api.openverse.org/v1"
	// pageBase is the address of an image's page on openverse.org.
	pageBase = "https://openverse.org/image/"

	// pageSize is the most Openverse returns without a key.
	pageSize = 20
	// maxPage: without a key, Openverse shows at most 240 results per
	// search.
	maxPage = 240 / pageSize
	// maxSearchLength is Openverse's limit for q.
	maxSearchLength = 200

	maxResponseSize   = 4 << 20 // a page is about 40 KB
	defaultRetryAfter = 60 * time.Second
)

// sources are the collections searched: their files are originals of
// wallpaper size and their sizes are correct (ADR 0048). Flickr,
// StockSnap and Rawpixel serve previews only; Wikimedia is a source of its
// own; the Met gives no sizes.
var sources = []string{"rijksmuseum", "museumsvictoria", "wordpress", "smk"}

// maxSide caps the size Openverse reports for collections whose files are
// smaller than their originals: SMK serves at most 2048 px.
var maxSide = map[string]int{"smk": 2048}

// serverExcludes are sent with every search, as for Commons.
var serverExcludes = []string{"nude", "naked", "nudity", "erotic"}

// formats lists the file types the WebView can show.
var formats = map[string]string{"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}

// Provider is the Openverse image provider. It is safe for concurrent use.
type Provider struct {
	client    *http.Client
	userAgent string
	baseURL   string
	limiter   *rate.Limiter
}

// New returns an Openverse provider that sends requests with client and
// identifies itself with userAgent.
func New(client *http.Client, userAgent string) *Provider {
	return &Provider{
		client:    client,
		userAgent: userAgent,
		baseURL:   defaultBaseURL,
		// Without a key Openverse allows 20 requests a minute; stay below
		// it, with a small burst for paging.
		limiter: rate.NewLimiter(rate.Every(4*time.Second), 3),
	}
}

func (p *Provider) ID() string   { return ID }
func (p *Provider) Name() string { return "Openverse" }

// ContentWarning implements providers.ContentWarner: the museums include
// nudes in art, and Openverse's mature flag is not complete.
func (p *Provider) ContentWarning() string {
	return "Openverse here searches three museums and the WordPress Photo Directory. Museums " +
		"include nudes in art; Kisetsu filters the results, but some unsuitable images may still appear."
}

// Search returns one page of results for q.
func (p *Provider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	q = q.Normalize()
	if err := q.Validate(); err != nil {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery, err)
	}
	if q.Page > maxPage {
		return providers.Page{Page: q.Page}, nil
	}
	search := searchText(q)
	if n := utf8.RuneCountInString(search); n > maxSearchLength {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery,
			fmt.Errorf("search is %d characters with Openverse filters; the limit is %d", n, maxSearchLength))
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.requestURL(q, search), nil)
	if err != nil {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery, err)
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return providers.Page{}, &providers.Error{
			Provider: ID, Kind: providers.ErrRateLimited, RetryAfter: retryAfter(resp.Header.Get("Retry-After")),
		}
	default:
		return providers.Page{}, p.fail(providers.ErrUnavailable, fmt.Errorf("HTTP %d%s", resp.StatusCode, detail(resp.Body)))
	}

	var body apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&body); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, fmt.Errorf("decode response: %w", err))
	}

	result := providers.Page{
		Images:  make([]images.Image, 0, len(body.Results)),
		Page:    q.Page,
		HasMore: q.Page < min(body.PageCount, maxPage),
	}
	for _, r := range body.Results {
		img, ok := r.toImage()
		if !ok || img.Validate() != nil || !q.Matches(img) {
			continue
		}
		result.Images = append(result.Images, img)
	}
	return result, nil
}

// searchText is the search text with the excluded words, the user's and
// Kisetsu's, as "-word".
func searchText(q providers.Query) string {
	var parts []string
	if q.Text != "" {
		parts = append(parts, q.Text)
	}
	for _, term := range q.Exclude {
		parts = append(parts, "-"+term)
	}
	for _, term := range serverExcludes {
		parts = append(parts, "-"+term)
	}
	return strings.Join(parts, " ")
}

func (p *Provider) requestURL(q providers.Query, search string) string {
	v := url.Values{
		"q":         {search},
		"source":    {strings.Join(sources, ",")},
		"page":      {strconv.Itoa(q.Page)},
		"page_size": {strconv.Itoa(pageSize)},
		"mature":    {"false"},
	}
	switch q.Orientation {
	case images.Landscape:
		v.Set("aspect_ratio", "wide")
	case images.Portrait:
		v.Set("aspect_ratio", "tall")
	case images.Square:
		v.Set("aspect_ratio", "square")
	}
	return p.baseURL + "/images/?" + v.Encode()
}

func (p *Provider) fail(kind, err error) *providers.Error {
	return &providers.Error{Provider: ID, Kind: kind, Err: err}
}

// detail returns Openverse's explanation of a failed request, such as
// "page_size may not exceed 20 for anonymous requests".
func detail(body io.Reader) string {
	var e struct {
		Detail string `json:"detail"`
	}
	if json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&e) != nil || e.Detail == "" {
		return ""
	}
	if len(e.Detail) > 200 {
		e.Detail = e.Detail[:200]
	}
	return ": " + e.Detail
}

func retryAfter(header string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultRetryAfter
}

type apiResponse struct {
	ResultCount int      `json:"result_count"`
	PageCount   int      `json:"page_count"`
	Results     []result `json:"results"`
}

type result struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	ForeignLandingURL string `json:"foreign_landing_url"`
	URL               string `json:"url"`
	Thumbnail         string `json:"thumbnail"`
	Creator           string `json:"creator"`
	CreatorURL        string `json:"creator_url"`
	License           string `json:"license"`
	LicenseVersion    string `json:"license_version"`
	LicenseURL        string `json:"license_url"`
	Source            string `json:"source"`
	Filetype          string `json:"filetype"`
	Filesize          int64  `json:"filesize"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	Mature            bool   `json:"mature"`
	Tags              []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

// toImage normalizes a result. It reports false for results that must
// not be shown: other collections, unknown sizes, undisplayable formats,
// mature ones and anything the word check flags.
func (r result) toImage() (images.Image, bool) {
	mime, ok := formats[strings.ToLower(r.Filetype)]
	if !ok || r.Mature || !slices.Contains(sources, r.Source) || r.Width <= 0 || r.Height <= 0 {
		return images.Image{}, false
	}
	texts := []string{r.Title}
	for _, t := range r.Tags {
		texts = append(texts, t.Name)
	}
	if providers.LooksUnsafe(texts...) {
		return images.Image{}, false
	}
	w, h := r.Width, r.Height
	if m := maxSide[r.Source]; m > 0 && max(w, h) > m {
		// Scaled as the server scales the file; the library stores the
		// real size on download.
		if w >= h {
			w, h = m, max(1, h*m/w)
		} else {
			w, h = max(1, w*m/h), m
		}
	}
	creatorURL := ""
	if isHTTP(r.CreatorURL) {
		creatorURL = r.CreatorURL
	}
	return images.Image{
		Provider:     ID,
		ProviderID:   r.ID,
		PageURL:      pageBase + url.PathEscape(r.ID),
		SourceURL:    r.ForeignLandingURL,
		ThumbnailURL: r.Thumbnail,
		PreviewURL:   r.Thumbnail,
		DownloadURL:  r.URL,
		Width:        w,
		Height:       h,
		MIMEType:     mime,
		FileSize:     r.Filesize,
		Author:       images.Author{Name: strings.TrimSpace(r.Creator), URL: creatorURL},
		License:      r.license(),
	}, true
}

func isHTTP(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// licenseNames maps Openverse's license codes to their usual names.
var licenseNames = map[string]string{
	"cc0":          "CC0",
	"pdm":          "Public Domain Mark",
	"by":           "CC BY",
	"by-sa":        "CC BY-SA",
	"by-nd":        "CC BY-ND",
	"by-nc":        "CC BY-NC",
	"by-nc-sa":     "CC BY-NC-SA",
	"by-nc-nd":     "CC BY-NC-ND",
	"sampling+":    "CC Sampling+",
	"nc-sampling+": "CC NC-Sampling+",
}

// license names the license, such as "CC BY 4.0". Openverse indexes only
// openly licensed works, so the license is always known; an unfamiliar
// code is shown as Openverse gives it.
func (r result) license() images.License {
	code := strings.ToLower(strings.TrimSpace(r.License))
	name, ok := licenseNames[code]
	if !ok {
		name = strings.ToUpper(code)
	}
	if v := strings.TrimSpace(r.LicenseVersion); v != "" && name != "" {
		name += " " + v
	}
	if name == "" {
		return images.License{
			Status: images.LicenseUnknown,
			Notice: "Openverse gives no license for this image. Check its page before reusing it.",
		}
	}
	l := images.License{Status: images.LicenseKnown, Name: name}
	if isHTTP(r.LicenseURL) {
		l.URL = r.LicenseURL
	}
	if strings.HasPrefix(code, "by") {
		l.Notice = "Attribution required: credit the author and link the license when you share this image."
	}
	return l
}
