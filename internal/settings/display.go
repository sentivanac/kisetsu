package settings

import (
	"context"
	"errors"
	"fmt"
)

const displayKey = "display.size"

// maxDisplaySide caps a stored screen side (16K).
const maxDisplaySide = 15360

// DisplaySize is the size fetched images must cover: the largest screen
// width and height, in device pixels. The window stores it, because the
// rotation timer runs without one and cannot ask the screen.
type DisplaySize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DefaultDisplaySize is used until the window has stored the real size.
var DefaultDisplaySize = DisplaySize{Width: 1920, Height: 1080}

func (d DisplaySize) valid() bool {
	return d.Width > 0 && d.Height > 0 && d.Width <= maxDisplaySide && d.Height <= maxDisplaySide
}

// Display returns the stored screen size, or the default. Unusable stored
// values also give the default.
func (s *Store) Display(ctx context.Context) (DisplaySize, error) {
	var d DisplaySize
	found, err := s.Load(ctx, displayKey, &d)
	switch {
	case errors.Is(err, ErrInvalidValue):
		return DefaultDisplaySize, nil
	case err != nil:
		return DefaultDisplaySize, err
	case !found || !d.valid():
		return DefaultDisplaySize, nil
	}
	return d, nil
}

// SaveDisplay stores the screen size.
func (s *Store) SaveDisplay(ctx context.Context, d DisplaySize) error {
	if !d.valid() {
		return fmt.Errorf("invalid display size %dx%d", d.Width, d.Height)
	}
	return s.Save(ctx, displayKey, d)
}
