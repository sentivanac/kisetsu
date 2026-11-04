package main

import (
	"slices"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

func TestHiddenTagsThroughApp(t *testing.T) {
	app := newTestApp(t, &stubProvider{id: "w"})

	if tags, err := app.GetHiddenTags(); err != nil || tags == nil || len(tags) != 0 {
		t.Errorf("defaults: %q (nil: %v), %v", tags, tags == nil, err)
	}
	tags, err := app.SetTagHidden("barefoot", true)
	if err != nil || !slices.Equal(tags, []string{"barefoot"}) {
		t.Errorf("hide: %q, %v", tags, err)
	}
	if tags, err := app.SetTagHidden("barefoot", false); err != nil || tags == nil || len(tags) != 0 {
		t.Errorf("show: %q, %v", tags, err)
	}
	if _, err := app.SetTagHidden(`"`, true); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("invalid tag: err = %v, want invalid_input", err)
	}
}

func TestImageTagsWithoutTags(t *testing.T) {
	app := newTestApp(t, &stubProvider{id: "w"})

	if tags, err := app.ImageTags("w", "abc"); err != nil || tags == nil || len(tags) != 0 {
		t.Errorf("tags = %q, %v; want []", tags, err)
	}
	if _, err := app.ImageTags("nope", "abc"); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("unknown source: err = %v", err)
	}
}
