// Package search runs image searches against the registered providers,
// one at a time (Search) or several concurrently (SearchMany).
package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sentivanac/kisetsu/internal/providers"
)

// DefaultTimeout bounds one search, including waiting for rate limits.
const DefaultTimeout = 20 * time.Second

// ErrUnknownProvider means no provider with the requested ID is registered.
var ErrUnknownProvider = errors.New("unknown provider")

// ProviderInfo describes a registered provider for the UI.
type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// ContentWarning is set for providers that cannot guarantee
	// safe-for-work results (see providers.ContentWarner).
	ContentWarning string `json:"contentWarning,omitempty"`
	// EnabledByDefault is false for providers with a content warning:
	// the user must turn them on.
	EnabledByDefault bool `json:"enabledByDefault"`
	// Categories is true for providers that apply Query.Categories.
	Categories bool `json:"categories"`
	// Tags is true for providers whose images have tags (Query.HiddenTags
	// applies to them).
	Tags bool `json:"tags"`
}

// Service runs searches. It is safe for concurrent use.
type Service struct {
	log       *slog.Logger
	providers []providers.Provider // registration order, shown in the UI
	byID      map[string]providers.Provider
	timeout   time.Duration

	// HiddenTags, if set, returns the tags to leave out of every search
	// (from the user's settings). If it fails, searches run without them.
	HiddenTags func(context.Context) ([]string, error)
}

// New returns a Service for the given providers. Provider IDs must be
// unique and non-empty.
func New(log *slog.Logger, ps ...providers.Provider) (*Service, error) {
	if len(ps) == 0 {
		return nil, errors.New("search: no providers registered")
	}
	byID := make(map[string]providers.Provider, len(ps))
	for _, p := range ps {
		id := p.ID()
		if id == "" {
			return nil, fmt.Errorf("search: provider %q has an empty ID", p.Name())
		}
		if _, dup := byID[id]; dup {
			return nil, fmt.Errorf("search: provider ID %q registered twice", id)
		}
		byID[id] = p
	}
	return &Service{log: log, providers: ps, byID: byID, timeout: DefaultTimeout}, nil
}

// Providers lists the registered providers in registration order.
func (s *Service) Providers() []ProviderInfo {
	out := make([]ProviderInfo, len(s.providers))
	for i, p := range s.providers {
		info := ProviderInfo{ID: p.ID(), Name: p.Name()}
		if w, ok := p.(providers.ContentWarner); ok {
			info.ContentWarning = w.ContentWarning()
		}
		info.EnabledByDefault = info.ContentWarning == ""
		if c, ok := p.(providers.Categorizer); ok {
			info.Categories = c.HasCategories()
		}
		_, info.Tags = p.(providers.Tagger)
		out[i] = info
	}
	return out
}

// ProviderName returns the display name for a provider ID, or the ID
// itself if it is not registered.
func (s *Service) ProviderName(id string) string {
	if p, ok := s.byID[id]; ok {
		return p.Name()
	}
	return id
}

// Search runs q against the provider with the given ID.
func (s *Service) Search(ctx context.Context, providerID string, q providers.Query) (providers.Page, error) {
	p, ok := s.byID[providerID]
	if !ok {
		return providers.Page{}, fmt.Errorf("%w %q", ErrUnknownProvider, providerID)
	}
	// Hidden tags always come from the settings, never from the caller.
	q.HiddenTags = nil
	if s.HiddenTags != nil {
		tags, err := s.HiddenTags(ctx)
		if err != nil {
			s.log.Warn("hidden tags could not be read; searching without them", "err", err)
		}
		q.HiddenTags = tags
	}
	// Split "-term" words into q.Exclude, then validate here as well, so an
	// invalid query never costs an API call.
	q = q.Normalize()
	if err := q.Validate(); err != nil {
		return providers.Page{}, &providers.Error{Provider: providerID, Kind: providers.ErrInvalidQuery, Err: err}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := time.Now()
	page, err := p.Search(ctx, q)
	elapsed := time.Since(start).Round(time.Millisecond)

	// Search text is private; it is only logged at debug level.
	s.log.Debug("search query", "provider", providerID, "text", q.Text, "exclude", q.Exclude)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.log.Debug("search canceled", "provider", providerID, "page", q.Page, "duration", elapsed)
		} else {
			s.log.Warn("search failed", "provider", providerID, "page", q.Page, "duration", elapsed, "err", err)
		}
		return providers.Page{}, err
	}
	s.log.Info("search done", "provider", providerID, "page", q.Page,
		"results", len(page.Images), "has_more", page.HasMore, "duration", elapsed)
	return page, nil
}

// Tags returns the tags of one image, for providers whose images have
// tags; others return none.
func (s *Service) Tags(ctx context.Context, providerID, imageID string) ([]string, error) {
	p, ok := s.byID[providerID]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownProvider, providerID)
	}
	t, ok := p.(providers.Tagger)
	if !ok {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	tags, err := t.Tags(ctx, imageID)
	if err != nil && !errors.Is(err, context.Canceled) {
		s.log.Warn("image tags failed", "provider", providerID, "err", err)
	}
	return tags, err
}
