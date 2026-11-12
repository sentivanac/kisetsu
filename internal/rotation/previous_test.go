package rotation

import (
	"context"
	"errors"
	"testing"
)

// shownSequence sets ids one after the other by hand, as history.
func shownSequence(t *testing.T, e *env, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if err := e.r.RecordManual(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
}

func current(t *testing.T, e *env) int64 {
	t.Helper()
	last, _, err := e.r.History.Last(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return last.ImageID
}

func TestPreviousWalksBack(t *testing.T) {
	e, ids := newEnv(t, 4)
	ctx := context.Background()
	shownSequence(t, e, ids[0], ids[1], ids[2], ids[3])

	for _, want := range []int64{ids[2], ids[1], ids[0]} {
		res, err := e.r.Previous(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if res.ImageID != want || current(t, e) != want {
			t.Errorf("Previous showed %d (history says %d), want %d", res.ImageID, current(t, e), want)
		}
	}
	// Nothing older.
	if _, err := e.r.Previous(ctx); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("at the start of the history: %v", err)
	}
}

func TestAnotherChangeEndsTheWayBack(t *testing.T) {
	e, ids := newEnv(t, 4)
	ctx := context.Background()
	shownSequence(t, e, ids[0], ids[1], ids[2])

	e.r.Previous(ctx) // ids[1]
	shownSequence(t, e, ids[3])
	// Back from ids[3]: the image before it, ids[1], not further back.
	res, err := e.r.Previous(ctx)
	if err != nil || res.ImageID != ids[1] {
		t.Errorf("Previous after another change: %d, %v; want %d", res.ImageID, err, ids[1])
	}
}

func TestPreviousSkipsWhatCannotBeShown(t *testing.T) {
	e, ids := newEnv(t, 4)
	ctx := context.Background()
	shownSequence(t, e, ids[0], ids[1], ids[2], ids[2], ids[3])

	e.files.missing["images/wallhaven/2.jpg"] = true // ids[2]'s file is gone
	e.r.Hidden = func(_ context.Context, id int64) bool { return id == ids[1] }
	res, err := e.r.Previous(ctx)
	if err != nil || res.ImageID != ids[0] {
		t.Errorf("Previous: %d, %v; want %d past a missing and a hidden image", res.ImageID, err, ids[0])
	}
}

func TestPreviousDoesNotCountAsShownByRotation(t *testing.T) {
	e, ids := newEnv(t, 2)
	ctx := context.Background()
	shownSequence(t, e, ids[0], ids[1])
	e.r.Previous(ctx)
	st, err := e.r.History.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Shown) != 0 {
		t.Errorf("shown in the cycle: %v, want none", st.Shown)
	}
}

func TestPreviousWithoutHistory(t *testing.T) {
	e, _ := newEnv(t, 2)
	if _, err := e.r.Previous(context.Background()); !errors.Is(err, ErrNoPrevious) {
		t.Errorf("no history: %v", err)
	}
}
