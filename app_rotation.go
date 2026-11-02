package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/schedule"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// Scheduler installs the rotation timer; *schedule.Manager in the app.
type Scheduler interface {
	Enable(ctx context.Context, interval time.Duration) error
	Disable(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (schedule.Status, error)
}

// RotationSettings mirrors rotation.Settings for the frontend.
type RotationSettings struct {
	Enabled      bool   `json:"enabled"`
	Interval     string `json:"interval"`
	CollectionID int64  `json:"collectionId"`
	Order        string `json:"order"`
}

// TimerStatus describes the systemd timer for the frontend.
type TimerStatus struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	NextRun   string `json:"nextRun"` // RFC 3339, "" when unknown
	// Outdated means the timer runs another Kisetsu binary (moved or
	// rebuilt elsewhere); RepairRotationTimer fixes it.
	Outdated bool   `json:"outdated"`
	Error    string `json:"error,omitempty"`
}

// RotationState is everything the Wallpaper tab shows.
type RotationState struct {
	Settings       RotationSettings `json:"settings"`
	Intervals      []string         `json:"intervals"`
	Timer          TimerStatus      `json:"timer"`
	LastImageID    int64            `json:"lastImageId"`
	LastChange     string           `json:"lastChange"` // RFC 3339, "" if none
	VarietyRunning bool             `json:"varietyRunning"`
}

// GetRotation returns the rotation settings and status.
func (a *App) GetRotation() (state RotationState, err error) {
	defer a.recoverPanic("GetRotation", &err)

	cfg, err := rotation.LoadSettings(a.ctx, a.settings)
	if err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "Rotation settings could not be loaded.", err)
	}
	state.Settings = RotationSettings{
		Enabled: cfg.Enabled, Interval: string(cfg.Interval), CollectionID: cfg.CollectionID, Order: string(cfg.Order),
	}
	for _, i := range rotation.Intervals {
		state.Intervals = append(state.Intervals, string(i))
	}

	st, err := a.schedule.Status(a.ctx)
	if err != nil {
		// The settings are still useful without timer details.
		a.log.Warn("cannot read rotation timer status", "err", err)
		state.Timer.Error = "The timer status could not be read."
	}
	state.Timer.Installed, state.Timer.Active = st.Installed, st.Active
	state.Timer.Outdated = st.Outdated(a.executable)
	if !st.NextRun.IsZero() {
		state.Timer.NextRun = st.NextRun.Format(time.RFC3339)
	}

	if last, ok, err := a.rotation.History.Last(a.ctx); err == nil && ok {
		state.LastImageID = last.ImageID
		state.LastChange = last.At.Format(time.RFC3339)
	}
	state.VarietyRunning = a.wallpaper.VarietyRunning()
	return state, nil
}

// SaveRotation stores the rotation settings and installs, updates or
// removes the timer to match. When enabling fails, rotation stays off, so
// the settings never claim a timer that does not exist.
func (a *App) SaveRotation(s RotationSettings) (state RotationState, err error) {
	defer a.recoverPanic("SaveRotation", &err)

	cfg := rotation.Settings{
		Enabled:      s.Enabled,
		Interval:     rotation.Interval(s.Interval),
		CollectionID: s.CollectionID,
		Order:        rotation.Order(s.Order),
	}
	if err := cfg.Validate(); err != nil {
		return RotationState{}, apperr.New(apperr.InvalidInput, "These rotation settings are not valid.", err)
	}

	if cfg.Enabled {
		interval, _ := cfg.Interval.Duration() // validated above
		if err := a.schedule.Enable(a.ctx, interval); err != nil {
			cfg.Enabled = false
			if saveErr := rotation.SaveSettings(a.ctx, a.settings, cfg); saveErr != nil {
				a.log.Warn("rotation settings not saved", "err", saveErr)
			}
			return RotationState{}, timerError(err)
		}
	} else if err := a.schedule.Disable(a.ctx); err != nil {
		return RotationState{}, timerError(err)
	}

	if err := rotation.SaveSettings(a.ctx, a.settings, cfg); err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "Rotation settings could not be saved.", err)
	}
	a.log.Info("rotation settings saved", "enabled", cfg.Enabled, "interval", cfg.Interval,
		"collection", cfg.CollectionID, "order", cfg.Order)
	return a.GetRotation()
}

// RepairRotationTimer reinstalls the timer for the running Kisetsu binary,
// for example after Kisetsu was moved.
func (a *App) RepairRotationTimer() (state RotationState, err error) {
	defer a.recoverPanic("RepairRotationTimer", &err)

	cfg, err := rotation.LoadSettings(a.ctx, a.settings)
	if err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "Rotation settings could not be loaded.", err)
	}
	if cfg.Enabled {
		interval, _ := cfg.Interval.Duration()
		if err := a.schedule.Enable(a.ctx, interval); err != nil {
			return RotationState{}, timerError(err)
		}
	}
	return a.GetRotation()
}

// NextWallpaper changes the wallpaper now, as the timer would.
func (a *App) NextWallpaper() (state RotationState, err error) {
	defer a.recoverPanic("NextWallpaper", &err)

	unlock, err := rotation.Lock(a.rotationLock)
	if errors.Is(err, rotation.ErrLocked) {
		return RotationState{}, apperr.New(apperr.InvalidInput, "The wallpaper is already being changed. Try again in a moment.", err)
	}
	if err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be changed.", err)
	}
	defer unlock()

	if _, err := a.rotation.Next(a.ctx); err != nil {
		if errors.Is(err, rotation.ErrNoImages) {
			return RotationState{}, apperr.New(apperr.InvalidInput,
				"There are no images to show. Download images, or choose another collection.", err)
		}
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be changed.", err)
	}
	a.restartRotationTimer()
	return a.GetRotation()
}

// restartRotationTimer makes the next automatic change come one full
// interval after a change the user made by hand. The wallpaper has already
// changed, so a failure is only logged.
func (a *App) restartRotationTimer() {
	restartAfterManualChange(a.ctx, a.settings, a.schedule, a.log)
}

func timerError(err error) error {
	switch {
	case errors.Is(err, schedule.ErrForeignUnit):
		return apperr.New(apperr.InvalidInput,
			"A systemd unit named kisetsu-rotate already exists and was not created by Kisetsu, so it was left alone. "+
				"Rename or remove it in ~/.config/systemd/user to use rotation.", err)
	case errors.Is(err, schedule.ErrTemporaryExecutable):
		return apperr.New(apperr.InvalidInput,
			"Kisetsu is running from a temporary build. Build it (make build) and start that copy to enable rotation.", err)
	}
	return apperr.New(apperr.Unavailable, "The rotation timer could not be set up. Details were written to the log file.", err)
}

// restartAfterManualChange restarts the rotation timer when rotation is on.
// The window and `kisetsu rotate` both use it.
func restartAfterManualChange(ctx context.Context, store *settings.Store, timer Scheduler, log *slog.Logger) {
	cfg, err := rotation.LoadSettings(ctx, store)
	if err != nil || !cfg.Enabled {
		return
	}
	if err := timer.Restart(ctx); err != nil {
		log.Warn("rotation timer not restarted after a manual change", "err", err)
	}
}
