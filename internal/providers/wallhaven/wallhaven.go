// Package wallhaven searches https://wallhaven.cc through its public API.
//
// API documentation: https://wallhaven.cc/help/api
package wallhaven

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

const (
	// ID is the stable provider identifier.
	ID = "wallhaven"

	defaultBaseURL = "https://wallhaven.cc/api/v1"

	// Wallhaven allows 45 API calls per minute per client. Stay below it,
	// because the limit is shared with anything else using the same IP.
	requestsPerMinute = 40
	burst             = 4

	// maxResponseSize guards against unexpectedly large responses.
	// A page of 24 results is about 30 KB.
	maxResponseSize = 2 << 20

	// defaultRetryAfter is used when a 429 response has no Retry-After header.
	defaultRetryAfter = time.Minute

	licenseNotice = "Wallhaven does not provide license information. " +
		"All images remain property of their original owners."
)

// Provider is the Wallhaven image provider. It is safe for concurrent use.
type Provider struct {
	client    *http.Client
	userAgent string
	baseURL   string
	limiter   *rate.Limiter
}

// New returns a Wallhaven provider that sends requests with client.
func New(client *http.Client, userAgent string) *Provider {
	return &Provider{
		client:    client,
		userAgent: userAgent,
		baseURL:   defaultBaseURL,
		limiter:   rate.NewLimiter(rate.Every(time.Minute/requestsPerMinute), burst),
	}
}

func (p *Provider) ID() string   { return ID }
func (p *Provider) Name() string { return "Wallhaven" }

// Search returns one page of safe-for-work results for q.
func (p *Provider) Search(ctx context.Context, q providers.Query) (providers.Page, error) {
	if err := q.Validate(); err != nil {
		return providers.Page{}, p.fail(providers.ErrInvalidQuery, err)
	}
	if err := p.limiter.Wait(ctx); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.searchURL(q), nil)
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

	if err := p.checkStatus(resp); err != nil {
		return providers.Page{}, err
	}

	var body searchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&body); err != nil {
		return providers.Page{}, p.fail(providers.ErrUnavailable, fmt.Errorf("decode response: %w", err))
	}

	page := providers.Page{
		Images:  make([]images.Image, 0, len(body.Data)),
		Page:    body.Meta.CurrentPage,
		HasMore: body.Meta.CurrentPage < body.Meta.LastPage,
	}
	for _, w := range body.Data {
		// Purity is requested as SFW only; this check also drops anything
		// else if the API ever returns it (for example with an API key
		// whose account settings allow more).
		if w.Purity != "sfw" {
			continue
		}
		img := w.toImage()
		if img.Validate() != nil || !q.Matches(img) {
			continue
		}
		page.Images = append(page.Images, img)
	}
	return page, nil
}

// searchURL translates q into Wallhaven search parameters.
func (p *Provider) searchURL(q providers.Query) string {
	v := url.Values{}
	text := strings.TrimSpace(q.Text)
	if text != "" {
		v.Set("q", text)
		v.Set("sorting", "relevance")
	} else {
		v.Set("sorting", "toplist")
	}
	v.Set("categories", "111") // general, anime, people
	v.Set("purity", "100")     // SFW only
	v.Set("page", strconv.Itoa(q.Page))
	if q.MinWidth > 0 || q.MinHeight > 0 {
		v.Set("atleast", fmt.Sprintf("%dx%d", max(q.MinWidth, 1), max(q.MinHeight, 1)))
	}
	// Wallhaven understands "landscape" and "portrait". Square images are
	// filtered locally by Query.Matches.
	switch q.Orientation {
	case images.Landscape, images.Portrait:
		v.Set("ratios", string(q.Orientation))
	}
	return p.baseURL + "/search?" + v.Encode()
}

func (p *Provider) checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusTooManyRequests:
		return &providers.Error{
			Provider:   ID,
			Kind:       providers.ErrRateLimited,
			RetryAfter: retryAfter(resp.Header.Get("Retry-After")),
		}
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return p.fail(providers.ErrInvalidQuery, fmt.Errorf("HTTP %d", resp.StatusCode))
	default:
		return p.fail(providers.ErrUnavailable, fmt.Errorf("HTTP %d", resp.StatusCode))
	}
}

func (p *Provider) fail(kind, err error) *providers.Error {
	return &providers.Error{Provider: ID, Kind: kind, Err: err}
}

// retryAfter parses a Retry-After header given in seconds.
func retryAfter(header string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultRetryAfter
}

type searchResponse struct {
	Data []wallpaper `json:"data"`
	Meta struct {
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
	} `json:"meta"`
}

type wallpaper struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Source   string `json:"source"`
	Purity   string `json:"purity"`
	Width    int    `json:"dimension_x"`
	Height   int    `json:"dimension_y"`
	FileSize int64  `json:"file_size"`
	FileType string `json:"file_type"`
	Path     string `json:"path"`
	Thumbs   struct {
		Large string `json:"large"`
		Small string `json:"small"`
	} `json:"thumbs"`
}

func (w wallpaper) toImage() images.Image {
	return images.Image{
		Provider:     ID,
		ProviderID:   w.ID,
		PageURL:      w.URL,
		ThumbnailURL: w.Thumbs.Small,
		PreviewURL:   w.Thumbs.Large,
		DownloadURL:  w.Path,
		Width:        w.Width,
		Height:       w.Height,
		MIMEType:     w.FileType,
		FileSize:     w.FileSize,
		// Wallhaven only knows the uploader, who is usually not the author.
		Author:    images.Author{},
		SourceURL: sourceURL(w.Source),
		License: images.License{
			Status: images.LicenseUnknown,
			Notice: licenseNotice,
		},
	}
}

// sourceURL keeps the uploader-supplied source only if it is an http(s) URL.
func sourceURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return u.String()
}
