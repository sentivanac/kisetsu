package main

import (
	"context"
	"errors"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// errNoCurrent means Kisetsu never set a wallpaper, so there is no
// current one to act on.
var errNoCurrent = errors.New("no wallpaper set by Kisetsu")

// HideImage keeps an image out of rotation for good, without deleting it
// (ADR 0054).
func (a *App) HideImage(id int64) (err error) {
	defer a.recoverPanic("HideImage", &err)
	if err := a.library.Hide(a.ctx, id); errors.Is(err, library.ErrNotFound) {
		return apperr.New(apperr.InvalidInput, "This image is no longer in Kisetsu.", err)
	} else if err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be hidden.", err)
	}
	a.log.Info("image hidden from rotation", "entry", id)
	return nil
}

// ShowImageAgain lets a hidden image back into rotation.
func (a *App) ShowImageAgain(id int64) (err error) {
	defer a.recoverPanic("ShowImageAgain", &err)
	if err := a.library.Unhide(a.ctx, id); err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be shown again.", err)
	}
	a.log.Info("image back in rotation", "entry", id)
	return nil
}

// ListHidden returns the images kept out of rotation, most recently
// hidden first.
func (a *App) ListHidden() (items []LibraryItem, err error) {
	defer a.recoverPanic("ListHidden", &err)
	entries, err := a.library.Hidden(a.ctx)
	if err != nil {
		return nil, apperr.New(apperr.Unavailable, "The hidden images could not be loaded.", err)
	}
	items = make([]LibraryItem, len(entries))
	for i, e := range entries {
		items[i] = libraryItem(e)
		items[i].Hidden = true
	}
	return items, nil
}

// HideCurrentWallpaper hides the wallpaper on screen and shows the next
// one: "Don't show this again".
func (a *App) HideCurrentWallpaper() (state RotationState, err error) {
	defer a.recoverPanic("HideCurrentWallpaper", &err)
	unlock, err := rotation.Lock(a.rotationLock)
	if errors.Is(err, rotation.ErrLocked) {
		return RotationState{}, apperr.New(apperr.InvalidInput, "The wallpaper is already being changed. Try again in a moment.", err)
	}
	if err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be changed.", err)
	}
	defer unlock()
	switch _, err := hideCurrentWallpaper(a.ctx, a.library, a.rotation); {
	case errors.Is(err, errNoCurrent):
		return RotationState{}, apperr.New(apperr.InvalidInput, "Kisetsu has not set a wallpaper yet.", err)
	case errors.Is(err, wallpaper.ErrUnsupportedDesktop):
		return RotationState{}, apperr.New(Unsupported, unsupportedDesktopMessage, err)
	case err != nil:
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be changed.", err)
	}
	return a.GetRotation()
}

// hideCurrentWallpaper hides the image Kisetsu set last and shows the next
// one; with nothing else left to show, the hidden image stays on screen
// until there is. The caller holds the rotation lock. Shared by the window
// and the tray.
func hideCurrentWallpaper(ctx context.Context, lib *library.Store, r *rotation.Rotator) (int64, error) {
	last, changed, err := r.History.Last(ctx)
	if err != nil {
		return 0, err
	}
	if !changed {
		return 0, errNoCurrent
	}
	if err := lib.Hide(ctx, last.ImageID); err != nil {
		return 0, err
	}
	r.Log.Info("current wallpaper hidden from rotation", "entry", last.ImageID)
	if _, err := r.Next(ctx); err != nil && !errors.Is(err, rotation.ErrNoImages) {
		return last.ImageID, err
	}
	return last.ImageID, nil
}

// AddToFavorites puts an image in the Favorites collection, made the
// first time; a fetched image is kept in the library first.
func (a *App) AddToFavorites(id int64) (err error) {
	defer a.recoverPanic("AddToFavorites", &err)
	if err := a.favs.add(a.ctx, id); errors.Is(err, library.ErrNotFound) {
		return apperr.New(apperr.InvalidInput, "This image is no longer in Kisetsu.", err)
	} else if err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be added to your favorites.", err)
	}
	a.log.Info("image added to favorites", "entry", id)
	return nil
}

// RemoveFromFavorites takes an image out of Favorites; it stays where it
// is.
func (a *App) RemoveFromFavorites(id int64) (err error) {
	defer a.recoverPanic("RemoveFromFavorites", &err)
	if err := a.favs.remove(a.ctx, id); err != nil {
		return apperr.New(apperr.Unavailable, "The image could not be removed from your favorites.", err)
	}
	return nil
}

// FavoriteCurrentWallpaper adds the wallpaper on screen to Favorites.
func (a *App) FavoriteCurrentWallpaper() (state RotationState, err error) {
	defer a.recoverPanic("FavoriteCurrentWallpaper", &err)
	switch err := favoriteCurrentWallpaper(a.ctx, a.favs, a.rotation.History); {
	case errors.Is(err, errNoCurrent):
		return RotationState{}, apperr.New(apperr.InvalidInput, "Kisetsu has not set a wallpaper yet.", err)
	case err != nil:
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be added to your favorites.", err)
	}
	return a.GetRotation()
}

// favoriteCurrentWallpaper adds the image Kisetsu set last to Favorites;
// shared by the window and the tray.
func favoriteCurrentWallpaper(ctx context.Context, favs favorites, history *rotation.History) error {
	last, changed, err := history.Last(ctx)
	if err != nil {
		return err
	}
	if !changed {
		return errNoCurrent
	}
	return favs.add(ctx, last.ImageID)
}
