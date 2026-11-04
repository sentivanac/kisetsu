package agent

import (
	"context"
	"errors"

	"github.com/sentivanac/kisetsu/internal/settings"
)

const prefsKey = "agent"

// Prefs are the agent's own preferences.
type Prefs struct {
	// ShowInTray shows Kisetsu's icon in the system tray. The agent runs
	// while this or rotation is on.
	ShowInTray bool `json:"showInTray"`
}

// LoadPrefs returns the saved preferences, or the defaults (tray off) when
// none are saved or they are unreadable.
func LoadPrefs(ctx context.Context, store *settings.Store) (Prefs, error) {
	var p Prefs
	_, err := store.Load(ctx, prefsKey, &p)
	if errors.Is(err, settings.ErrInvalidValue) {
		return Prefs{}, nil
	}
	return p, err
}

// SavePrefs stores p.
func SavePrefs(ctx context.Context, store *settings.Store, p Prefs) error {
	return store.Save(ctx, prefsKey, p)
}
