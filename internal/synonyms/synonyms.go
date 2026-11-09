// Package synonyms suggests other words for a search: "autumn" also as
// "fall" and "foliage" (ADR 0049). The sources cannot search several words
// as alternatives in one request, so Kisetsu offers the alternatives as
// searches of their own, which the user runs with one click.
//
// The list is small and written by hand: words people use for wallpapers,
// and spelling variants (British and American).
package synonyms

import "strings"

// groups lists words and phrases that mean about the same for a wallpaper
// search. Every member suggests the others. All lowercase.
var groups = [][]string{
	// Seasons and weather
	{"autumn", "fall", "foliage"},
	{"spring", "blossom"},
	{"winter", "snow", "snowy"},
	{"fog", "mist", "haze"},
	{"foggy", "misty", "hazy"},
	{"rain", "rainy"},
	{"storm", "thunderstorm", "lightning"},
	{"clouds", "cloudy sky"},

	// Times of day and sky
	{"sunset", "dusk", "golden hour"},
	{"sunrise", "dawn"},
	{"night sky", "starry sky", "stars"},
	{"milky way", "galaxy"},
	{"space", "cosmos", "universe"},
	{"nebula", "deep space"},
	{"aurora", "northern lights"},
	{"moon", "full moon"},

	// Landscape
	{"landscape", "scenery", "nature"},
	{"mountain", "peak"},
	{"mountains", "peaks", "mountain range"},
	{"forest", "woods", "woodland"},
	{"sea", "ocean"},
	{"coast", "coastline", "shore", "seaside"},
	{"lake", "pond"},
	{"river", "stream"},
	{"waterfall", "cascade"},
	{"desert", "dunes", "sand dunes"},
	{"meadow", "field", "grassland"},
	{"canyon", "gorge", "ravine"},
	{"cave", "cavern"},
	{"flowers", "blossoms", "bloom"},
	{"garden", "park"},

	// Places and things
	{"city", "cityscape", "skyline"},
	{"village", "town"},
	{"castle", "fortress"},
	{"temple", "shrine"},
	{"harbour", "harbor", "port"},
	{"boat", "ship", "sailboat"},
	{"train", "railway", "railroad"},
	{"car", "automobile"},
	{"cat", "kitten"},
	{"dog", "puppy"},
	{"painting", "artwork"},
	{"vintage", "retro"},

	// Spelling variants
	{"colour", "color"},
	{"colourful", "colorful"},
	{"grey", "gray"},
	{"cosy", "cozy"},
}

// MaxSuggestions is the most searches Suggest offers.
const MaxSuggestions = 6

// index maps each member to the other members of its group.
var index = func() map[string][]string {
	m := map[string][]string{}
	for _, g := range groups {
		for _, w := range g {
			for _, other := range g {
				if other != w {
					m[w] = append(m[w], other)
				}
			}
		}
	}
	return m
}()

// longest is the most words a member has.
const longest = 2

// Suggest returns other searches for text, each with one word or phrase
// replaced by a synonym, in the order the words appear: "misty forest"
// gives "foggy forest", "hazy forest", "misty woods", "misty woodland".
// Excluded words ("-snow") are kept and never replaced. The user's other
// words keep their spelling.
func Suggest(text string) []string {
	words := strings.Fields(text)
	var out []string
	seen := map[string]bool{strings.ToLower(strings.Join(words, " ")): true}
	for i := 0; i < len(words) && len(out) < MaxSuggestions; i++ {
		// The longest member starting here wins: "night sky" before "night".
		for n := min(longest, len(words)-i); n >= 1; n-- {
			span := words[i : i+n]
			if strings.HasPrefix(span[0], "-") {
				break
			}
			others, ok := index[strings.ToLower(strings.Join(span, " "))]
			if !ok {
				continue
			}
			for _, other := range others {
				replaced := append(append(append([]string{}, words[:i]...), other), words[i+n:]...)
				s := strings.Join(replaced, " ")
				if key := strings.ToLower(s); !seen[key] && len(out) < MaxSuggestions {
					seen[key] = true
					out = append(out, s)
				}
			}
			i += n - 1
			break
		}
	}
	return out
}
