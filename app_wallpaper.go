package main

import (
	"context"
	"errors"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// unsupportedDesktopMessage is shown when neither Variety nor a supported
// desktop can set the wallpaper.
const unsupportedDesktopMessage = "Kisetsu cannot set the wallpaper on this desktop yet. GNOME, MATE, Cinnamon and Variety are supported."

// Unsupported is the apperr code for features the user's system lacks.
const Unsupported apperr.Code = "unsupported"

// WallpaperSetter sets the desktop wallpaper; *wallpaper.Setter in the app.
type WallpaperSetter interface {
	Set(ctx context.Context, path string) (wallpaper.Method, error)
	// VarietyRunning reports whether Variety would change the wallpaper too.
	VarietyRunning() bool
}

// WallpaperResult tells the frontend how the wallpaper was set.
type WallpaperResult struct {
	Method  string `json:"method"`
	Message string `json:"message"`
}

// SetWallpaper makes a library image the desktop wallpaper.
func (a *App) SetWallpaper(imageID int64) (result WallpaperResult, err error) {
	defer a.recoverPanic("SetWallpaper", &err)

	e, err := a.library.Get(a.ctx, imageID)
	if errors.Is(err, library.ErrNotFound) {
		return WallpaperResult{}, apperr.New(apperr.InvalidInput, "This image is no longer in your library.", err)
	}
	if err != nil {
		return WallpaperResult{}, apperr.New(apperr.Unavailable, "The wallpaper could not be set.", err)
	}
	path, err := a.files.OriginalPath(e.FilePath)
	if err != nil {
		return WallpaperResult{}, apperr.New(apperr.Unavailable,
			"The image file is missing from your library, so it cannot be used as wallpaper.", err)
	}

	method, err := a.wallpaper.Set(a.ctx, path)
	switch {
	case errors.Is(err, wallpaper.ErrUnsupportedDesktop):
		return WallpaperResult{}, apperr.New(Unsupported, unsupportedDesktopMessage, err)
	case err != nil:
		return WallpaperResult{}, apperr.New(apperr.Unavailable, "The wallpaper could not be set.", err)
	}

	a.log.Info("wallpaper set", "entry", imageID, "method", method)
	// Rotation must know what is on screen. The wallpaper is already set,
	// so a failure here is only logged.
	if err := a.rotation.RecordManual(a.ctx, imageID); err != nil {
		a.log.Warn("wallpaper not recorded in history", "entry", imageID, "err", err)
	}
	a.restartRotationTimer()
	result = WallpaperResult{Method: string(method)}
	switch method {
	case wallpaper.Variety:
		result.Message = "Set through Variety. Variety may replace it at its next scheduled change."
	default:
		result.Message = "Set as your wallpaper."
	}
	return result, nil
}
