package onscreen

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/overlay"
	"github.com/sentivanac/kisetsu/internal/settings"
)

const settingsKey = "overlay"

// Settings say what text is drawn on the wallpaper. Everything is off by
// default.
type Settings struct {
	// Credit shows the image's author, license and source.
	Credit       bool           `json:"credit"`
	CreditCorner overlay.Corner `json:"creditCorner"`
}

// DefaultSettings are used until the user changes them.
var DefaultSettings = Settings{CreditCorner: overlay.BottomLeft}

// Any reports whether any text is drawn.
func (s Settings) Any() bool {
	return s.Credit
}

// Validate checks that s can be used.
func (s Settings) Validate() error {
	if !s.CreditCorner.Valid() {
		return fmt.Errorf("unknown corner %q", s.CreditCorner)
	}
	return nil
}

// LoadSettings returns the saved settings, or the defaults when none are
// saved or the saved ones are unusable.
func LoadSettings(ctx context.Context, store *settings.Store) (Settings, error) {
	s := DefaultSettings
	found, err := store.Load(ctx, settingsKey, &s)
	switch {
	case errors.Is(err, settings.ErrInvalidValue):
		return DefaultSettings, nil
	case err != nil:
		return DefaultSettings, err
	case !found || s.Validate() != nil:
		return DefaultSettings, nil
	}
	return s, nil
}

// SaveSettings stores s after validating it.
func SaveSettings(ctx context.Context, store *settings.Store, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	return store.Save(ctx, settingsKey, s)
}
