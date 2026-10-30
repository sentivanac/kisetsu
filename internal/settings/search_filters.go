package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/images"
)

const searchFiltersKey = "search.filters"

// maxMinSide caps the remembered minimum resolution (16K).
const maxMinSide = 15360

// SearchFilters are the search filters remembered between sessions.
// The search text is deliberately not stored: it belongs to search
// history, which is private and handled separately.
type SearchFilters struct {
	ProviderID  string             `json:"providerId"` // empty means the first provider
	MinWidth    int                `json:"minWidth"`
	MinHeight   int                `json:"minHeight"`
	Orientation images.Orientation `json:"orientation"` // empty means any
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
	return errors.Join(errs...)
}

// SearchFilters returns the remembered filters, or the defaults if none
// were saved. Stored values that are unreadable or invalid also yield the
// defaults: a bad preference must never break searching. Only database
// failures are returned as errors.
func (s *Store) SearchFilters(ctx context.Context) (SearchFilters, error) {
	var f SearchFilters
	found, err := s.Load(ctx, searchFiltersKey, &f)
	switch {
	case errors.Is(err, ErrInvalidValue):
		return SearchFilters{}, nil
	case err != nil:
		return SearchFilters{}, err
	case !found || f.Validate() != nil:
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
