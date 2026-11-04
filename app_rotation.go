package main

import (
	"context"
	"errors"
	"time"

	"github.com/sentivanac/kisetsu/internal/agent"
	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// LegacyTimer is the systemd timer of Kisetsu 0.1.0, which the agent
// replaces; *legacytimer.Timer in the app.
type LegacyTimer interface {
	Installed() (bool, error)
	Disable(ctx context.Context) error
}

// RotationSettings mirrors rotation.Settings for the frontend.
type RotationSettings struct {
	Enabled      bool   `json:"enabled"`
	Interval     string `json:"interval"`
	CollectionID int64  `json:"collectionId"`
	// SavedSearchID rotates the images fetched for a saved search.
	SavedSearchID int64  `json:"savedSearchId"`
	Order         string `json:"order"`
}

// AgentState describes the background agent for the frontend.
type AgentState struct {
	Running bool `json:"running"`
	// Autostart means the agent starts at login.
	Autostart bool `json:"autostart"`
	// Outdated means the autostart entry starts another Kisetsu binary
	// (moved or rebuilt elsewhere); RepairAgent fixes it.
	Outdated bool   `json:"outdated"`
	Error    string `json:"error,omitempty"`
}

// RotationState is everything the Wallpaper tab shows.
type RotationState struct {
	Settings  RotationSettings `json:"settings"`
	Intervals []string         `json:"intervals"`
	Agent     AgentState       `json:"agent"`
	// ShowInTray shows Kisetsu's icon in the system tray.
	ShowInTray bool `json:"showInTray"`
	// TrayHost is false when the desktop shows no tray icons (plain GNOME
	// without the AppIndicator extension).
	TrayHost bool `json:"trayHost"`
	// NextChange is when rotation changes the wallpaper next (RFC 3339),
	// "" when rotation is off.
	NextChange     string `json:"nextChange"`
	LastImageID    int64  `json:"lastImageId"`
	LastChange     string `json:"lastChange"` // RFC 3339, "" if none
	VarietyRunning bool   `json:"varietyRunning"`
	// SourceImages is how many images the chosen source has now.
	SourceImages int `json:"sourceImages"`
}

// GetRotation returns the rotation settings and status.
func (a *App) GetRotation() (state RotationState, err error) {
	defer a.recoverPanic("GetRotation", &err)

	cfg, err := rotation.LoadSettings(a.ctx, a.settings)
	if err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "Rotation settings could not be loaded.", err)
	}
	state.Settings = RotationSettings{
		Enabled: cfg.Enabled, Interval: string(cfg.Interval), CollectionID: cfg.CollectionID,
		SavedSearchID: cfg.SavedSearchID, Order: string(cfg.Order),
	}
	var ids []int64
	if cfg.SavedSearchID != 0 {
		ids, err = a.library.FetchedIDs(a.ctx, cfg.SavedSearchID)
	} else {
		ids, err = a.library.IDs(a.ctx, cfg.CollectionID)
	}
	if err == nil {
		state.SourceImages = len(ids)
	}
	for _, i := range rotation.Intervals {
		state.Intervals = append(state.Intervals, string(i))
	}

	st, err := a.agent.Status()
	if err != nil {
		// The settings are still useful without agent details.
		a.log.Warn("cannot read agent status", "err", err)
		state.Agent.Error = "The state of the background process could not be read."
	}
	if prefs, err := agent.LoadPrefs(a.ctx, a.settings); err == nil {
		state.ShowInTray = prefs.ShowInTray
	}
	state.TrayHost = a.trayHost != nil && a.trayHost()
	state.Agent.Running = st.Running
	state.Agent.Autostart = st.Autostart.Installed
	state.Agent.Outdated = st.Autostart.Outdated(a.executable)

	last, changed, err := a.rotation.History.Last(a.ctx)
	if err == nil && changed {
		state.LastImageID = last.ImageID
		state.LastChange = last.At.Format(time.RFC3339)
	}
	if due, on := agent.Due(cfg, last.At, err == nil && changed); on {
		// The agent checks every half minute; an overdue change comes next.
		if now := time.Now(); due.Before(now) {
			due = now
		}
		state.NextChange = due.Format(time.RFC3339)
	}
	state.VarietyRunning = a.wallpaper.VarietyRunning()
	return state, nil
}

