package rotation

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoPrevious means the history has no earlier wallpaper to go back to.
var ErrNoPrevious = errors.New("no earlier wallpaper")

// backKey stores how far back Previous has gone (ADR 0054).
const backKey = "rotation_back"

// back is where a run of Previous stands: the history row Previous wrote
// last, and the older row whose image it showed. It applies only while
// that row is still the newest: any other change ends the run.
type back struct {
	Record   int64 `json:"record"`
	Position int64 `json:"position"`
}

// historyScan is how many older rows Previous looks through at most.
const historyScan = 200

// Previous shows the wallpaper before the current one. Called again, it
// goes further back; "Next" afterwards shows a new image, as always.
// Images that are gone, hidden from rotation or whose files are missing
// are skipped. The change counts as one made by hand: rotation does not
// count it as shown in its cycle, and waits a full interval after it.
func (r *Rotator) Previous(ctx context.Context) (Result, error) {
	last, ok, err := r.History.lastEntry(ctx)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, ErrNoPrevious
	}
	from := last.ID
	var b back
	if found, err := r.Settings.Load(ctx, backKey, &b); err == nil && found && b.Record == last.ID {
		from = b.Position
	}

	older, err := r.History.before(ctx, from, historyScan)
	if err != nil {
		return Result{}, err
	}
	for _, e := range older {
		if e.ImageID == last.ImageID {
			continue // the same picture again is no step back
		}
		if hidden, err := r.Library.IsHidden(ctx, e.ImageID); err != nil || hidden {
			continue
		}
		img, path, err := r.path(ctx, e.ImageID)
		if err != nil {
			continue // deleted, or its file is gone
		}
		method, err := r.Screen.Show(ctx, img, path)
		if err != nil {
			return Result{}, err
		}
		st, err := r.History.State(ctx)
		if err != nil {
			return Result{}, err
		}
		record, err := r.History.record(ctx, e.ImageID, st.Cycle, ByUser)
		if err != nil {
			return Result{}, err
		}
		if err := r.Settings.Save(ctx, backKey, back{Record: record, Position: e.ID}); err != nil {
			// The wallpaper changed; only a further Previous starts over.
			r.Log.Warn("position in the wallpaper history not saved", "err", err)
		}
		r.Log.Info("previous wallpaper", "entry", e.ImageID, "method", method)
		return Result{ImageID: e.ImageID, Method: method}, nil
	}
	return Result{}, fmt.Errorf("%w in the last %d changes", ErrNoPrevious, historyScan)
}
