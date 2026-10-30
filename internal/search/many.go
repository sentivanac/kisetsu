package search

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

var (
	// ErrNoSources means a multi-source search named no providers.
	ErrNoSources = errors.New("no sources selected")
	// ErrDuplicateSource means a provider was named twice in one search.
	ErrDuplicateSource = errors.New("source requested twice")
)

// SourcePage selects one provider and the page to fetch from it. Each
// provider pages independently, so "Load more" asks every source for its
// own next page.
type SourcePage struct {
	ProviderID string `json:"providerId"`
	Page       int    `json:"page"` // 1-based
}

// SourceOutcome is what one provider returned in a multi-source search.
type SourceOutcome struct {
	ProviderID string
	Page       int
	HasMore    bool
	Count      int   // images contributed by this provider
	Err        error // nil on success; a *providers.Error otherwise
}

// MultiResult is the combined result of a multi-source search.
type MultiResult struct {
	// Images interleaves the providers' results: first image of each
	// source in request order, then the second, and so on.
	Images  []images.Image
	Sources []SourceOutcome // in request order
}

// SearchMany runs q against several providers concurrently. A provider
// that fails does not affect the others: its error is reported in its
// SourceOutcome and SearchMany still succeeds. An error is returned only
// for an invalid request (no sources, unknown or repeated provider, or an
// invalid query), before any provider is called.
func (s *Service) SearchMany(ctx context.Context, sources []SourcePage, q providers.Query) (MultiResult, error) {
	if len(sources) == 0 {
		return MultiResult{}, ErrNoSources
	}
	seen := make(map[string]bool, len(sources))
	for _, src := range sources {
		if _, ok := s.byID[src.ProviderID]; !ok {
			return MultiResult{}, fmt.Errorf("%w %q", ErrUnknownProvider, src.ProviderID)
		}
		if seen[src.ProviderID] {
			return MultiResult{}, fmt.Errorf("%w: %q", ErrDuplicateSource, src.ProviderID)
		}
		seen[src.ProviderID] = true
	}
	q = q.Normalize()
	for _, src := range sources {
		q.Page = src.Page
		if err := q.Validate(); err != nil {
			return MultiResult{}, &providers.Error{Provider: src.ProviderID, Kind: providers.ErrInvalidQuery, Err: err}
		}
	}

	pages := make([]providers.Page, len(sources))
	outcomes := make([]SourceOutcome, len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Go(func() {
			sq := q
			sq.Page = src.Page
			page, err := s.Search(ctx, src.ProviderID, sq)
			pages[i] = page
			outcomes[i] = SourceOutcome{
				ProviderID: src.ProviderID,
				Page:       src.Page,
				HasMore:    err == nil && page.HasMore,
				Count:      len(page.Images),
				Err:        err,
			}
		})
	}
	wg.Wait()

	return MultiResult{Images: interleave(pages), Sources: outcomes}, nil
}

// interleave merges pages round-robin, so no source buries the others.
func interleave(pages []providers.Page) []images.Image {
	total, longest := 0, 0
	for _, p := range pages {
		total += len(p.Images)
		longest = max(longest, len(p.Images))
	}
	out := make([]images.Image, 0, total)
	for i := range longest {
		for _, p := range pages {
			if i < len(p.Images) {
				out = append(out, p.Images[i])
			}
		}
	}
	return out
}
