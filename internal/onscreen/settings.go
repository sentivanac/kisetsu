package onscreen

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentivanac/kisetsu/internal/overlay"
	"github.com/sentivanac/kisetsu/internal/settings"
)

const settingsKey = "overlay"

// ClockSize is how large the clock is drawn.
type ClockSize string

const (
	Small  ClockSize = "small"
	Medium ClockSize = "medium"
	Large  ClockSize = "large"
)

// share is the time's font size as a share of the screen height.
func (s ClockSize) share() (float64, bool) {
	switch s {
	case Small:
		return 0.05, true
	case Medium:
		return 0.075, true
	case Large:
		return 0.11, true
	}
	return 0, false
}

// Settings say what text is drawn on the wallpaper. Everything is off by
// default.
type Settings struct {
	// Credit shows the image's author, license and source.
	Credit       bool           `json:"credit"`
	CreditCorner overlay.Corner `json:"creditCorner"`

	// Clock shows the time, redrawn every minute by the agent.
	Clock       bool           `json:"clock"`
	ClockCorner overlay.Corner `json:"clockCorner"`
	// Clock12h shows "3:04 PM" instead of "15:04".
	Clock12h  bool      `json:"clock12h"`
	ClockDate bool      `json:"clockDate"`
	ClockSize ClockSize `json:"clockSize"`
}

// DefaultSettings are used until the user changes them. Settings saved
// before the clock existed get the clock's defaults.
var DefaultSettings = Settings{
	CreditCorner: overlay.BottomLeft,
	ClockCorner:  overlay.BottomRight,
	ClockDate:    true,
	ClockSize:    Medium,
}

// Any reports whether any text is drawn.
func (s Settings) Any() bool {
	return s.Credit || s.Clock
}

// Validate checks that s can be used.
func (s Settings) Validate() error {
	var errs []error
	if !s.CreditCorner.Valid() {
		errs = append(errs, fmt.Errorf("unknown corner %q", s.CreditCorner))
	}
	if !s.ClockCorner.Valid() {
		errs = append(errs, fmt.Errorf("unknown corner %q", s.ClockCorner))
	}
	if _, ok := s.ClockSize.share(); !ok {
		errs = append(errs, fmt.Errorf("unknown clock size %q", s.ClockSize))
	}
	return errors.Join(errs...)
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
