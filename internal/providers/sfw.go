package providers

import (
	"strings"
	"unicode"
)

// Some sources have no safe-for-work rating (Wikimedia Commons, museum
// collections through Openverse). Their providers drop results whose
// texts contain a risky word: a best-effort filter, which is why such
// sources are off by default and the UI says so (ADR 0011, ADR 0048).

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

// LooksUnsafe reports whether any of the texts contains a risky word.
// Matching is per word, so "Essex" or "Sussex" do not match "sex".
func LooksUnsafe(texts ...string) bool {
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
