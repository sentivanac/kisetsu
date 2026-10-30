package main

import (
	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// GetSearchFilters returns the remembered search filters, or the defaults.
func (a *App) GetSearchFilters() (filters settings.SearchFilters, err error) {
	defer a.recoverPanic("GetSearchFilters", &err)

	filters, err = a.settings.SearchFilters(a.ctx)
	if err != nil {
		return settings.SearchFilters{}, apperr.New(apperr.Unavailable, "Saved search filters could not be loaded.", err)
	}
	return filters, nil
}

// SaveSearchFilters remembers the search filters for the next session.
func (a *App) SaveSearchFilters(filters settings.SearchFilters) (err error) {
	defer a.recoverPanic("SaveSearchFilters", &err)

	if err := filters.Validate(); err != nil {
		return apperr.New(apperr.InvalidInput, "These search filters are not valid.", err)
	}
	if err := a.settings.SaveSearchFilters(a.ctx, filters); err != nil {
		return apperr.New(apperr.Unavailable, "Search filters could not be saved.", err)
	}
	return nil
}
