package images

import (
	"slices"
	"testing"
)

func TestNormalizeCategories(t *testing.T) {
	tests := []struct {
		name string
		in   []Category
		want []Category
	}{
		{"none means all", nil, nil},
		{"all means none", []Category{People, General, Anime}, nil},
		{"standard order", []Category{People, General}, []Category{General, People}},
		{"repeats removed", []Category{Anime, Anime}, []Category{Anime}},
	}
	for _, tt := range tests {
		got, err := NormalizeCategories(tt.in)
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
	if _, err := NormalizeCategories([]Category{"manga"}); err == nil {
		t.Error("an unknown category was accepted")
	}
}
