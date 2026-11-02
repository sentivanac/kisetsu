package main

import (
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/searches"
)

// SavedSearch is a saved search for the frontend.
type SavedSearch struct {
	ID    int64          `json:"id"`
	Name  string         `json:"name"`
	Query searches.Query `json:"query"`
}

// ListSavedSearches returns the saved searches by name.
func (a *App) ListSavedSearches() (list []SavedSearch, err error) {
	defer a.recoverPanic("ListSavedSearches", &err)

	saved, err := a.savedSearches.List(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "Saved searches could not be loaded.", err)
	}
	list = make([]SavedSearch, len(saved))
	for i, s := range saved {
		list[i] = SavedSearch{ID: s.ID, Name: s.Name, Query: s.Query}
	}
	return list, nil
}

// SaveSearch saves a search under a name.
func (a *App) SaveSearch(name string, query searches.Query) (saved SavedSearch, err error) {
	defer a.recoverPanic("SaveSearch", &err)

	s, err := a.savedSearches.Create(a.ctx, name, query)
	if err != nil {
		return SavedSearch{}, savedSearchError(err, "The search could not be saved.")
	}
	a.log.Info("search saved", "id", s.ID, "sources", len(s.Query.ProviderIDs))
	return SavedSearch{ID: s.ID, Name: s.Name, Query: s.Query}, nil
}

// RenameSavedSearch changes a saved search's name.
func (a *App) RenameSavedSearch(id int64, name string) (err error) {
	defer a.recoverPanic("RenameSavedSearch", &err)

	if err := a.savedSearches.Rename(a.ctx, id, name); err != nil {
		return savedSearchError(err, "The search could not be renamed.")
	}
	return nil
}

// DeleteSavedSearch removes a saved search.
func (a *App) DeleteSavedSearch(id int64) (err error) {
	defer a.recoverPanic("DeleteSavedSearch", &err)

	if err := a.savedSearches.Delete(a.ctx, id); err != nil {
		return savedSearchError(err, "The search could not be deleted.")
	}
	return nil
}

func savedSearchError(err error, fallback string) error {
	switch {
	case errors.Is(err, searches.ErrNameTaken):
		return apperr.New(apperr.InvalidInput, "A saved search with this name already exists.", err)
	case errors.Is(err, searches.ErrInvalidName):
		return apperr.New(apperr.InvalidInput,
			fmt.Sprintf("Names must be 1 to %d characters long.", searches.MaxName), err)
	case errors.Is(err, searches.ErrInvalidQuery):
		return apperr.New(apperr.InvalidInput,
			"This search cannot be saved. Choose at least one source and check the search text.", err)
	case errors.Is(err, searches.ErrNotFound):
		return apperr.New(apperr.InvalidInput, "This saved search no longer exists.", err)
	}
	return apperr.New(apperr.Unavailable, fallback, err)
}
