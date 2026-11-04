package main

import (
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/providers"
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

// GetHiddenTags returns the tags left out of every search.
func (a *App) GetHiddenTags() (tags []string, err error) {
	defer a.recoverPanic("GetHiddenTags", &err)

	tags, err = a.settings.HiddenTags(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "Hidden tags could not be loaded.", err)
	}
	return nonNil(tags), nil
}

// SetTagHidden hides a tag from all searches, or shows it again, and
// returns the new list of hidden tags.
func (a *App) SetTagHidden(tag string, hidden bool) (tags []string, err error) {
	defer a.recoverPanic("SetTagHidden", &err)

	tags, err = a.settings.SetTagHidden(a.ctx, tag, hidden)
	if errors.Is(err, settings.ErrInvalidTag) {
		return nil, apperr.New(apperr.InvalidInput,
			fmt.Sprintf("This tag cannot be hidden. At most %d tags of up to %d characters can be hidden.",
				providers.MaxHiddenTags, providers.MaxTagLength), err)
	}
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "Hidden tags could not be saved.", err)
	}
	return nonNil(tags), nil
}

// nonNil returns an empty list instead of nil, so the frontend receives
// [] rather than null.
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
