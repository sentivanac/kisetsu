package library

import (
	"context"
	"fmt"
)

// Hide keeps an image out of rotation for good, without deleting it
// (ADR 0054). Hiding it twice is not an error.
func (s *Store) Hide(ctx context.Context, imageID int64) error {
	if err := s.exists(ctx, "images", imageID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO hidden_images (image_id) VALUES (?) ON CONFLICT DO NOTHING`, imageID); err != nil {
		return fmt.Errorf("hide image %d: %w", imageID, err)
	}
	return nil
}

// Unhide lets an image back into rotation.
func (s *Store) Unhide(ctx context.Context, imageID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM hidden_images WHERE image_id = ?`, imageID); err != nil {
		return fmt.Errorf("show image %d again: %w", imageID, err)
	}
	return nil
}

// IsHidden reports whether an image is kept out of rotation.
func (s *Store) IsHidden(ctx context.Context, imageID int64) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM hidden_images WHERE image_id = ?`, imageID).Scan(&n); err != nil {
		return false, fmt.Errorf("read hidden images: %w", err)
	}
	return n > 0, nil
}

// HiddenIDs returns the images kept out of rotation.
func (s *Store) HiddenIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT image_id FROM hidden_images`)
	if err != nil {
		return nil, fmt.Errorf("read hidden images: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("read hidden images: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// Hidden returns the hidden images, most recently hidden first.
func (s *Store) Hidden(ctx context.Context) ([]Entry, error) {
	// No column name is in both tables, so entryColumns needs no prefix.
	rows, err := s.db.QueryContext(ctx, `SELECT `+entryColumns+`
		FROM images JOIN hidden_images ON hidden_images.image_id = images.id
		ORDER BY hidden_images.hidden_at DESC, hidden_images.image_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list hidden images: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
