package main

import (
	"context"
	"errors"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/overlay"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// unsupportedDesktopMessage is shown when neither Variety nor a supported
// desktop can set the wallpaper.
const unsupportedDesktopMessage = "Kisetsu cannot set the wallpaper on this desktop yet. GNOME, KDE Plasma, Xfce, MATE, Cinnamon and Variety are supported."

// Unsupported is the apperr code for features the user's system lacks.
const Unsupported apperr.Code = "unsupported"

// WallpaperSetter sets the desktop wallpaper; *wallpaper.Setter in the app.
type WallpaperSetter interface {
	Set(ctx context.Context, path string) (wallpaper.Method, error)
	// VarietyRunning reports whether Variety would change the wallpaper too.
	VarietyRunning() bool
	// ViaVariety reports whether Set hands the image to Variety.
	ViaVariety() bool
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

	method, err := a.screen.Show(a.ctx, e, path)
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
	result = WallpaperResult{Method: string(method)}
	switch method {
	case wallpaper.Variety:
		result.Message = "Set through Variety. Variety may replace it at its next scheduled change."
	default:
		result.Message = "Set as your wallpaper."
	}
	return result, nil
}

// WallpaperText mirrors onscreen.Settings for the frontend: the text drawn
// on the wallpaper (ADR 0047).
type WallpaperText struct {
	Credit       bool   `json:"credit"`
	CreditCorner string `json:"creditCorner"`
}

func wallpaperText(s onscreen.Settings) WallpaperText {
	return WallpaperText{Credit: s.Credit, CreditCorner: string(s.CreditCorner)}
}

// SaveWallpaperText stores what text is drawn on the wallpaper and shows
// the current wallpaper again with it (or without it, when turned off).
func (a *App) SaveWallpaperText(t WallpaperText) (state RotationState, err error) {
	defer a.recoverPanic("SaveWallpaperText", &err)

	cfg := onscreen.Settings{Credit: t.Credit, CreditCorner: overlay.Corner(t.CreditCorner)}
	if err := cfg.Validate(); err != nil {
		return RotationState{}, apperr.New(apperr.InvalidInput, "These settings are not valid.", err)
	}
	old, err := onscreen.LoadSettings(a.ctx, a.settings)
	if err != nil {
		old = onscreen.DefaultSettings
	}
	if err := onscreen.SaveSettings(a.ctx, a.settings, cfg); err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "The setting could not be saved.", err)
	}
	a.log.Info("wallpaper text settings saved", "credit", cfg.Credit, "corner", cfg.CreditCorner)
	if cfg != old && (cfg.Any() || old.Any()) {
		a.showCurrentAgain()
	}
	return a.GetRotation()
}

// showCurrentAgain shows the wallpaper Kisetsu set last again, so a change
// of its text shows at once. It is skipped while Variety sets the
// wallpaper (Kisetsu draws no text then) or while another change is in
// progress (which uses the new settings anyway). Problems are only logged:
// the setting is saved and applies from the next change.
func (a *App) showCurrentAgain() {
	if a.wallpaper.ViaVariety() {
		return
	}
	unlock, err := rotation.Lock(a.rotationLock)
	if err != nil {
		a.log.Debug("wallpaper not shown again", "err", err)
		return
	}
	defer unlock()

	last, changed, err := a.rotation.History.Last(a.ctx)
	if err != nil || !changed {
		return
	}
	e, err := a.library.Get(a.ctx, last.ImageID)
	if err != nil {
		a.log.Warn("wallpaper not shown again", "entry", last.ImageID, "err", err)
		return
	}
	path, err := a.files.OriginalPath(e.FilePath)
	if err != nil {
		a.log.Warn("wallpaper not shown again", "entry", last.ImageID, "err", err)
		return
	}
	if _, err := a.screen.Show(a.ctx, e, path); err != nil {
		a.log.Warn("wallpaper not shown again", "entry", last.ImageID, "err", err)
	}
}
