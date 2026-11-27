package main

import (
	"context"
	"net/http"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/updates"
)

// updateEvent tells the window that UpdateState may have changed.
const updateEvent = "update:state"

// The first check waits a little: at login, the network may not be up yet.
const (
	windowUpdateDelay = 5 * time.Second
	agentUpdateDelay  = time.Minute
)

// UpdateState is what is known about a newer version.
type UpdateState struct {
	Enabled bool `json:"enabled"`
	// Available is the newer version, or "".
	Available string `json:"available"`
	URL       string `json:"url"`       // its page
	CheckedAt string `json:"checkedAt"` // RFC 3339; "" = never
	// ViaSystem means Kisetsu's APT repository is among the system's
	// package sources: new versions come with the system's updates
	// (ADR 0057).
	ViaSystem bool `json:"viaSystem"`
}

// updateChecker builds the checker shared by the window and the agent.
func (c *core) updateChecker(client *http.Client) *updates.Checker {
	return &updates.Checker{
		Client: client, UserAgent: appinfo.UserAgent(), Version: appinfo.Version,
		ReleaseURL: appinfo.ProjectURL, Settings: settings.NewStore(c.db), Log: c.log,
	}
}

// GetUpdateState returns what the last check found.
func (a *App) GetUpdateState() (state UpdateState, err error) {
	defer a.recoverPanic("GetUpdateState", &err)

	st, err := a.updates.State(a.ctx)
	if err != nil {
		return UpdateState{}, apperr.New(apperr.Unavailable, "The update setting could not be loaded.", err)
	}
	state = UpdateState{Enabled: st.Enabled, Available: st.Available, ViaSystem: updates.FromAPT(updates.AptDir)}
	if st.Available != "" {
		state.URL = a.updates.PageURL(st.Available)
	}
	if !st.CheckedAt.IsZero() {
		state.CheckedAt = st.CheckedAt.Format(time.RFC3339)
	}
	return state, nil
}

// SetUpdateCheck turns the daily check on or off. Turned on, it checks at
// once, if no answer from the last day is known.
func (a *App) SetUpdateCheck(on bool) (state UpdateState, err error) {
	defer a.recoverPanic("SetUpdateCheck", &err)

	if err := a.updates.SetEnabled(a.ctx, on); err != nil {
		return UpdateState{}, apperr.New(apperr.Unavailable, "The setting could not be saved.", err)
	}
	a.log.Info("check for new versions turned on or off", "enabled", on)
	if on {
		if err := a.updates.CheckIfDue(a.ctx); err != nil {
			a.log.Info("no answer about a newer version", "err", err)
		}
	}
	a.emit(updateEvent, nil)
	return a.GetUpdateState()
}

// watchUpdates checks while the window is open.
func (a *App) watchUpdates(ctx context.Context) {
	a.updates.Run(ctx, windowUpdateDelay, func() { a.emit(updateEvent, nil) })
}
