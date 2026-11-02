// Package commons searches Wikimedia Commons through the MediaWiki API.
//
// API documentation: https://www.mediawiki.org/wiki/API:Search and
// https://www.mediawiki.org/wiki/Extension:CirrusSearch for search syntax.
// Usage policy: https://foundation.wikimedia.org/wiki/Policy:Wikimedia_Foundation_User-Agent_Policy
package commons

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
	ID = "wikimedia-commons"

	defaultBaseURL = "https://commons.wikimedia.org/w/api.php"

	pageSize = 24
	// maxOffset is the deepest result CirrusSearch returns.
	maxOffset = 10000
	// maxSearchLength is CirrusSearch's limit, not counting filetype:.
	maxSearchLength = 300

	thumbWidth   = 500  // grid thumbnails
	previewWidth = 1280 // shown while the original loads

	maxResponseSize   = 4 << 20 // a page with metadata is about 60 KB
	defaultRetryAfter = 30 * time.Second

	// curated limits results to images reviewed by the community: high
	// quality and much less likely to be unsuitable (see sfw.go).
	curated = "incategory:Quality_images|Featured_pictures_on_Wikimedia_Commons"
)

// displayable lists formats the WebView can show. Commons also has TIFF
// and other bitmap formats.
var displayable = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

// Provider is the Wikimedia Commons image provider. It is safe for
// concurrent use.
type Provider struct {
	client    *http.Client
	userAgent string
	baseURL   string
	limiter   *rate.Limiter
}

// New returns a Commons provider that sends requests with client.
// Wikimedia requires a descriptive User-Agent with contact information.
func New(client *http.Client, userAgent string) *Provider {
	return &Provider{
		client:    client,
		userAgent: userAgent,
		baseURL:   defaultBaseURL,
		// Wikimedia asks API clients to be gentle; two requests per second
		// is far below any limit and fast enough for interactive search.
		limiter: rate.NewLimiter(rate.Every(500*time.Millisecond), 2),
	}
}

func (p *Provider) ID() string   { return ID }
func (p *Provider) Name() string { return "Wikimedia Commons" }

// ContentWarning implements providers.ContentWarner: Commons has no
// safe-for-work rating, so the filter in sfw.go is best effort.
func (p *Provider) ContentWarning() string {
	return "Wikimedia Commons has no safe-for-work rating. Kisetsu filters its results, " +
		"but some unsuitable images may still appear."
}

// Search returns one page of results for q from Commons' curated
// collections, with unsuitable and undisplayable files removed.
func (p *Provider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	q = q.Normalize()
	if err := q.Validate(); err != nil {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery, err)
	}
	offset := (q.Page - 1) * pageSize
	if offset >= maxOffset {
		return providers.Page{Page: q.Page}, nil
	}
	search := searchText(q)
	if n := utf8.RuneCountInString(strings.ReplaceAll(search, "filetype:bitmap", "")); n > maxSearchLength {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery,
			fmt.Errorf("search is %d characters with Commons filters; the limit is %d", n, maxSearchLength))
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.requestURL(search, offset), nil)
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
		return providers.Page{}, p.fail(providers.ErrUnavailable, fmt.Errorf("HTTP %d", resp.StatusCode))
	}

	var body apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&body); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, fmt.Errorf("decode response: %w", err))
	}
	if body.Error != nil {
		return providers.Page{}, p.apiError(body.Error, resp.Header.Get("Retry-After"))
	}

	// Generator results are unordered; index is the search rank.
	slices.SortFunc(body.Query.Pages, func(a, b page) int { return cmp.Compare(a.Index, b.Index) })

	result := providers.Page{
		Images:  make([]images.Image, 0, len(body.Query.Pages)),
		Page:    q.Page,
		HasMore: body.Continue.Offset > 0 && body.Continue.Offset < maxOffset,
	}
	for _, pg := range body.Query.Pages {
		img, ok := pg.toImage()
		if !ok || img.Validate() != nil || !q.Matches(img) {
			continue
		}
		result.Images = append(result.Images, img)
	}
	return result, nil
}

// searchText translates a normalized q into CirrusSearch syntax.
func searchText(q providers.Query) string {
	parts := []string{}
	if q.Text != "" {
		parts = append(parts, q.Text)
	}
	parts = append(parts, "filetype:bitmap", curated)
	// filew/fileh compare strictly, so ">1919" means "at least 1920".
	if q.MinWidth > 0 {
		parts = append(parts, fmt.Sprintf("filew:>%d", q.MinWidth-1))
	}
	if q.MinHeight > 0 {
		parts = append(parts, fmt.Sprintf("fileh:>%d", q.MinHeight-1))
	}
	for _, term := range q.Exclude {
		parts = append(parts, "-"+term)
	}
	for _, term := range serverExcludes {
		parts = append(parts, "-"+term)
	}
	return strings.Join(parts, " ")
}

func (p *Provider) requestURL(search string, offset int) string {
	v := url.Values{
		"action":        {"query"},
		"format":        {"json"},
		"formatversion": {"2"},
		// Ask the server to refuse the request when replicas lag, as the
		// API etiquette recommends for automated clients.
		"maxlag":                {"5"},
		"generator":             {"search"},
		"gsrnamespace":          {"6"}, // File:
		"gsrsearch":             {search},
		"gsrlimit":              {strconv.Itoa(pageSize)},
		"gsroffset":             {strconv.Itoa(offset)},
		"prop":                  {"imageinfo|categories"},
		"clshow":                {"!hidden"},
		"cllimit":               {"max"},
		"iiprop":                {"url|size|mime|extmetadata"},
		"iiurlwidth":            {strconv.Itoa(thumbWidth)},
		"iiextmetadatafilter":   {"LicenseShortName|LicenseUrl|Artist|AttributionRequired|ImageDescription"},
		"iiextmetadatalanguage": {"en"},
	}
	return p.baseURL + "?" + v.Encode()
}

