// Package settings stores user preferences in the database.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalidValue means a stored setting could not be decoded, for example
// because an older or newer Kisetsu saved it in another shape.
var ErrInvalidValue = errors.New("invalid stored setting")

// Store reads and writes settings as JSON values in the settings table.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Load decodes the setting key into dst. It reports false, leaving dst
// unchanged, if the setting has never been saved.
func (s *Store) Load(ctx context.Context, key string, dst any) (bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load setting %q: %w", key, err)
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return false, fmt.Errorf("decode setting %q: %w: %w", key, ErrInvalidValue, err)
	}
	return true, nil
}

// Save stores v as the setting key, replacing any previous value.
func (s *Store) Save(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode setting %q: %w", key, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET
			value = excluded.value,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		key, string(raw))
	if err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}
