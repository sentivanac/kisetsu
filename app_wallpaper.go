package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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
	Credit         bool   `json:"credit"`
	CreditCorner   string `json:"creditCorner"`
	Clock          bool   `json:"clock"`
	ClockCorner    string `json:"clockCorner"`
	Clock12h       bool   `json:"clock12h"`
	ClockDate      bool   `json:"clockDate"`
	ClockSize      string `json:"clockSize"`
	TextFile       bool   `json:"textFile"`
	TextFilePath   string `json:"textFilePath"`
	TextFileCorner string `json:"textFileCorner"`
	TextFileSize   string `json:"textFileSize"`
	Darken         string `json:"darken"`
	Blur           string `json:"blur"`
	Tone           string `json:"tone"`
	Vignette       bool   `json:"vignette"`
}

func wallpaperText(s onscreen.Settings) WallpaperText {
	return WallpaperText{
		Credit: s.Credit, CreditCorner: string(s.CreditCorner),
		Clock: s.Clock, ClockCorner: string(s.ClockCorner), Clock12h: s.Clock12h,
		ClockDate: s.ClockDate, ClockSize: string(s.ClockSize),
		TextFile: s.TextFile, TextFilePath: s.TextFilePath, TextFileCorner: string(s.TextFileCorner),
		TextFileSize: string(s.TextFileSize),
		Darken:       string(s.Darken), Blur: string(s.Blur), Tone: string(s.Tone), Vignette: s.Vignette,
	}
}

// SaveWallpaperText stores what text is drawn on the wallpaper and shows
// the current wallpaper again with it (or without it, when turned off).
// The clock needs the agent, which redraws it every minute: it is started
// with the clock, and the clock stays off when it cannot run.
func (a *App) SaveWallpaperText(t WallpaperText) (state RotationState, err error) {
	defer a.recoverPanic("SaveWallpaperText", &err)

	cfg := onscreen.Settings{
		Credit: t.Credit, CreditCorner: overlay.Corner(t.CreditCorner),
		Clock: t.Clock, ClockCorner: overlay.Corner(t.ClockCorner), Clock12h: t.Clock12h,
		ClockDate: t.ClockDate, ClockSize: onscreen.Size(t.ClockSize),
		TextFile: t.TextFile, TextFilePath: t.TextFilePath, TextFileCorner: overlay.Corner(t.TextFileCorner),
		TextFileSize: onscreen.Size(t.TextFileSize),
		Darken:       onscreen.Level(t.Darken), Blur: onscreen.Level(t.Blur), Tone: onscreen.Tone(t.Tone), Vignette: t.Vignette,
	}
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
	// The clock and the text file need the agent, which redraws them.
	if cfg.Live() != old.Live() {
		if err := a.updateAgent(); err != nil {
			if cfg.Live() {
				cfg.Clock, cfg.TextFile = false, false
				if saveErr := onscreen.SaveSettings(a.ctx, a.settings, cfg); saveErr != nil {
					a.log.Warn("wallpaper text settings not saved", "err", saveErr)
				}
			}
			return RotationState{}, err
		}
	}
	a.log.Info("wallpaper text settings saved", "credit", cfg.Credit, "credit_corner", cfg.CreditCorner,
		"clock", cfg.Clock, "clock_corner", cfg.ClockCorner, "clock_12h", cfg.Clock12h,
		"clock_date", cfg.ClockDate, "clock_size", cfg.ClockSize,
		"text_file", cfg.TextFile, "text_file_corner", cfg.TextFileCorner, "text_file_size", cfg.TextFileSize,
		"darken", cfg.Darken, "blur", cfg.Blur, "tone", cfg.Tone, "vignette", cfg.Vignette)
	if cfg != old && (cfg.Any() || old.Any()) {
		a.showCurrentAgain()
	}
	return a.GetRotation()
}

// ChooseTextFile asks for the text file to show on the wallpaper, in the
// system's file dialog. It returns the chosen path, or "" when the dialog
// was closed.
func (a *App) ChooseTextFile() (path string, err error) {
	defer a.recoverPanic("ChooseTextFile", &err)
	path, err = runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a text file to show on the wallpaper",
		Filters: []runtime.FileFilter{
			{DisplayName: "Text files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil {
		return "", apperr.New(apperr.Unavailable, "The file dialog could not be opened.", err)
	}
	return path, nil
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

	e, path, ok, err := currentWallpaper(a.ctx, a.rotation.History, a.library, a.files)
	if err != nil {
		a.log.Warn("wallpaper not shown again", "err", err)
	}
	if !ok {
		return
	}
	if _, err := a.screen.Show(a.ctx, e, path); err != nil {
		a.log.Warn("wallpaper not shown again", "entry", e.ID, "err", err)
	}
}
