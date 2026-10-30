package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/search"
)

// ListProviders returns the image providers the user can search.
func (a *App) ListProviders() (list []search.ProviderInfo, err error) {
	defer a.recoverPanic("ListProviders", &err)

	return a.search.Providers(), nil
}

// SourceStatus reports how one source did in a multi-source search.
type SourceStatus struct {
	ProviderID string         `json:"providerId"`
	Name       string         `json:"name"`
	Page       int            `json:"page"`
	HasMore    bool           `json:"hasMore"`
	Count      int            `json:"count"`
	Error      *apperr.Public `json:"error,omitempty"`
}

// SearchResult is the combined result of a multi-source search.
type SearchResult struct {
	Images  []images.Image `json:"images"`
	Sources []SourceStatus `json:"sources"`
}

// SearchSources searches several sources at once, each at its own page.
// A source that fails is reported in its SourceStatus; the call itself
// fails only for an invalid request.
//
// Starting a search cancels the previous one, if it is still running.
func (a *App) SearchSources(sources []search.SourcePage, query providers.Query) (result SearchResult, err error) {
	defer a.recoverPanic("SearchSources", &err)

	ctx, done := a.beginSearch()
	defer done()

	multi, err := a.search.SearchMany(ctx, sources, query)
	if errors.Is(ctx.Err(), context.Canceled) {
		return SearchResult{}, apperr.New(apperr.Canceled, "The search was canceled.", ctx.Err())
	}
	if err != nil {
		return SearchResult{}, a.searchError(err)
	}

	result = SearchResult{Images: multi.Images, Sources: make([]SourceStatus, len(multi.Sources))}
	for i, out := range multi.Sources {
		status := SourceStatus{
			ProviderID: out.ProviderID,
			Name:       a.search.ProviderName(out.ProviderID),
			Page:       out.Page,
			HasMore:    out.HasMore,
			Count:      out.Count,
		}
		if out.Err != nil {
			public := apperr.ToPublic(a.searchError(out.Err))
			status.Error = &public
		}
		result.Sources[i] = status
	}
	return result, nil
}

// CancelSearch stops the running search, if any. The canceled call
// returns an error with code "canceled".
func (a *App) CancelSearch() (err error) {
	defer a.recoverPanic("CancelSearch", &err)

	a.searchMu.Lock()
	defer a.searchMu.Unlock()
	if a.cancelSearch != nil {
		a.cancelSearch()
	}
	return nil
}

// beginSearch cancels the running search and returns the context for a
// new one. done must be called when the new search ends.
func (a *App) beginSearch() (ctx context.Context, done func()) {
	ctx, cancel := context.WithCancel(a.ctx)

	a.searchMu.Lock()
	if a.cancelSearch != nil {
		a.cancelSearch()
	}
	a.searchSeq++
	seq := a.searchSeq
	a.cancelSearch = cancel
	a.searchMu.Unlock()

	return ctx, func() {
		a.searchMu.Lock()
		// A newer search may already have replaced this one.
		if a.searchSeq == seq {
			a.cancelSearch = nil
		}
		a.searchMu.Unlock()
		cancel()
	}
}

// searchError turns a search failure into a message for the user.
func (a *App) searchError(err error) error {
	if errors.Is(err, search.ErrNoSources) {
		return apperr.New(apperr.InvalidInput, "Select at least one image source.", err)
	}
	if errors.Is(err, search.ErrUnknownProvider) {
		return apperr.New(apperr.InvalidInput, "This image source is not available.", err)
	}
	if errors.Is(err, search.ErrDuplicateSource) {
		return apperr.New(apperr.InvalidInput, "Each image source can only be searched once at a time.", err)
	}

	pe, ok := errors.AsType[*providers.Error](err)
	if !ok {
		return apperr.InternalError(err)
	}
	name := a.search.ProviderName(pe.Provider)

	switch {
	case errors.Is(err, providers.ErrInvalidQuery):
		return apperr.New(apperr.InvalidInput,
			"This search is not valid. Check the search text and filters.", err)

	case errors.Is(err, providers.ErrRateLimited):
		e := apperr.New(apperr.RateLimited,
			fmt.Sprintf("%s received too many searches. Please wait a moment and try again.", name), err)
		e.RetryAfter = pe.RetryAfter
		return e

	case errors.Is(err, context.DeadlineExceeded):
		return apperr.New(apperr.ProviderUnavailable,
			fmt.Sprintf("%s did not respond in time. Please try again.", name), err)

	case errors.Is(err, providers.ErrUnavailable):
		return apperr.New(apperr.ProviderUnavailable,
			fmt.Sprintf("%s could not be reached. Check your internet connection and try again.", name), err)
	}
	return apperr.InternalError(err)
}
