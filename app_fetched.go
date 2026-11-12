package main

import (
	"errors"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// FetchedStats tells how much space fetched images use.
type FetchedStats struct {
	Images   int   `json:"images"`
	Bytes    int64 `json:"bytes"`
	MaxBytes int64 `json:"maxBytes"`
}

// ListFetched returns fetched images, newest first.
func (a *App) ListFetched(offset, limit int) (page LibraryPage, err error) {
	defer a.recoverPanic("ListFetched", &err)

	if offset < 0 || limit < 1 || limit > maxLibraryPage {
		return LibraryPage{}, apperr.New(apperr.InvalidInput, "This part of the fetched images cannot be shown.", nil)
	}
	entries, err := a.library.List(a.ctx, library.ListOptions{Origin: library.Fetched, Limit: limit + 1, Offset: offset})
	if err != nil {
		return LibraryPage{}, apperr.New(apperr.Unavailable, "Fetched images could not be loaded.", err)
	}
	page.HasMore = len(entries) > limit
	entries = entries[:min(len(entries), limit)]
	page.Items = make([]LibraryItem, len(entries))
	for i, e := range entries {
		page.Items[i] = libraryItem(e)
	}
	a.markHidden(page.Items)
	return page, nil
}

// GetFetchedStats returns the number and size of fetched images and the
// limit they are kept to.
func (a *App) GetFetchedStats() (stats FetchedStats, err error) {
	defer a.recoverPanic("GetFetchedStats", &err)

	st, err := a.library.FetchedStats(a.ctx)
	if err != nil {
		return FetchedStats{}, apperr.New(apperr.Unavailable, "Fetched images could not be loaded.", err)
	}
	return FetchedStats{Images: st.Images, Bytes: st.Bytes, MaxBytes: fetched.MaxBytes}, nil
}

// KeepFetched moves a fetched image into the user's library.
func (a *App) KeepFetched(id int64) (item LibraryItem, err error) {
	defer a.recoverPanic("KeepFetched", &err)

	e, err := a.fetched.Keep(a.ctx, id)
	switch {
	case errors.Is(err, fetched.ErrNotFetched):
		return LibraryItem{}, apperr.New(apperr.InvalidInput, "This image is already in your library.", err)
	case errors.Is(err, library.ErrNotFound):
		return LibraryItem{}, apperr.New(apperr.InvalidInput, "This image no longer exists.", err)
	case err != nil:
		return LibraryItem{}, apperr.New(apperr.Unavailable, "The image could not be kept.", err)
	}
	return libraryItem(e), nil
}

// SaveDisplaySize stores the size fetched images must cover. The window
// calls it at startup with the largest screen size.
func (a *App) SaveDisplaySize(width, height int) (err error) {
	defer a.recoverPanic("SaveDisplaySize", &err)

	if err := a.settings.SaveDisplay(a.ctx, settings.DisplaySize{Width: width, Height: height}); err != nil {
		return apperr.New(apperr.InvalidInput, "The screen size could not be saved.", err)
	}
	return nil
}
