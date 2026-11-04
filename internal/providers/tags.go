package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxHiddenTags limits the hidden tags sent with every search.
	MaxHiddenTags = 100
	// MaxTagLength limits one tag, in characters.
	MaxTagLength = 64
)

// Tagger is implemented by providers whose images carry tags, such as
// Wallhaven. Tags returns the tag names of one image (by its provider ID).
// It costs an API call, so it is only used when the user looks at an
// image.
type Tagger interface {
	Tags(ctx context.Context, imageID string) ([]string, error)
}

// NormalizeTag trims a tag and collapses inner whitespace. Tags are
// compared case-insensitively.
func NormalizeTag(tag string) string {
	return strings.Join(strings.Fields(tag), " ")
}

// NormalizeTags normalizes each tag and removes empty ones and repeats
// (case-insensitively; the first spelling wins).
func NormalizeTags(tags []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range tags {
		t = NormalizeTag(t)
		key := strings.ToLower(t)
		if t == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// ValidateTags checks a list of hidden tags: at most MaxHiddenTags, each
// at most MaxTagLength characters, without quotes or control characters
// (a tag is sent inside quotes when it has several words).
func ValidateTags(tags []string) error {
	var errs []error
	if len(tags) > MaxHiddenTags {
		errs = append(errs, fmt.Errorf("more than %d hidden tags", MaxHiddenTags))
	}
	for _, t := range tags {
		if t == "" || utf8.RuneCountInString(t) > MaxTagLength ||
			strings.ContainsAny(t, `"`) || strings.ContainsFunc(t, unicode.IsControl) {
			errs = append(errs, fmt.Errorf("hidden tag %q must be 1 to %d characters, without quotes", t, MaxTagLength))
		}
	}
	return errors.Join(errs...)
}
