// Package providers defines the contract every image provider implements.
//
// Each provider lives in its own subpackage (for example
// providers/wallhaven) and is registered in main.go. Adding a provider
// must not require changes to this package or to the search package.
package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sentivanac/kisetsu/internal/images"
)

// Provider searches one external image source.
type Provider interface {
	// ID is a stable lowercase identifier, e.g. "wallhaven". It is stored
	// in the database, so it must never change.
	ID() string
	// Name is the human-readable name shown in the UI.
	Name() string
	// Search returns one page of results. It must honour ctx cancellation,
	// return only images that pass images.Image.Validate, and return
	// errors as *Error.
	Search(ctx context.Context, q Query) (Page, error)
}

const (
	// MaxQueryLength limits the search text, in characters.
	MaxQueryLength = 256
	// MaxExcludeTerms limits the number of excluded terms.
	MaxExcludeTerms = 16
	// MaxTermLength limits one excluded term, in characters.
	MaxTermLength = 64
)

// Query is a provider-independent search request. Each provider
// translates it into its own API parameters.
type Query struct {
	Text string `json:"text"` // empty means browse (e.g. popular images)
	// Exclude lists terms results must not match, without the leading "-".
	// Normalize fills it from "-term" words in Text.
	Exclude   []string `json:"exclude"`
	Page      int      `json:"page"`      // 1-based
	MinWidth  int      `json:"minWidth"`  // 0 means no limit
	MinHeight int      `json:"minHeight"` // 0 means no limit
	// Orientation filters results; empty means any.
	Orientation images.Orientation `json:"orientation"`
}

// Normalize moves "-term" words from Text into Exclude, collapses
// whitespace and removes duplicate excluded terms (case-insensitively).
// Words with an inner hyphen, such as "sci-fi", stay in Text; a lone "-"
// is dropped. Quoted phrases are not supported. Normalize is idempotent.
func (q Query) Normalize() Query {
	var include []string
	exclude := make([]string, 0, len(q.Exclude))
	seen := map[string]bool{}
	addExclude := func(term string) {
		key := strings.ToLower(term)
		if term == "" || seen[key] {
			return
		}
		seen[key] = true
		exclude = append(exclude, term)
	}

	for _, term := range q.Exclude {
		addExclude(strings.TrimLeft(strings.TrimSpace(term), "-"))
	}
	for word := range strings.FieldsSeq(q.Text) {
		if strings.HasPrefix(word, "-") {
			addExclude(strings.TrimLeft(word, "-"))
			continue
		}
		include = append(include, word)
	}

	q.Text = strings.Join(include, " ")
	q.Exclude = nil
	if len(exclude) > 0 {
		q.Exclude = exclude
	}
	return q
}

// Validate checks that q is well formed.
func (q Query) Validate() error {
	var errs []error
	if utf8.RuneCountInString(strings.TrimSpace(q.Text)) > MaxQueryLength {
		errs = append(errs, fmt.Errorf("search text is longer than %d characters", MaxQueryLength))
	}
	if len(q.Exclude) > MaxExcludeTerms {
		errs = append(errs, fmt.Errorf("more than %d excluded terms", MaxExcludeTerms))
	}
	for _, term := range q.Exclude {
		if term == "" || strings.ContainsFunc(term, unicode.IsSpace) || utf8.RuneCountInString(term) > MaxTermLength {
			errs = append(errs, fmt.Errorf("excluded term %q must be a single word of at most %d characters", term, MaxTermLength))
		}
	}
	if q.Page < 1 {
		errs = append(errs, fmt.Errorf("page %d is not positive", q.Page))
	}
	if q.MinWidth < 0 || q.MinHeight < 0 {
		errs = append(errs, fmt.Errorf("minimum size %dx%d is negative", q.MinWidth, q.MinHeight))
	}
	if q.Orientation != "" && !q.Orientation.Valid() {
		errs = append(errs, fmt.Errorf("unknown orientation %q", q.Orientation))
	}
	return errors.Join(errs...)
}

// Matches reports whether img satisfies the filters in q. Providers use it
// for filters their API cannot apply, and as a final safety check.
func (q Query) Matches(img images.Image) bool {
	if img.Width < q.MinWidth || img.Height < q.MinHeight {
		return false
	}
	if q.Orientation != "" && img.Orientation() != q.Orientation {
		return false
	}
	return true
}

// Page is one page of search results.
type Page struct {
	Images  []images.Image `json:"images"`
	Page    int            `json:"page"`
	HasMore bool           `json:"hasMore"`
}
