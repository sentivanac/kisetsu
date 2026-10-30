package commons

import (
	"strings"
	"unicode"
)

// Wikimedia Commons has no safe-for-work flag. Kisetsu reduces the risk in
// three ways (see ADR 0011):
//
//  1. searches are limited to the curated Quality and Featured collections;
//  2. serverExcludes are sent as "-term" with every search;
//  3. results whose title, description or categories contain a riskyWord
//     are dropped locally.
//
// This is a best-effort filter, not a guarantee, which is why the source is
// off by default and the UI says so.

// serverExcludes is kept short: Commons limits a search to 300 characters.
var serverExcludes = []string{"nude", "naked", "nudity", "erotic"}

var riskyWords = map[string]bool{
	"nude": true, "nudes": true, "naked": true, "nudity": true,
	"nudism": true, "nudist": true, "naturism": true, "naturist": true,
	"erotic": true, "erotica": true, "eroticism": true,
	"sex": true, "sexual": true, "sexuality": true,
	"porn": true, "porno": true, "pornography": true, "pornographic": true,
	"topless": true, "bottomless": true, "undressed": true, "lingerie": true,
	"genitals": true, "genitalia": true, "penis": true, "vagina": true, "vulva": true,
	"breasts": true, "nipples": true, "buttocks": true,
}

// looksUnsafe reports whether any of the texts contains a risky word.
// Matching is per word, so "Essex" or "Sussex" do not match "sex".
func looksUnsafe(texts ...string) bool {
	for _, text := range texts {
		words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		for _, w := range words {
			if riskyWords[w] {
				return true
			}
		}
	}
	return false
}