// SaveRotation stores the rotation settings and starts the agent (with
// its autostart entry) or removes the entry; the agent exits by itself
// once rotation is off. When the agent cannot be set up, rotation stays
// off, so the settings never claim a rotation that does not run.
func (a *App) SaveRotation(s RotationSettings) (state RotationState, err error) {
	defer a.recoverPanic("SaveRotation", &err)

	cfg := rotation.Settings{
		Enabled:       s.Enabled,
		Interval:      rotation.Interval(s.Interval),
		CollectionID:  s.CollectionID,
		SavedSearchID: s.SavedSearchID,
		Order:         rotation.Order(s.Order),
	}
	if err := cfg.Validate(); err != nil {
		return RotationState{}, apperr.New(apperr.InvalidInput, "These rotation settings are not valid.", err)
	}
	// Saved first: a starting agent with rotation off would exit at once.
	if err := rotation.SaveSettings(a.ctx, a.settings, cfg); err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "Rotation settings could not be saved.", err)
	}

	if cfg.Enabled {
		if err := a.agent.Enable(); err != nil {
			cfg.Enabled = false
			if saveErr := rotation.SaveSettings(a.ctx, a.settings, cfg); saveErr != nil {
				a.log.Warn("rotation settings not saved", "err", saveErr)
			}
			return RotationState{}, agentError(err)
		}
	} else if !a.agentWanted() {
		if err := a.agent.Disable(); err != nil {
			return RotationState{}, agentError(err)
		}
	}
	a.retireTimer()

	a.log.Info("rotation settings saved", "enabled", cfg.Enabled, "interval", cfg.Interval,
		"collection", cfg.CollectionID, "saved_search", cfg.SavedSearchID, "order", cfg.Order)
	return a.GetRotation()
}

// SetShowInTray shows or hides Kisetsu's tray icon. The agent shows it,
// so it is restarted; with rotation off as well, it stops.
func (a *App) SetShowInTray(show bool) (state RotationState, err error) {
	defer a.recoverPanic("SetShowInTray", &err)

	if err := agent.SavePrefs(a.ctx, a.settings, agent.Prefs{ShowInTray: show}); err != nil {
		return RotationState{}, apperr.New(apperr.Unavailable, "The setting could not be saved.", err)
	}
	if err := a.agent.Stop(); err != nil {
		a.log.Warn("agent not stopped", "err", err)
	}
	if a.agentWanted() {
		if err := a.agent.Enable(); err != nil {
			if show {
				if saveErr := agent.SavePrefs(a.ctx, a.settings, agent.Prefs{}); saveErr != nil {
					a.log.Warn("agent preferences not saved", "err", saveErr)
				}
			}
			return RotationState{}, agentError(err)
		}
	} else if err := a.agent.Disable(); err != nil {
		return RotationState{}, agentError(err)
	}
	a.log.Info("tray icon setting saved", "show", show)
	return a.GetRotation()
}

// agentWanted reports whether the agent has anything to do: rotation or
// the tray icon is on. Unreadable settings count as wanted.
func (a *App) agentWanted() bool {
	cfg, err := rotation.LoadSettings(a.ctx, a.settings)
	if err != nil {
		return true
	}
	prefs, err := agent.LoadPrefs(a.ctx, a.settings)
	return err != nil || cfg.Enabled || prefs.ShowInTray
}

// RepairAgent rewrites the autostart entry for the running Kisetsu binary
// (for example after Kisetsu was moved) and starts the agent.
func (a *App) RepairAgent() (state RotationState, err error) {
	defer a.recoverPanic("RepairAgent", &err)

	if a.agentWanted() {
		if err := a.agent.Enable(); err != nil {
			return RotationState{}, agentError(err)
		}
	}
	return a.GetRotation()
}

// StartAgent starts the background agent, for example after it stopped.
func (a *App) StartAgent() (state RotationState, err error) {
	defer a.recoverPanic("StartAgent", &err)

	if err := a.agent.Start(); err != nil {
		return RotationState{}, agentError(err)
	}
	return a.GetRotation()
}

// retireTimer replaces the systemd timer of Kisetsu 0.1.0 with the agent:
// when rotation is on, the agent is set up first, then the timer is
// removed (only if Kisetsu created it). Problems are logged; the timer is
// kept when the agent cannot run.
func (a *App) retireTimer() {
	if a.legacyTimer == nil {
		return
	}
	if installed, err := a.legacyTimer.Installed(); err != nil || !installed {
		return
	}
	if a.agentWanted() {
		if err := a.agent.Enable(); err != nil {
			a.log.Warn("old rotation timer kept: the agent could not be set up", "err", err)
			return
		}
	}
	if err := a.legacyTimer.Disable(a.ctx); err != nil {
		a.log.Warn("old rotation timer not removed", "err", err)
		return
	}
	a.log.Info("old rotation timer replaced by the agent")
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
		if errors.Is(err, wallpaper.ErrUnsupportedDesktop) {
			return RotationState{}, apperr.New(Unsupported, unsupportedDesktopMessage, err)
		}
		return RotationState{}, apperr.New(apperr.Unavailable, "The wallpaper could not be changed.", err)
	}
	// The agent counts the next change from this one.
	return a.GetRotation()
}

func agentError(err error) error {
	switch {
	case errors.Is(err, autostart.ErrForeign):
		return apperr.New(apperr.InvalidInput,
			"A file named "+autostart.FileName+" already exists in ~/.config/autostart and was not created by Kisetsu, "+
				"so it was left alone. Rename or remove it to use rotation.", err)
	case errors.Is(err, autostart.ErrTemporaryExecutable):
		return apperr.New(apperr.InvalidInput,
			"Kisetsu is running from a temporary build. Build it (make build) and start that copy to enable rotation.", err)
	}
	return apperr.New(apperr.Unavailable, "The background process could not be set up. Details were written to the log file.", err)
}
