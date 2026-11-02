package rotation

import (
	"math/rand/v2"
	"slices"
)

// State is what the history says about the current rotation.
type State struct {
	Cycle   int            // current cycle number
	Current int64          // image shown last; 0 if none
	Shown   map[int64]bool // images shown in the current cycle
}

// Pick chooses the next image among candidates (oldest first) and returns
// it with the cycle it belongs to.
//
// Random order shows every candidate once per cycle: it picks among the
// candidates not yet shown in the current cycle. When all have been shown,
// a new cycle starts and only the image on screen is excluded, so the same
// image never appears twice in a row. (Picking "the least recently shown"
// instead would repeat the first cycle's order forever.)
//
// Sequential order takes the candidate after the current one, wrapping
// around; it starts with the oldest when the current image is not a
// candidate.
//
// Pick returns ok=false when there are no candidates.
func Pick(candidates []int64, st State, order Order, rng *rand.Rand) (id int64, cycle int, ok bool) {
	if len(candidates) == 0 {
		return 0, st.Cycle, false
	}

	if order == Sequential {
		next := 0
		if i := slices.Index(candidates, st.Current); i >= 0 {
			next = (i + 1) % len(candidates)
		}
		return candidates[next], st.Cycle, true
	}

	// Never "change" to the image already on screen (for example one the
	// user set by hand) while any other candidate exists: if it is the only
	// image left in the cycle, a new cycle starts instead.
	notCurrent := func(c int64) bool { return c != st.Current }
	cycle = st.Cycle
	pool := filter(unshown(candidates, st.Shown), notCurrent)
	if len(pool) == 0 {
		cycle++
		pool = filter(candidates, notCurrent)
	}
	if len(pool) == 0 { // the image on screen is the only candidate
		pool = candidates
	}
	return pool[rng.IntN(len(pool))], cycle, true
}

func filter(ids []int64, keep func(int64) bool) []int64 {
	var out []int64
	for _, id := range ids {
		if keep(id) {
			out = append(out, id)
		}
	}
	return out
}

func unshown(candidates []int64, shown map[int64]bool) []int64 {
	var pool []int64
	for _, c := range candidates {
		if !shown[c] {
			pool = append(pool, c)
		}
	}
	return pool
}
