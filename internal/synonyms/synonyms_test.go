package synonyms

import (
	"slices"
	"strings"
	"testing"
)

func TestSuggest(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"autumn", []string{"fall", "foliage"}},
		{"Misty forest", []string{"foggy forest", "hazy forest", "Misty woods", "Misty woodland"}},
		// A phrase wins over its first word.
		{"night sky over the sea", []string{"starry sky over the sea", "stars over the sea", "night sky over the ocean"}},
		{"northern lights", []string{"aurora"}},
		// Excluded words stay as they are.
		{"mountains -snow", []string{"peaks -snow", "mountain range -snow"}},
		{"winter -autumn", []string{"snow -autumn", "snowy -autumn"}},
		{"grey sky", []string{"gray sky"}},
		{"", nil},
		{"abstract", nil},
	}
	for _, tt := range tests {
		if got := Suggest(tt.text); !slices.Equal(got, tt.want) {
			t.Errorf("Suggest(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestSuggestIsLimited(t *testing.T) {
	got := Suggest("misty autumn forest by the sea at sunset")
	if len(got) != MaxSuggestions {
		t.Errorf("%d suggestions, want %d: %q", len(got), MaxSuggestions, got)
	}
}

func TestGroups(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range groups {
		if len(g) < 2 {
			t.Errorf("group %q has no synonym", g)
		}
		for _, w := range g {
			if w != strings.ToLower(strings.TrimSpace(w)) || len(strings.Fields(w)) > longest {
				t.Errorf("%q must be lowercase, trimmed and at most %d words", w, longest)
			}
			// A word in two groups would mix their meanings.
			if seen[w] {
				t.Errorf("%q is in more than one group", w)
			}
			seen[w] = true
		}
	}
}
