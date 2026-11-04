package images

import (
	"fmt"
	"slices"
)

// Category is a kind of image content, as image sources such as
// Wallhaven classify it. Sources without categories ignore it.
type Category string

const (
	General Category = "general" // landscapes, nature, architecture, art
	Anime   Category = "anime"   // anime and manga
	People  Category = "people"  // photographs and drawings of people
)

// Categories lists every category, in the order sources use them.
var Categories = []Category{General, Anime, People}

// Valid reports whether c is one of the defined categories.
func (c Category) Valid() bool {
	return slices.Contains(Categories, c)
}

// NormalizeCategories returns the chosen categories in the order of
// Categories, without repeats. Choosing none or all means no restriction
// and returns nil. Unknown categories are an error.
func NormalizeCategories(chosen []Category) ([]Category, error) {
	set := map[Category]bool{}
	for _, c := range chosen {
		if !c.Valid() {
			return nil, fmt.Errorf("unknown category %q", c)
		}
		set[c] = true
	}
	if len(set) == 0 || len(set) == len(Categories) {
		return nil, nil
	}
	var out []Category
	for _, c := range Categories {
		if set[c] {
			out = append(out, c)
		}
	}
	return out, nil
}
