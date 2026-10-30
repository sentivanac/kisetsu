// Package search runs image searches against the registered providers.
//
// In Phase 2 a search goes to one provider chosen by the user. Searching
// several providers concurrently arrives in Phase 3 without changing the
// Provider interface.
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
}

// Service runs searches. It is safe for concurrent use.
type Service struct {
	log       *slog.Logger
	providers []providers.Provider // registration order, shown in the UI
	byID      map[string]providers.Provider
	timeout   time.Duration
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
		out[i] = ProviderInfo{ID: p.ID(), Name: p.Name()}
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
	// Validate here as well, so an invalid query never costs an API call.
	if err := q.Validate(); err != nil {
		return providers.Page{}, &providers.Error{Provider: providerID, Kind: providers.ErrInvalidQuery, Err: err}
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	start := time.Now()
	page, err := p.Search(ctx, q)
	elapsed := time.Since(start).Round(time.Millisecond)

	// Search text is private; it is only logged at debug level.
	s.log.Debug("search query", "provider", providerID, "text", q.Text)
	if err != nil {
		s.log.Warn("search failed", "provider", providerID, "page", q.Page, "duration", elapsed, "err", err)
		return providers.Page{}, err
	}
	s.log.Info("search done", "provider", providerID, "page", q.Page,
		"results", len(page.Images), "has_more", page.HasMore, "duration", elapsed)
	return page, nil
}
