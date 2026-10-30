// Package images defines the provider-independent image model.
package images

import (
	"errors"
	"fmt"
	"net/url"
)

// Orientation describes the shape of an image.
type Orientation string

const (
	Landscape Orientation = "landscape"
	Portrait  Orientation = "portrait"
	Square    Orientation = "square"
)

// Valid reports whether o is one of the defined orientations.
func (o Orientation) Valid() bool {
	switch o {
	case Landscape, Portrait, Square:
		return true
	}
	return false
}

// LicenseStatus says whether the license of an image is known.
// There is deliberately no zero value: every provider must decide.
type LicenseStatus string

const (
	// LicenseKnown means License.Name (and usually License.URL) identify
	// the license, e.g. "CC BY-SA 4.0".
	LicenseKnown LicenseStatus = "known"
	// LicenseUnknown means the provider gives no license. The image must
	// not be treated as free to redistribute.
	LicenseUnknown LicenseStatus = "unknown"
)

// License describes the terms under which an image may be used.
type License struct {
	Status LicenseStatus `json:"status"`
	Name   string        `json:"name"`   // e.g. "CC BY-SA 4.0"; empty when unknown
	URL    string        `json:"url"`    // license text
	Notice string        `json:"notice"` // provider statement to show users
}

// Author is the creator of an image. Empty fields mean unknown;
// providers must never guess.
type Author struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Image is one image result, normalized across providers.
type Image struct {
	Provider     string  `json:"provider"`     // provider ID, e.g. "wallhaven"
	ProviderID   string  `json:"providerId"`   // image ID at the provider
	PageURL      string  `json:"pageUrl"`      // image page at the provider, for attribution
	ThumbnailURL string  `json:"thumbnailUrl"` // small image for grids
	PreviewURL   string  `json:"previewUrl"`   // uncropped, possibly small; shown while DownloadURL loads
	DownloadURL  string  `json:"downloadUrl"`  // original file
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	MIMEType     string  `json:"mimeType"` // e.g. "image/jpeg"; empty when unknown
	FileSize     int64   `json:"fileSize"` // bytes; 0 when unknown
	Author       Author  `json:"author"`
	SourceURL    string  `json:"sourceUrl"` // original source, if the provider knows it
	License      License `json:"license"`
}

// AspectRatio returns width divided by height, or 0 if the height is unknown.
func (img Image) AspectRatio() float64 {
	if img.Height <= 0 {
		return 0
	}
	return float64(img.Width) / float64(img.Height)
}

// Orientation classifies the image by its dimensions. Images whose sides
// differ by less than 5% count as square.
func (img Image) Orientation() Orientation {
	r := img.AspectRatio()
	switch {
	case r == 0:
		return ""
	case r > 1.05:
		return Landscape
	case r < 1/1.05:
		return Portrait
	}
	return Square
}

// Validate checks that the fields every result needs are present.
// Providers call it on each normalized image and drop invalid ones.
func (img Image) Validate() error {
	var errs []error
	if img.Provider == "" {
		errs = append(errs, errors.New("provider is empty"))
	}
	if img.ProviderID == "" {
		errs = append(errs, errors.New("provider ID is empty"))
	}
	if img.Width <= 0 || img.Height <= 0 {
		errs = append(errs, fmt.Errorf("invalid dimensions %dx%d", img.Width, img.Height))
	}
	for name, u := range map[string]string{
		"page URL":      img.PageURL,
		"thumbnail URL": img.ThumbnailURL,
		"download URL":  img.DownloadURL,
	} {
		if err := checkHTTPURL(u); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	switch img.License.Status {
	case LicenseKnown:
		if img.License.Name == "" {
			errs = append(errs, errors.New("known license has no name"))
		}
	case LicenseUnknown:
	default:
		errs = append(errs, fmt.Errorf("license status %q is not set", img.License.Status))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("image %s/%s: %w", img.Provider, img.ProviderID, err)
	}
	return nil
}

// checkHTTPURL accepts only absolute http(s) URLs, so a provider response
// cannot make the UI load file:// or javascript: URLs.
func checkHTTPURL(raw string) error {
	if raw == "" {
		return errors.New("is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("%q is not an http(s) URL", raw)
	}
	return nil
}
