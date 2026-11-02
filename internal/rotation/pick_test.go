package rotation

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

func seeded() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

// simulate picks n times, updating the state as the history would.
func simulate(candidates []int64, order Order, n int) []int64 {
	rng := seeded()
	st := State{Shown: map[int64]bool{}}
	var shown []int64
	for range n {
		id, cycle, _ := Pick(candidates, st, order, rng)
		if cycle != st.Cycle {
			st.Cycle, st.Shown = cycle, map[int64]bool{}
		}
		st.Shown[id] = true
		st.Current = id
		shown = append(shown, id)
	}
	return shown
}

func TestRandomShowsEveryImageOncePerCycle(t *testing.T) {
	candidates := []int64{1, 2, 3, 4, 5}
	shown := simulate(candidates, Random, 5*20)

	orders := map[string]bool{}
	for c := range 20 {
		cycle := slices.Clone(shown[c*5 : c*5+5])
		orders[fmt.Sprint(cycle)] = true
		slices.Sort(cycle)
		if !slices.Equal(cycle, candidates) {
			t.Fatalf("cycle %d showed %v, want each image once", c, shown[c*5:c*5+5])
		}
	}
	// A new random order each cycle, not the first one repeated.
	if len(orders) < 10 {
		t.Errorf("only %d different orders in 20 cycles", len(orders))
	}
	for i := 1; i < len(shown); i++ {
		if shown[i] == shown[i-1] {
			t.Fatalf("image %d shown twice in a row at %d", shown[i], i)
		}
	}
}

func TestRandomAvoidsTheImageOnScreen(t *testing.T) {
	// The user set image 2 by hand; it was not shown by rotation yet.
	st := State{Cycle: 3, Current: 2, Shown: map[int64]bool{1: true}}
	for range 50 {
		id, cycle, _ := Pick([]int64{1, 2, 3}, st, Random, seeded())
		if id != 3 || cycle != 3 {
			t.Fatalf("picked %d in cycle %d, want 3 in cycle 3", id, cycle)
		}
	}
}

func TestPickEdgeCases(t *testing.T) {
	if _, _, ok := Pick(nil, State{}, Random, seeded()); ok {
		t.Error("picked from no candidates")
	}
	// One image: shown every time, starting new cycles.
	st := State{Cycle: 0, Current: 7, Shown: map[int64]bool{7: true}}
	if id, cycle, ok := Pick([]int64{7}, st, Random, seeded()); !ok || id != 7 || cycle != 1 {
		t.Errorf("single image: %d, cycle %d, %v", id, cycle, ok)
	}
	// Images shown in the cycle but no longer candidates are ignored.
	st = State{Current: 9, Shown: map[int64]bool{9: true, 1: true}}
	if id, _, _ := Pick([]int64{1, 2}, st, Random, seeded()); id != 2 {
		t.Errorf("picked %d, want 2", id)
	}
}

func TestSequentialWrapsAround(t *testing.T) {
	if got := simulate([]int64{10, 20, 30}, Sequential, 7); !slices.Equal(got, []int64{10, 20, 30, 10, 20, 30, 10}) {
		t.Errorf("sequence = %v", got)
	}
	// The image on screen is not in the source (e.g. another collection).
	if id, _, _ := Pick([]int64{10, 20}, State{Current: 99}, Sequential, seeded()); id != 10 {
		t.Errorf("unknown current: picked %d, want the oldest", id)
	}
}
