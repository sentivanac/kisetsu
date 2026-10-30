package providers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
)

func TestQueryValidate(t *testing.T) {
	valid := []Query{
		{Page: 1},
		{Text: "misty mountains", Page: 3, MinWidth: 1920, MinHeight: 1080, Orientation: images.Landscape},
		{Text: strings.Repeat("季", MaxQueryLength), Page: 1}, // counts characters, not bytes
	}
	for _, q := range valid {
		if err := q.Validate(); err != nil {
			t.Errorf("%+v: unexpected error: %v", q, err)
		}
	}

	invalid := map[string]Query{
		"page zero":       {Page: 0},
		"negative width":  {Page: 1, MinWidth: -1},
		"bad orientation": {Page: 1, Orientation: "diagonal"},
		"text too long":   {Page: 1, Text: strings.Repeat("a", MaxQueryLength+1)},
	}
	for name, q := range invalid {
		if err := q.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestQueryMatches(t *testing.T) {
	landscape := images.Image{Width: 2560, Height: 1440}
	portrait := images.Image{Width: 1080, Height: 1920}

	tests := []struct {
		name  string
		query Query
		img   images.Image
		want  bool
	}{
		{"no filters", Query{}, portrait, true},
		{"large enough", Query{MinWidth: 1920, MinHeight: 1080}, landscape, true},
		{"too narrow", Query{MinWidth: 3840}, landscape, false},
		{"too short", Query{MinHeight: 2000}, landscape, false},
		{"orientation matches", Query{Orientation: images.Portrait}, portrait, true},
		{"orientation differs", Query{Orientation: images.Portrait}, landscape, false},
	}
	for _, tt := range tests {
		if got := tt.query.Matches(tt.img); got != tt.want {
			t.Errorf("%s: Matches() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestErrorSupportsIsAndAs(t *testing.T) {
	cause := context.DeadlineExceeded
	var err error = &Error{Provider: "test", Kind: ErrUnavailable, Err: cause}

	if !errors.Is(err, ErrUnavailable) {
		t.Error("errors.Is(err, ErrUnavailable) = false")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("cause is not reachable with errors.Is")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Error("errors.Is(err, ErrRateLimited) = true")
	}
	if pe, ok := errors.AsType[*Error](err); !ok || pe.Provider != "test" {
		t.Errorf("errors.AsType = %v, %v", pe, ok)
	}
}

func TestErrorMessage(t *testing.T) {
	err := &Error{Provider: "wallhaven", Kind: ErrRateLimited, RetryAfter: 30 * time.Second}

	if got, want := err.Error(), "wallhaven: rate limited (retry after 30s)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
