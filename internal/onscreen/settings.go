package onscreen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

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

// Level is how strong an effect is.
type Level string

const (
	LevelOff    Level = "off"
	LevelLight  Level = "light"
	LevelMedium Level = "medium"
	LevelStrong Level = "strong"
)

// darkenShares and blurShares give each level's strength: the share of
// brightness taken away, and the blur radius as a share of the screen
// height (5 px on a 1080 px screen for medium).
var (
	darkenShares = map[Level]float64{LevelOff: 0, LevelLight: 0.2, LevelMedium: 0.35, LevelStrong: 0.5}
	blurShares   = map[Level]float64{LevelOff: 0, LevelLight: 0.002, LevelMedium: 0.005, LevelStrong: 0.012}
)

// Tone is how the colors of the image are shown.
type Tone string

const (
	ToneOriginal Tone = "original"
	ToneGray     Tone = "black-and-white"
	ToneSepia    Tone = "sepia"
)

var tones = map[Tone]overlay.Tone{ToneOriginal: overlay.Original, ToneGray: overlay.Gray, ToneSepia: overlay.Sepia}

// vignetteStrength is how much the corners are darkened.
const vignetteStrength = 0.5

// Settings say what is drawn on the wallpaper: text and effects.
// Everything is off by default.
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

	// TextFile shows the content of the file at TextFilePath, redrawn by
	// the agent when it changes (ADR 0051).
	TextFile       bool           `json:"textFile"`
	TextFilePath   string         `json:"textFilePath"`
	TextFileCorner overlay.Corner `json:"textFileCorner"`

	// Effects change the whole image (ADR 0050).
	Darken   Level `json:"darken"`
	Blur     Level `json:"blur"`
	Tone     Tone  `json:"tone"`
	Vignette bool  `json:"vignette"`
}

// DefaultSettings are used until the user changes them. Settings saved
// before the clock existed get the clock's defaults.
var DefaultSettings = Settings{
	CreditCorner:   overlay.BottomLeft,
	ClockCorner:    overlay.BottomRight,
	ClockDate:      true,
	ClockSize:      Medium,
	TextFileCorner: overlay.TopRight,
	Darken:         LevelOff,
	Blur:           LevelOff,
	Tone:           ToneOriginal,
}

// Any reports whether anything is drawn: text or an effect.
func (s Settings) Any() bool {
	return s.Credit || s.Live() || !s.effects().None()
}

// Live reports whether anything is drawn that the agent keeps up to date:
// the clock or the text file.
func (s Settings) Live() bool {
	return s.Clock || s.TextFile
}

// effects returns the effects to apply to the image.
func (s Settings) effects() overlay.Effects {
	e := overlay.Effects{Darken: darkenShares[s.Darken], Blur: blurShares[s.Blur], Tone: tones[s.Tone]}
	if s.Vignette {
		e.Vignette = vignetteStrength
	}
	return e
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
	if !s.TextFileCorner.Valid() {
		errs = append(errs, fmt.Errorf("unknown corner %q", s.TextFileCorner))
	}
	if s.TextFile && !filepath.IsAbs(s.TextFilePath) {
		errs = append(errs, fmt.Errorf("text file %q is not an absolute path", s.TextFilePath))
	}
	if _, ok := s.ClockSize.share(); !ok {
		errs = append(errs, fmt.Errorf("unknown clock size %q", s.ClockSize))
	}
	if _, ok := darkenShares[s.Darken]; !ok {
		errs = append(errs, fmt.Errorf("unknown darken level %q", s.Darken))
	}
	if _, ok := blurShares[s.Blur]; !ok {
		errs = append(errs, fmt.Errorf("unknown blur level %q", s.Blur))
	}
	if _, ok := tones[s.Tone]; !ok {
		errs = append(errs, fmt.Errorf("unknown tone %q", s.Tone))
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
