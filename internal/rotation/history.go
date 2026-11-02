package rotation

import (
	"context"
	"database/sql"
	"fmt"
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
