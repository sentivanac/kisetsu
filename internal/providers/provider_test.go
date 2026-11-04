package providers

import (
	"context"
	"errors"
	"slices"
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
		{Page: 1, Categories: []images.Category{images.General, images.People}},
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
		"bad category":    {Page: 1, Categories: []images.Category{"manga"}},
		"text too long":   {Page: 1, Text: strings.Repeat("a", MaxQueryLength+1)},
	}
	for name, q := range invalid {
		if err := q.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestQueryNormalize(t *testing.T) {
	tests := []struct {
		name        string
		in          Query
		wantText    string
		wantExclude []string
	}{
		{"plain text", Query{Text: "misty mountains"}, "misty mountains", nil},
		{"collapses whitespace", Query{Text: "  misty \t mountains  "}, "misty mountains", nil},
		{"extracts excluded terms", Query{Text: "mountains -snow -anime"}, "mountains", []string{"snow", "anime"}},
		{"keeps inner hyphens", Query{Text: "sci-fi city"}, "sci-fi city", nil},
		{"drops a lone hyphen", Query{Text: "lake - forest"}, "lake forest", nil},
		{"strips repeated hyphens", Query{Text: "lake --snow"}, "lake", []string{"snow"}},
		{"removes duplicates ignoring case", Query{Text: "lake -Snow -snow"}, "lake", []string{"Snow"}},
		// The first spelling wins; existing excludes are read before Text.
		{"merges with existing excludes", Query{Text: "lake -snow", Exclude: []string{"ice", "SNOW"}}, "lake", []string{"ice", "SNOW"}},
		{"exclude only", Query{Text: "-snow"}, "", []string{"snow"}},
		{"keeps non-ASCII terms", Query{Text: "村 -雪"}, "村", []string{"雪"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.Normalize()
			if got.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", got.Text, tt.wantText)
			}
			if !slices.Equal(got.Exclude, tt.wantExclude) {
				t.Errorf("Exclude = %q, want %q", got.Exclude, tt.wantExclude)
			}
			if again := got.Normalize(); again.Text != got.Text || !slices.Equal(again.Exclude, got.Exclude) {
				t.Errorf("Normalize is not idempotent: %+v -> %+v", got, again)
			}
		})
	}
}

func TestQueryNormalizeCategories(t *testing.T) {
	q := Query{Categories: []images.Category{images.People, images.General, images.People}}.Normalize()
	if !slices.Equal(q.Categories, []images.Category{images.General, images.People}) {
		t.Errorf("Categories = %q", q.Categories)
	}
	if q := (Query{Categories: images.Categories}).Normalize(); q.Categories != nil {
		t.Errorf("all categories = %q, want nil (no restriction)", q.Categories)
	}
	if q := (Query{Page: 1, Categories: []images.Category{"manga"}}).Normalize(); q.Validate() == nil {
		t.Error("Normalize hid an unknown category from Validate")
	}
}

func TestQueryValidateExcludedTerms(t *testing.T) {
	tooMany := make([]string, MaxExcludeTerms+1)
	for i := range tooMany {
		tooMany[i] = "t" + strings.Repeat("x", i)
	}
	invalid := map[string][]string{
		"too many":  tooMany,
		"empty":     {""},
		"two words": {"snow storm"},
		"too long":  {strings.Repeat("a", MaxTermLength+1)},
	}
	for name, exclude := range invalid {
		if err := (Query{Page: 1, Exclude: exclude}).Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if err := (Query{Page: 1, Exclude: []string{"snow", "anime"}}).Validate(); err != nil {
		t.Errorf("valid excludes: %v", err)
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
