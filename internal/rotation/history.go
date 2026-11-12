package rotation

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Reason says why a wallpaper was set.
type Reason string

const (
	ByRotation Reason = "rotation"
	ByUser     Reason = "manual"
)

// History stores every wallpaper Kisetsu set (table wallpaper_history).
type History struct {
	db *sql.DB
}

// NewHistory returns a History backed by db.
func NewHistory(db *sql.DB) *History {
	return &History{db: db}
}

// State returns the current cycle, the last image and the images shown in
// the current cycle. Images deleted from the library are gone from the
// history as well (ON DELETE CASCADE).
func (h *History) State(ctx context.Context) (State, error) {
	st := State{Shown: map[int64]bool{}}
	err := h.db.QueryRowContext(ctx,
		`SELECT cycle, image_id FROM wallpaper_history ORDER BY id DESC LIMIT 1`).Scan(&st.Cycle, &st.Current)
	if err == sql.ErrNoRows {
		return st, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read wallpaper history: %w", err)
	}

	rows, err := h.db.QueryContext(ctx,
		`SELECT DISTINCT image_id FROM wallpaper_history WHERE cycle = ? AND reason = 'rotation'`, st.Cycle)
	if err != nil {
		return State{}, fmt.Errorf("read wallpaper history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return State{}, fmt.Errorf("read wallpaper history: %w", err)
		}
		st.Shown[id] = true
	}
	return st, rows.Err()
}

// Record notes that an image was set as wallpaper.
func (h *History) Record(ctx context.Context, imageID int64, cycle int, reason Reason) error {
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO wallpaper_history (image_id, cycle, reason) VALUES (?, ?, ?)`, imageID, cycle, string(reason))
	if err != nil {
		return fmt.Errorf("record wallpaper: %w", err)
	}
	return nil
}

// LastChange is the most recent wallpaper Kisetsu set.
type LastChange struct {
	ImageID int64
	Reason  Reason
	At      time.Time
}

// Last returns the most recent change, and false if there is none.
func (h *History) Last(ctx context.Context) (LastChange, bool, error) {
	var lc LastChange
	var reason, at string
	err := h.db.QueryRowContext(ctx,
		`SELECT image_id, reason, shown_at FROM wallpaper_history ORDER BY id DESC LIMIT 1`).Scan(&lc.ImageID, &reason, &at)
	if err == sql.ErrNoRows {
		return LastChange{}, false, nil
	}
	if err != nil {
		return LastChange{}, false, fmt.Errorf("read wallpaper history: %w", err)
	}
	lc.Reason = Reason(reason)
	if lc.At, err = time.Parse("2006-01-02T15:04:05.000Z", at); err != nil {
		return LastChange{}, false, fmt.Errorf("read wallpaper history: %w", err)
	}
	return lc, true, nil
}

// Entry is one wallpaper change in the history.
type Entry struct {
	ID      int64 // the history row
	ImageID int64
}

// lastEntry returns the most recent history row, and false if there is
// none.
func (h *History) lastEntry(ctx context.Context) (Entry, bool, error) {
	var e Entry
	err := h.db.QueryRowContext(ctx,
		`SELECT id, image_id FROM wallpaper_history ORDER BY id DESC LIMIT 1`).Scan(&e.ID, &e.ImageID)
	if err == sql.ErrNoRows {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, fmt.Errorf("read wallpaper history: %w", err)
	}
	return e, true, nil
}

// before returns up to limit history rows older than row id, newest first.
func (h *History) before(ctx context.Context, id int64, limit int) ([]Entry, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, image_id FROM wallpaper_history WHERE id < ? ORDER BY id DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("read wallpaper history: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ImageID); err != nil {
			return nil, fmt.Errorf("read wallpaper history: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// record notes a change and returns its history row.
func (h *History) record(ctx context.Context, imageID int64, cycle int, reason Reason) (int64, error) {
	res, err := h.db.ExecContext(ctx,
		`INSERT INTO wallpaper_history (image_id, cycle, reason) VALUES (?, ?, ?)`, imageID, cycle, string(reason))
	if err != nil {
		return 0, fmt.Errorf("record wallpaper: %w", err)
	}
	return res.LastInsertId()
}
