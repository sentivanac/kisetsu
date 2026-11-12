package main

import (
	"context"
	"errors"
	"strings"

	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// Favorites is an ordinary collection, made the first time an image is
// added to it (ADR 0054). Its ID is remembered, so renaming it keeps it
// the favorites; a collection the user already named so is used instead
// of a second one.
const (
	favoritesKey  = "favorites_collection"
	favoritesName = "Favorites"
)

// favorites adds images to the Favorites collection; shared by the window
// and the tray.
type favorites struct {
	settings *settings.Store
	library  *library.Store
	fetched  *fetched.Service
}

// collection returns the Favorites collection, and false when there is
// none yet and create is not set.
func (f favorites) collection(ctx context.Context, create bool) (int64, bool, error) {
	var id int64
	if found, err := f.settings.Load(ctx, favoritesKey, &id); err == nil && found && id > 0 {
		exists, err := f.library.HasCollection(ctx, id)
		if err != nil {
			return 0, false, err
		}
		if exists {
			return id, true, nil
		}
	}
	all, err := f.library.Collections(ctx)
	if err != nil {
		return 0, false, err
	}
	for _, c := range all {
		if strings.EqualFold(c.Name, favoritesName) || strings.EqualFold(c.Name, "Favourites") {
			return c.ID, true, f.remember(ctx, c.ID)
		}
	}
	if !create {
		return 0, false, nil
	}
	c, err := f.library.CreateCollection(ctx, favoritesName)
	if err != nil {
		return 0, false, err
	}
	return c.ID, true, f.remember(ctx, c.ID)
}

func (f favorites) remember(ctx context.Context, id int64) error {
	return f.settings.Save(ctx, favoritesKey, id)
}

// add puts an image in Favorites. A fetched image is kept in the library
// first, so a favorite never disappears with the cache.
func (f favorites) add(ctx context.Context, imageID int64) error {
	e, err := f.library.Get(ctx, imageID)
	if err != nil {
		return err
	}
	if e.Origin == library.Fetched {
		if _, err := f.fetched.Keep(ctx, imageID); err != nil && !errors.Is(err, fetched.ErrNotFetched) {
			return err
		}
	}
	id, _, err := f.collection(ctx, true)
	if err != nil {
		return err
	}
	return f.library.AddToCollection(ctx, id, imageID)
}

// remove takes an image out of Favorites; the image stays.
func (f favorites) remove(ctx context.Context, imageID int64) error {
	id, ok, err := f.collection(ctx, false)
	if err != nil || !ok {
		return err
	}
	return f.library.RemoveFromCollection(ctx, id, imageID)
}

// members returns the images in Favorites.
func (f favorites) members(ctx context.Context) (map[int64]bool, error) {
	id, ok, err := f.collection(ctx, false)
	if err != nil || !ok {
		return map[int64]bool{}, err
	}
	ids, err := f.library.IDs(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(ids))
	for _, i := range ids {
		out[i] = true
	}
	return out, nil
}
