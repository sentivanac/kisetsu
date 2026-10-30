package library

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrCollectionExists means another collection has the same name
	// (names are compared case-insensitively).
	ErrCollectionExists = errors.New("a collection with this name already exists")
	// ErrInvalidName means a collection name is empty or too long.
	ErrInvalidName = errors.New("invalid collection name")
)

// MaxCollectionName limits collection names, in characters.
const MaxCollectionName = 100

// Collection is a named group of library images.
type Collection struct {
	ID        int64
	Name      string
	Images    int // number of images in the collection
	CreatedAt time.Time
}

func cleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || utf8.RuneCountInString(name) > MaxCollectionName {
		return "", fmt.Errorf("%w: must be 1 to %d characters", ErrInvalidName, MaxCollectionName)
	}
	return name, nil
}

// nameTaken reports whether a collection other than exceptID has name.
func (s *Store) nameTaken(ctx context.Context, name string, exceptID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM collections WHERE name = ? AND id != ?`, name, exceptID).Scan(&n)
	return n > 0, err
}

// CreateCollection creates a collection and returns it.
func (s *Store) CreateCollection(ctx context.Context, name string) (Collection, error) {
	name, err := cleanName(name)
	if err != nil {
		return Collection{}, err
	}
	if taken, err := s.nameTaken(ctx, name, 0); err != nil {
		return Collection{}, fmt.Errorf("create collection: %w", err)
	} else if taken {
		return Collection{}, fmt.Errorf("%w: %q", ErrCollectionExists, name)
	}

	var c Collection
	var created string
	err = s.db.QueryRowContext(ctx, `INSERT INTO collections (name) VALUES (?) RETURNING id, name, created_at`, name).
		Scan(&c.ID, &c.Name, &created)
	if err != nil {
		return Collection{}, fmt.Errorf("create collection: %w", err)
	}
	if c.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return Collection{}, fmt.Errorf("create collection: %w", err)
	}
	return c, nil
}

// RenameCollection changes the name of a collection.
func (s *Store) RenameCollection(ctx context.Context, id int64, name string) error {
	name, err := cleanName(name)
	if err != nil {
		return err
	}
	if taken, err := s.nameTaken(ctx, name, id); err != nil {
		return fmt.Errorf("rename collection: %w", err)
	} else if taken {
		return fmt.Errorf("%w: %q", ErrCollectionExists, name)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE collections SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("rename collection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: collection %d", ErrNotFound, id)
	}
	return nil
}

// DeleteCollection removes a collection. Its images stay in the library.
func (s *Store) DeleteCollection(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: collection %d", ErrNotFound, id)
	}
	return nil
}

// Collections lists all collections by name, with their image counts.
func (s *Store) Collections(ctx context.Context) ([]Collection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.created_at, count(ci.image_id)
		FROM collections c
		LEFT JOIN collection_images ci ON ci.collection_id = c.id
		GROUP BY c.id
		ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()

	var out []Collection
	for rows.Next() {
		var c Collection
		var created string
		if err := rows.Scan(&c.ID, &c.Name, &created, &c.Images); err != nil {
			return nil, fmt.Errorf("list collections: %w", err)
		}
		if c.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
			return nil, fmt.Errorf("list collections: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	return out, nil
}

// AddToCollection puts an image in a collection. Adding it twice is not an
// error.
func (s *Store) AddToCollection(ctx context.Context, collectionID, imageID int64) error {
	if err := s.exists(ctx, "collections", collectionID); err != nil {
		return err
	}
	if err := s.exists(ctx, "images", imageID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO collection_images (collection_id, image_id) VALUES (?, ?) ON CONFLICT DO NOTHING`,
		collectionID, imageID)
	if err != nil {
		return fmt.Errorf("add to collection: %w", err)
	}
	return nil
}

// RemoveFromCollection takes an image out of a collection. The image stays
// in the library. Removing an image that is not in the collection is not
// an error.
func (s *Store) RemoveFromCollection(ctx context.Context, collectionID, imageID int64) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM collection_images WHERE collection_id = ? AND image_id = ?`, collectionID, imageID)
	if err != nil {
		return fmt.Errorf("remove from collection: %w", err)
	}
	return nil
}

// ImageCollections returns the IDs of the collections an image belongs to.
func (s *Store) ImageCollections(ctx context.Context, imageID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT collection_id FROM collection_images WHERE image_id = ? ORDER BY collection_id`, imageID)
	if err != nil {
		return nil, fmt.Errorf("image collections: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("image collections: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// exists checks that a row with id exists in table (a fixed, trusted name).
func (s *Store) exists(ctx context.Context, table string, id int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE id = ?`, id).Scan(&n); err != nil {
		return fmt.Errorf("check %s %d: %w", table, id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s %d", ErrNotFound, strings.TrimSuffix(table, "s"), id)
	}
	return nil
}
