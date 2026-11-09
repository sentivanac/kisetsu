package providers

import "testing"

func TestLooksUnsafe(t *testing.T) {
	unsafe := []string{"Category:Nude sculptures", "Topless woman", "NAKED-eye astronomy? no: naked", "erotic art"}
	for _, s := range unsafe {
		if !LooksUnsafe(s) {
			t.Errorf("LooksUnsafe(%q) = false", s)
		}
	}
	safe := []string{"Essex coastline", "Sussex downs", "Middlesex", "Mountains of Nepal", ""}
	for _, s := range safe {
		if LooksUnsafe(s) {
			t.Errorf("LooksUnsafe(%q) = true", s)
		}
	}
}
