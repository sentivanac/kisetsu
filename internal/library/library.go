// Package library stores downloaded images and collections in the database.
//
// It only manages database records. Files are written and removed by the
// download and library services, which use the paths stored here.
package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
)

var (
	// ErrNotFound means no record has the requested ID.
	ErrNotFound = errors.New("not found in library")
	// ErrAlreadyInLibrary means the same provider image was added before.
	ErrAlreadyInLibrary = errors.New("image already in library")
	// ErrDuplicateFile means a file with the same content (SHA-256) exists.
	ErrDuplicateFile = errors.New("identical file already in library")
)

// timeLayout matches the timestamps written by SQLite's strftime in the
// migrations.
const timeLayout = "2006-01-02T15:04:05.000Z"

// Entry is one image in the library.
type Entry struct {
	ID int64
	// Image keeps the provider metadata and attribution exactly as found.
	Image images.Image
	// FilePath and ThumbnailPath are relative to the library directory.
	FilePath      string
	ThumbnailPath string
	SHA256        string
	DHash         uint64
	AddedAt       time.Time
}

// Store reads and writes library records.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const entryColumns = `id, provider, provider_id, page_url, download_url, source_url,
	author_name, author_url, license_status, license_name, license_url, license_notice,
	width, height, mime_type, file_size, file_path, thumbnail_path, sha256, dhash, added_at`

// Add stores a new entry and returns its ID. It fails with
// ErrAlreadyInLibrary or ErrDuplicateFile instead of storing a duplicate.
func (s *Store) Add(ctx context.Context, e Entry) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	defer tx.Rollback() // no-op after commit

	var n int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM images WHERE provider = ? AND provider_id = ?`,
		e.Image.Provider, e.Image.ProviderID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	if n > 0 {
		return 0, fmt.Errorf("%w: %s/%s", ErrAlreadyInLibrary, e.Image.Provider, e.Image.ProviderID)
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM images WHERE sha256 = ?`, e.SHA256).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	if n > 0 {
		return 0, fmt.Errorf("%w: %s", ErrDuplicateFile, e.SHA256)
	}

	img := e.Image
	res, err := tx.ExecContext(ctx, `INSERT INTO images (
			provider, provider_id, page_url, download_url, source_url,
			author_name, author_url, license_status, license_name, license_url, license_notice,
			width, height, mime_type, file_size, file_path, thumbnail_path, sha256, dhash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Provider, img.ProviderID, img.PageURL, img.DownloadURL, img.SourceURL,
		img.Author.Name, img.Author.URL, string(img.License.Status), img.License.Name, img.License.URL, img.License.Notice,
		img.Width, img.Height, img.MIMEType, img.FileSize, e.FilePath, e.ThumbnailPath, e.SHA256, int64(e.DHash))
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	return id, nil
}

// Get returns the entry with the given ID.
func (s *Store) Get(ctx context.Context, id int64) (Entry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM images WHERE id = ?`, id)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, fmt.Errorf("%w: image %d", ErrNotFound, id)
	}
	return e, err
}

// FindBySource returns the entry downloaded from a provider image, if any.
func (s *Store) FindBySource(ctx context.Context, provider, providerID string) (Entry, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM images WHERE provider = ? AND provider_id = ?`,
		provider, providerID)
	return found(scanEntry(row))
}

// FindBySHA256 returns the entry whose file has the given SHA-256, if any.
func (s *Store) FindBySHA256(ctx context.Context, sum string) (Entry, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+entryColumns+` FROM images WHERE sha256 = ?`, sum)
	return found(scanEntry(row))
}

// ListOptions select and page library entries.
type ListOptions struct {
	CollectionID int64 // 0 means all images
	Limit        int   // 0 means no limit
	Offset       int
}

// List returns entries, newest first.
func (s *Store) List(ctx context.Context, opts ListOptions) ([]Entry, error) {
	query := `SELECT ` + entryColumns + ` FROM images`
	var args []any
	if opts.CollectionID != 0 {
		query += ` WHERE id IN (SELECT image_id FROM collection_images WHERE collection_id = ?)`
		args = append(args, opts.CollectionID)
	}
	query += ` ORDER BY added_at DESC, id DESC`
	if opts.Limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, opts.Limit, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return out, nil
}

// Stats summarizes the library.
type Stats struct {
	Images int
	Bytes  int64 // total size of the original files
}

// Stats returns the number of images and their total size.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT count(*), coalesce(sum(file_size), 0) FROM images`).Scan(&st.Images, &st.Bytes)
	if err != nil {
		return Stats{}, fmt.Errorf("library stats: %w", err)
	}
	return st, nil
}

// HashRef pairs an entry with its perceptual hash.
type HashRef struct {
	ID    int64
	DHash uint64
}

// DHashes returns the perceptual hash of every entry, for finding visually
// similar images.
func (s *Store) DHashes(ctx context.Context) ([]HashRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, dhash FROM images`)
	if err != nil {
		return nil, fmt.Errorf("list hashes: %w", err)
	}
	defer rows.Close()

	var out []HashRef
	for rows.Next() {
		var ref HashRef
		var h int64
		if err := rows.Scan(&ref.ID, &h); err != nil {
			return nil, fmt.Errorf("list hashes: %w", err)
		}
		ref.DHash = uint64(h)
		out = append(out, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list hashes: %w", err)
	}
	return out, nil
}

// Delete removes the entry and its collection memberships. It does not
// touch the files; the caller removes them.
func (s *Store) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete image %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: image %d", ErrNotFound, id)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(row scanner) (Entry, error) {
	var e Entry
	var status, added string
	var dhash int64
	img := &e.Image
	err := row.Scan(&e.ID, &img.Provider, &img.ProviderID, &img.PageURL, &img.DownloadURL, &img.SourceURL,
		&img.Author.Name, &img.Author.URL, &status, &img.License.Name, &img.License.URL, &img.License.Notice,
		&img.Width, &img.Height, &img.MIMEType, &img.FileSize, &e.FilePath, &e.ThumbnailPath, &e.SHA256, &dhash, &added)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, err
		}
		return Entry{}, fmt.Errorf("read image: %w", err)
	}
	img.License.Status = images.LicenseStatus(status)
	e.DHash = uint64(dhash)
	if e.AddedAt, err = time.Parse(timeLayout, added); err != nil {
		return Entry{}, fmt.Errorf("read image %d: added_at: %w", e.ID, err)
	}
	return e, nil
}

// found turns sql.ErrNoRows into a "not found" result.
func found(e Entry, err error) (Entry, bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, err
	}
	return e, true, nil
}
