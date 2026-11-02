// Package rotation changes the wallpaper to the next library image.
//
// It chooses the image (package documentation in pick.go), records what
// was shown, and sets it with the wallpaper package. Scheduling is done
// outside: by a systemd user timer that runs `kisetsu rotate`.
package rotation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sentivanac/kisetsu/internal/settings"
)

const settingsKey = "rotation"

// Interval is how often the wallpaper changes.
type Interval string

const (
	Every15Minutes Interval = "15m"
	Every30Minutes Interval = "30m"
	Hourly         Interval = "1h"
	Every3Hours    Interval = "3h"
	Daily          Interval = "24h"
)

// Intervals lists the offered intervals, shortest first.
var Intervals = []Interval{Every15Minutes, Every30Minutes, Hourly, Every3Hours, Daily}

// Duration returns the interval as a time.Duration.
func (i Interval) Duration() (time.Duration, error) {
	for _, known := range Intervals {
		if i == known {
			return time.ParseDuration(string(i))
		}
	}
	return 0, fmt.Errorf("unknown interval %q", i)
}

// Order is how the next image is chosen.
type Order string

const (
	// Random shows every image once per cycle, in random order.
	Random Order = "random"
	// Sequential shows images in the order they were added.
	Sequential Order = "sequential"
)

// Settings are the rotation preferences.
type Settings struct {
	Enabled      bool     `json:"enabled"`
	Interval     Interval `json:"interval"`
	CollectionID int64    `json:"collectionId"` // 0 = the whole library
	// SavedSearchID, when set, rotates the images fetched for that saved
	// search instead of the library (ADR 0030).
	SavedSearchID int64 `json:"savedSearchId"`
	Order         Order `json:"order"`
}

// DefaultSettings are used until the user changes them.
var DefaultSettings = Settings{Interval: Hourly, Order: Random}

// Validate checks that s can be used.
func (s Settings) Validate() error {
	var errs []error
	if _, err := s.Interval.Duration(); err != nil {
		errs = append(errs, err)
	}
	if s.Order != Random && s.Order != Sequential {
		errs = append(errs, fmt.Errorf("unknown order %q", s.Order))
	}
	if s.CollectionID < 0 || s.SavedSearchID < 0 {
		errs = append(errs, fmt.Errorf("invalid source %d/%d", s.CollectionID, s.SavedSearchID))
	}
	if s.CollectionID != 0 && s.SavedSearchID != 0 {
		errs = append(errs, errors.New("choose a collection or a saved search, not both"))
	}
	return errors.Join(errs...)
}

// LoadSettings returns the saved settings, or the defaults when none are
// saved or the saved ones are unusable: a bad preference must not stop
// the wallpaper from changing.
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
