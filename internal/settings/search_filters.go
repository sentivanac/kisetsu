package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/images"
)

const searchFiltersKey = "search.filters"

const (
	// maxMinSide caps the remembered minimum resolution (16K).
	maxMinSide = 15360
	// maxProviders caps the remembered sources.
	maxProviders = 16
)

// SearchFilters are the search filters remembered between sessions.
// The search text is deliberately not stored: it belongs to search
// history, which is private and handled separately.
type SearchFilters struct {
	// ProviderIDs are the selected sources; empty means the defaults.
	ProviderIDs []string           `json:"providerIds"`
	MinWidth    int                `json:"minWidth"`
	MinHeight   int                `json:"minHeight"`
	Orientation images.Orientation `json:"orientation"` // empty means any
}

// storedSearchFilters also reads the format used before multi-source
// search, which kept a single "providerId".
type storedSearchFilters struct {
	SearchFilters
	LegacyProviderID string `json:"providerId"`
}

// Validate checks that f can be used as search filters.
func (f SearchFilters) Validate() error {
	var errs []error
	if f.MinWidth < 0 || f.MinHeight < 0 || f.MinWidth > maxMinSide || f.MinHeight > maxMinSide {
		errs = append(errs, fmt.Errorf("minimum size %dx%d is outside 0..%d", f.MinWidth, f.MinHeight, maxMinSide))
	}
	if f.Orientation != "" && !f.Orientation.Valid() {
		errs = append(errs, fmt.Errorf("unknown orientation %q", f.Orientation))
	}
	if len(f.ProviderIDs) > maxProviders {
		errs = append(errs, fmt.Errorf("more than %d sources", maxProviders))
	}
	seen := map[string]bool{}
	for _, id := range f.ProviderIDs {
		if id == "" || seen[id] {
			errs = append(errs, fmt.Errorf("source %q is empty or repeated", id))
		}
		seen[id] = true
	}
	return errors.Join(errs...)
}

// SearchFilters returns the remembered filters, or the defaults if none
// were saved. Stored values that are unreadable or invalid also yield the
// defaults: a bad preference must never break searching. Only database
// failures are returned as errors. Unknown source IDs are kept; the UI
// ignores sources that are not registered.
func (s *Store) SearchFilters(ctx context.Context) (SearchFilters, error) {
	var stored storedSearchFilters
	found, err := s.Load(ctx, searchFiltersKey, &stored)
	switch {
	case errors.Is(err, ErrInvalidValue):
		return SearchFilters{}, nil
	case err != nil:
		return SearchFilters{}, err
	case !found:
		return SearchFilters{}, nil
	}

	f := stored.SearchFilters
	if len(f.ProviderIDs) == 0 && stored.LegacyProviderID != "" {
		f.ProviderIDs = []string{stored.LegacyProviderID}
	}
	if f.Validate() != nil {
		return SearchFilters{}, nil
	}
	return f, nil
}

// SaveSearchFilters remembers f.
func (s *Store) SaveSearchFilters(ctx context.Context, f SearchFilters) error {
	if err := f.Validate(); err != nil {
		return err
	}
	return s.Save(ctx, searchFiltersKey, f)
}
