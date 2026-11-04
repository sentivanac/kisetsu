package providers

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{"  long \t hair ", "Barefoot", "barefoot", "", "  "})
	if want := []string{"long hair", "Barefoot"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateTags(t *testing.T) {
	if err := ValidateTags([]string{"barefoot", "long hair", "季節"}); err != nil {
		t.Errorf("valid tags: %v", err)
	}
	tooMany := make([]string, MaxHiddenTags+1)
	for i := range tooMany {
		tooMany[i] = "t" + strings.Repeat("x", i%5)
	}
	invalid := map[string][]string{
		"empty":    {""},
		"too long": {strings.Repeat("a", MaxTagLength+1)},
		"quote":    {`say "hi"`},
		"control":  {"a\nb"},
		"too many": tooMany,
	}
	for name, tags := range invalid {
		if ValidateTags(tags) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestQueryNormalizesHiddenTags(t *testing.T) {
	q := Query{Page: 1, HiddenTags: []string{" long  hair", "LONG HAIR"}}.Normalize()
	if !slices.Equal(q.HiddenTags, []string{"long hair"}) {
		t.Errorf("HiddenTags = %q", q.HiddenTags)
	}
	if err := (Query{Page: 1, HiddenTags: []string{`a"b`}}).Validate(); err == nil {
		t.Error("a tag with a quote was accepted")
	}
}
