// Package searches stores searches the user saved by name.
package searches

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/providers"
)

var (
	// ErrNotFound means no saved search has the requested ID.
	ErrNotFound = errors.New("saved search not found")
	// ErrNameTaken means another saved search has the same name
	// (compared case-insensitively).
	ErrNameTaken = errors.New("a saved search with this name already exists")
	// ErrInvalidName means a name is empty or too long.
	ErrInvalidName = errors.New("invalid saved search name")
	// ErrInvalidQuery means the search could not be run.
	ErrInvalidQuery = errors.New("invalid saved search")
)

const (
	// MaxName limits names, in characters.
	MaxName = 100
	// maxProviders caps the sources of one search.
	maxProviders = 16

	timeLayout = "2006-01-02T15:04:05.000Z"
)

// Query is what a saved search remembers.
type Query struct {
	Text        string             `json:"text"` // may contain "-word" exclusions
	ProviderIDs []string           `json:"providerIds"`
	MinWidth    int                `json:"minWidth"`
	MinHeight   int                `json:"minHeight"`
	Orientation images.Orientation `json:"orientation"` // empty means any
}

// Validate checks that q can be run unattended: at least one source, no
// repeated sources, and a valid provider query.
func (q Query) Validate() error {
	var errs []error
	if len(q.ProviderIDs) == 0 {
		errs = append(errs, errors.New("no sources"))
	}
	if len(q.ProviderIDs) > maxProviders {
		errs = append(errs, fmt.Errorf("more than %d sources", maxProviders))
	}
	seen := map[string]bool{}
	for _, id := range q.ProviderIDs {
		if id == "" || seen[id] {
			errs = append(errs, fmt.Errorf("source %q is empty or repeated", id))
		}
		seen[id] = true
	}
	if err := q.Provider(1).Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidQuery, err)
	}
	return nil
}

// Provider returns the provider query for one page of this search.
func (q Query) Provider(page int) providers.Query {
	return providers.Query{
		Text: q.Text, Page: page, MinWidth: q.MinWidth, MinHeight: q.MinHeight, Orientation: q.Orientation,
	}.Normalize()
}

// Saved is a saved search.
type Saved struct {
	ID        int64
	Name      string
	Query     Query
	CreatedAt time.Time
}

// Store reads and writes saved searches.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func cleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || utf8.RuneCountInString(name) > MaxName {
		return "", fmt.Errorf("%w: must be 1 to %d characters", ErrInvalidName, MaxName)
	}
	return name, nil
}

func (s *Store) nameTaken(ctx context.Context, name string, exceptID int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM saved_searches WHERE name = ? AND id != ?`, name, exceptID).Scan(&n); err != nil {
		return fmt.Errorf("check saved search name: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%w: %q", ErrNameTaken, name)
	}
	return nil
}

// Create saves a search under a name.
func (s *Store) Create(ctx context.Context, name string, q Query) (Saved, error) {
	name, err := cleanName(name)
	if err != nil {
		return Saved{}, err
	}
	if err := q.Validate(); err != nil {
		return Saved{}, err
	}
	if err := s.nameTaken(ctx, name, 0); err != nil {
		return Saved{}, err
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return Saved{}, err
	}
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO saved_searches (name, query) VALUES (?, ?) RETURNING id, name, query, created_at`, name, string(raw))
	return scan(row)
}

// Rename changes a saved search's name.
func (s *Store) Rename(ctx context.Context, id int64, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	if err := s.nameTaken(ctx, name, id); err != nil {
		return err
	}
	return s.exec(ctx, `UPDATE saved_searches SET name = ? WHERE id = ?`, name, id)
}

// Delete removes a saved search.
func (s *Store) Delete(ctx context.Context, id int64) error {
	return s.exec(ctx, `DELETE FROM saved_searches WHERE id = ?`, id)
}

// Get returns one saved search.
func (s *Store) Get(ctx context.Context, id int64) (Saved, error) {
	return scan(s.db.QueryRowContext(ctx, `SELECT id, name, query, created_at FROM saved_searches WHERE id = ?`, id))
}

// FindByName returns the saved search with this name (case-insensitive).
func (s *Store) FindByName(ctx context.Context, name string) (Saved, error) {
	clean, err := cleanName(name)
	if err != nil {
		return Saved{}, err
	}
	return scan(s.db.QueryRowContext(ctx,
		`SELECT id, name, query, created_at FROM saved_searches WHERE name = ?`, clean))
}

// List returns all saved searches by name. Searches whose stored query can
// no longer be read are left out rather than breaking the list.
func (s *Store) List(ctx context.Context) ([]Saved, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, query, created_at FROM saved_searches ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list saved searches: %w", err)
	}
	defer rows.Close()
	var out []Saved
	for rows.Next() {
		saved, err := scan(rows)
		if errors.Is(err, ErrInvalidQuery) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, saved)
	}
	return out, rows.Err()
}

func (s *Store) exec(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("saved search: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (Saved, error) {
	var saved Saved
	var raw, created string
	err := row.Scan(&saved.ID, &saved.Name, &raw, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Saved{}, ErrNotFound
	}
	if err != nil {
		return Saved{}, fmt.Errorf("read saved search: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &saved.Query); err != nil {
		return Saved{}, fmt.Errorf("%w: %w", ErrInvalidQuery, err)
	}
	if saved.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return Saved{}, fmt.Errorf("read saved search: %w", err)
	}
	return saved, nil
}