func (p *Provider) apiError(e *apiErrorBody, retryHeader string) error {
	switch {
	case e.Code == "maxlag":
		return &providers.Error{Provider: ID, Kind: providers.ErrUnavailable,
			RetryAfter: retryAfter(retryHeader), Err: fmt.Errorf("server busy (maxlag)")}
	case e.Code == "ratelimited":
		return &providers.Error{Provider: ID, Kind: providers.ErrRateLimited, RetryAfter: retryAfter(retryHeader)}
	case strings.HasPrefix(e.Code, "cirrussearch-query-too-long"), strings.HasPrefix(e.Code, "badvalue"):
		return p.fail(providers.ErrInvalidQuery, fmt.Errorf("API error %s: %s", e.Code, e.Info))
	}
	return p.fail(providers.ErrUnavailable, fmt.Errorf("API error %s: %s", e.Code, e.Info))
}

func (p *Provider) fail(kind, err error) *providers.Error {
	return &providers.Error{Provider: ID, Kind: kind, Err: err}
}

func retryAfter(header string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultRetryAfter
}

type apiResponse struct {
	Error    *apiErrorBody `json:"error"`
	Continue struct {
		Offset int `json:"gsroffset"`
	} `json:"continue"`
	Query struct {
		Pages []page `json:"pages"`
	} `json:"query"`
}

type apiErrorBody struct {
	Code string `json:"code"`
	Info string `json:"info"`
}

type page struct {
	PageID     int    `json:"pageid"`
	Title      string `json:"title"`
	Index      int    `json:"index"`
	Categories []struct {
		Title string `json:"title"`
	} `json:"categories"`
	ImageInfo []imageInfo `json:"imageinfo"`
}

type imageInfo struct {
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Size           int64  `json:"size"`
	MIME           string `json:"mime"`
	URL            string `json:"url"`
	ThumbURL       string `json:"thumburl"`
	DescriptionURL string `json:"descriptionurl"`
	ExtMetadata    map[string]struct {
		Value json.RawMessage `json:"value"`
	} `json:"extmetadata"`
}

// meta returns an extmetadata value as a string. Values are usually
// strings but can be numbers or booleans.
func (ii imageInfo) meta(key string) string {
	raw := ii.ExtMetadata[key].Value
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// toImage normalizes a result. It reports false for files that must not be
// shown: undisplayable formats and anything the SFW check flags.
func (pg page) toImage() (images.Image, bool) {
	if len(pg.ImageInfo) == 0 {
		return images.Image{}, false
	}
	ii := pg.ImageInfo[0]
	if !displayable[ii.MIME] {
		return images.Image{}, false
	}

	description := htmlText(ii.meta("ImageDescription"))
	texts := []string{pg.Title, description}
	for _, c := range pg.Categories {
		texts = append(texts, c.Title)
	}
	if looksUnsafe(texts...) {
		return images.Image{}, false
	}

	authorName, authorURL, authorCredit := parseArtist(ii.meta("Artist"))
	return images.Image{
		Provider:     ID,
		ProviderID:   strconv.Itoa(pg.PageID),
		PageURL:      ii.DescriptionURL,
		ThumbnailURL: thumbAtWidth(ii, thumbWidth),
		PreviewURL:   thumbAtWidth(ii, previewWidth),
		DownloadURL:  ii.URL,
		Width:        ii.Width,
		Height:       ii.Height,
		MIMEType:     ii.MIME,
		FileSize:     ii.Size,
		Author:       images.Author{Name: authorName, URL: authorURL, Credit: authorCredit},
		License:      license(ii),
	}, true
}

func htmlText(fragment string) string {
	text, _ := parseHTML(fragment)
	return text
}

func license(ii imageInfo) images.License {
	name := htmlText(ii.meta("LicenseShortName"))
	if name == "" {
		return images.License{
			Status: images.LicenseUnknown,
			Notice: "Commons gives no license for this file. Check its page before reusing it.",
		}
	}
	licenseURL, _ := absoluteLink(ii.meta("LicenseUrl"))
	l := images.License{Status: images.LicenseKnown, Name: name, URL: licenseURL}
	if ii.meta("AttributionRequired") == "true" {
		l.Notice = "Attribution required: credit the author and link the license when you share this image."
	}
	return l
}

// thumbPx matches the width prefix of a Commons thumbnail file name,
// for example "500px-" in ".../thumb/d/dd/Name.jpg/500px-Name.jpg".
var thumbPx = regexp.MustCompile(`^\d+px-`)

// thumbAtWidth returns a thumbnail URL of the given width, derived from the
// thumbnail Commons returned. Only the width prefix of the last path
// segment changes, so the URL's encoding stays exactly as Commons sent it.
// Images narrower than width use the original, because Commons does not
// upscale.
func thumbAtWidth(ii imageInfo, width int) string {
	if ii.Width <= width || !strings.Contains(ii.ThumbURL, "/thumb/") {
		return ii.URL
	}
	path, query, _ := strings.Cut(ii.ThumbURL, "?")
	slash := strings.LastIndex(path, "/")
	file := path[slash+1:]
	if !thumbPx.MatchString(file) {
		return ii.URL
	}
	out := path[:slash+1] + thumbPx.ReplaceAllString(file, fmt.Sprintf("%dpx-", width))
	if query != "" {
		out += "?" + query
	}
	return out
}
