package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sentivanac/kisetsu/internal/providers"
)

const hiddenTagsKey = "search.hiddenTags"

// ErrInvalidTag means a tag cannot be hidden: too long, with quotes, or
// one too many.
var ErrInvalidTag = errors.New("invalid hidden tag")

// HiddenTags returns the tags the user hid, in the order they were added.
// An unreadable or invalid stored value yields none: a bad preference must
// never break searching. Only database failures are returned as errors.
func (s *Store) HiddenTags(ctx context.Context) ([]string, error) {
	var tags []string
	found, err := s.Load(ctx, hiddenTagsKey, &tags)
	switch {
	case errors.Is(err, ErrInvalidValue):
		return nil, nil
	case err != nil:
		return nil, err
	case !found:
		return nil, nil
	}
	tags = providers.NormalizeTags(tags)
	if providers.ValidateTags(tags) != nil {
		return nil, nil
	}
	return tags, nil
}

// SetTagHidden adds tag to the hidden tags or removes it (compared
// case-insensitively), and returns the new list.
func (s *Store) SetTagHidden(ctx context.Context, tag string, hidden bool) ([]string, error) {
	tag = providers.NormalizeTag(tag)
	if err := providers.ValidateTags([]string{tag}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTag, err)
	}
	current, err := s.HiddenTags(ctx)
	if err != nil {
		return nil, err
	}
	var next []string
	for _, t := range current {
		if !strings.EqualFold(t, tag) {
			next = append(next, t)
		}
	}
	if hidden {
		if len(next) < len(current) {
			return current, nil // already hidden; the list stays as it is
		}
		next = append(next, tag)
	}
	next = providers.NormalizeTags(next)
	if err := providers.ValidateTags(next); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTag, err)
	}
	if err := s.Save(ctx, hiddenTagsKey, next); err != nil {
		return nil, err
	}
	return next, nil
}
