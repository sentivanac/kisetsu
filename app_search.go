package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/search"
)

// ListProviders returns the image providers the user can search.
func (a *App) ListProviders() (list []search.ProviderInfo, err error) {
	defer a.recoverPanic("ListProviders", &err)

	return a.search.Providers(), nil
}

// Search runs one page of a search with the given provider.
func (a *App) Search(providerID string, query providers.Query) (page providers.Page, err error) {
	defer a.recoverPanic("Search", &err)

	page, err = a.search.Search(a.ctx, providerID, query)
	if err != nil {
		return providers.Page{}, a.searchError(err)
	}
	return page, nil
}

// searchError turns a search failure into a message for the user.
func (a *App) searchError(err error) error {
	if errors.Is(err, search.ErrUnknownProvider) {
		return apperr.New(apperr.InvalidInput, "This image source is not available.", err)
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
