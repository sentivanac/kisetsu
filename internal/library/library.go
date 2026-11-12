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
	"slices"
	"strings"
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

// Origin says how an image came into the database.
type Origin string

const (
	// FromUser images are the user's library: downloaded or kept by hand.
	FromUser Origin = "library"
	// Fetched images were downloaded automatically from a saved search and
	// live in the cache until kept (ADR 0028).
	Fetched Origin = "fetched"
	// FromFolder images are files in a local folder the user added, used
	// where they are (ADR 0053).
	FromFolder Origin = "folder"
)

// Entry is one image in the library.
type Entry struct {
	ID int64
	// Image keeps the provider metadata and attribution exactly as found.
	Image images.Image
	// FilePath and ThumbnailPath are relative paths understood by
	// libfiles ("images/…", "thumbs/…", "fetched/…" for fetched images,
	// "folders/<folder id>/…" for a folder's files).
	FilePath      string
	ThumbnailPath string
	// SHA256 is the hash of the file as downloaded; fetched images may be
	// stored smaller than that. Folder images have none.
	SHA256  string
	DHash   uint64
	AddedAt time.Time
	// Origin is FromUser unless the image was fetched; empty means FromUser.
	Origin        Origin
	SavedSearchID int64 // the search a fetched image came from; 0 if none
	// FolderID is the folder of a folder image; 0 for others.
	FolderID int64
	// FileModTime is a folder file's modification time (Unix nanoseconds)
	// when it was read, so syncing notices a change.
	FileModTime int64
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
	author_name, author_url, author_credit, license_status, license_name, license_url, license_notice,
	width, height, mime_type, file_size, file_path, thumbnail_path, coalesce(sha256, ''), dhash, added_at,
	origin, coalesce(saved_search_id, 0), coalesce(folder_id, 0), coalesce(file_mod_time, 0)`

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
	if e.SHA256 != "" {
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM images WHERE sha256 = ?`, e.SHA256).Scan(&n)
		if err != nil {
			return 0, fmt.Errorf("add image: %w", err)
		}
		if n > 0 {
			return 0, fmt.Errorf("%w: %s", ErrDuplicateFile, e.SHA256)
		}
	}

	origin := e.Origin
	if origin == "" {
		origin = FromUser
	}
	e.Origin = origin
	id, err := insert(ctx, tx, e)
	if err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("add image: %w", err)
	}
	return id, nil
}

// execer is a *sql.DB or a *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// insert writes e as a new row and returns its ID. Empty SHA-256, saved
// search, folder and modification time are stored as NULL.
func insert(ctx context.Context, x execer, e Entry) (int64, error) {
	img := e.Image
	res, err := x.ExecContext(ctx, `INSERT INTO images (
			provider, provider_id, page_url, download_url, source_url,
			author_name, author_url, author_credit, license_status, license_name, license_url, license_notice,
			width, height, mime_type, file_size, file_path, thumbnail_path, sha256, dhash, origin, saved_search_id,
			folder_id, file_mod_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		img.Provider, img.ProviderID, img.PageURL, img.DownloadURL, img.SourceURL,
		img.Author.Name, img.Author.URL, img.Author.Credit, string(img.License.Status), img.License.Name, img.License.URL, img.License.Notice,
		img.Width, img.Height, img.MIMEType, img.FileSize, e.FilePath, e.ThumbnailPath, nullString(e.SHA256), int64(e.DHash),
		string(e.Origin), nullInt(e.SavedSearchID), nullInt(e.FolderID), nullInt(e.FileModTime))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
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

// SourceRef names a provider image.
type SourceRef struct {
	Provider   string
	ProviderID string
}

// maxRefsPerQuery keeps IDsBySource below SQLite's limit of bound
// parameters (two per reference).
const maxRefsPerQuery = 400

// IDsBySource returns the library entry IDs of the given provider images
// that are in the library. Images not in the library are left out.
func (s *Store) IDsBySource(ctx context.Context, refs []SourceRef) (map[SourceRef]int64, error) {
	out := make(map[SourceRef]int64)
	for chunk := range slices.Chunk(refs, maxRefsPerQuery) {
		values := strings.TrimSuffix(strings.Repeat("(?, ?),", len(chunk)), ",")
		args := make([]any, 0, 2*len(chunk))
		for _, r := range chunk {
			args = append(args, r.Provider, r.ProviderID)
		}
		rows, err := s.db.QueryContext(ctx,
			`SELECT id, provider, provider_id FROM images
			WHERE origin = 'library' AND (provider, provider_id) IN (VALUES `+values+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("find images by source: %w", err)
		}
		for rows.Next() {
			var id int64
			var r SourceRef
			if err := rows.Scan(&id, &r.Provider, &r.ProviderID); err != nil {
				rows.Close()
				return nil, fmt.Errorf("find images by source: %w", err)
			}
			out[r] = id
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("find images by source: %w", err)
		}
	}
	return out, nil
}

// ListOptions select and page library entries.
type ListOptions struct {
	CollectionID int64 // 0 means all images
	// Origin selects the user's library (default) or fetched images.
	Origin Origin
	// SavedSearchID limits fetched images to one saved search; 0 = all.
	SavedSearchID int64
	// FolderID limits folder images to one folder; 0 = all.
	FolderID    int64
	OldestFirst bool
	Limit       int // 0 means no limit
	Offset      int
}

// List returns entries, newest first unless OldestFirst is set.
//
// A collection lists all its members, whatever their origin: images from
// the user's folders can be in collections too (ADR 0053). Without a
// collection, only the chosen origin is listed (the user's library by
// default).
func (s *Store) List(ctx context.Context, opts ListOptions) ([]Entry, error) {
	var query string
	var args []any
	if opts.CollectionID != 0 && opts.Origin == "" {
		query = `SELECT ` + entryColumns + ` FROM images
			WHERE id IN (SELECT image_id FROM collection_images WHERE collection_id = ?)`
		args = append(args, opts.CollectionID)
	} else {
		origin := opts.Origin
		if origin == "" {
			origin = FromUser
		}
		query = `SELECT ` + entryColumns + ` FROM images WHERE origin = ?`
		args = append(args, string(origin))
		if opts.CollectionID != 0 {
			query += ` AND id IN (SELECT image_id FROM collection_images WHERE collection_id = ?)`
			args = append(args, opts.CollectionID)
		}
	}
	if opts.SavedSearchID != 0 {
		query += ` AND saved_search_id = ?`
		args = append(args, opts.SavedSearchID)
	}
	if opts.FolderID != 0 {
		query += ` AND folder_id = ?`
		args = append(args, opts.FolderID)
	}
	if opts.OldestFirst {
		query += ` ORDER BY added_at, id`
	} else {
		query += ` ORDER BY added_at DESC, id DESC`
	}
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

// IDs returns the IDs of the user's library (collectionID 0) or of all the
// members of one collection, folder images included, oldest first.
// Fetched images are not part of the library.
func (s *Store) IDs(ctx context.Context, collectionID int64) ([]int64, error) {
	query := `SELECT id FROM images WHERE origin = 'library' ORDER BY added_at, id`
	var args []any
	if collectionID != 0 {
		query = `SELECT id FROM images
			WHERE id IN (SELECT image_id FROM collection_images WHERE collection_id = ?)
			ORDER BY added_at, id`
		args = append(args, collectionID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list image IDs: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list image IDs: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FetchedIDs returns the IDs of the images fetched for a saved search,
// oldest first.
func (s *Store) FetchedIDs(ctx context.Context, savedSearchID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM images WHERE origin = 'fetched' AND saved_search_id = ? ORDER BY added_at, id`, savedSearchID)
	if err != nil {
		return nil, fmt.Errorf("list fetched image IDs: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list fetched image IDs: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HasSavedSearch reports whether a saved search exists.
func (s *Store) HasSavedSearch(ctx context.Context, id int64) (bool, error) {
	err := s.exists(ctx, "saved_searches", id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// HasCollection reports whether a collection exists.
func (s *Store) HasCollection(ctx context.Context, id int64) (bool, error) {
	err := s.exists(ctx, "collections", id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Stats summarizes the library.
type Stats struct {
	Images int
	Bytes  int64 // total size of the original files
}

// Stats returns the number of images in the user's library and their
// total size.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	return s.stats(ctx, FromUser)
}

// FetchedStats returns the number and total size of fetched images.
func (s *Store) FetchedStats(ctx context.Context) (Stats, error) {
	return s.stats(ctx, Fetched)
}

func (s *Store) stats(ctx context.Context, origin Origin) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*), coalesce(sum(file_size), 0) FROM images WHERE origin = ?`, string(origin)).
		Scan(&st.Images, &st.Bytes)
	if err != nil {
		return Stats{}, fmt.Errorf("library stats: %w", err)
	}
	return st, nil
}

// Keep moves a fetched image into the user's library: its files now live
// at filePath and thumbPath, and it counts as added now.
func (s *Store) Keep(ctx context.Context, id int64, filePath, thumbPath string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE images
		SET origin = 'library', saved_search_id = NULL, file_path = ?, thumbnail_path = ?,
			added_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ? AND origin = 'fetched'`, filePath, thumbPath, id)
	if err != nil {
		return fmt.Errorf("keep image %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: fetched image %d", ErrNotFound, id)
	}
	return nil
}

// HashRef pairs an entry with its perceptual hash.
type HashRef struct {
	ID    int64
	DHash uint64
}

// DHashes returns the perceptual hash of every entry, for finding visually
// similar images.
func (s *Store) DHashes(ctx context.Context) ([]HashRef, error) {
	// Folder files are not hashed (ADR 0053).
	rows, err := s.db.QueryContext(ctx, `SELECT id, dhash FROM images WHERE origin != 'folder'`)
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
		&img.Author.Name, &img.Author.URL, &img.Author.Credit, &status, &img.License.Name, &img.License.URL, &img.License.Notice,
		&img.Width, &img.Height, &img.MIMEType, &img.FileSize, &e.FilePath, &e.ThumbnailPath, &e.SHA256, &dhash, &added,
		&e.Origin, &e.SavedSearchID, &e.FolderID, &e.FileModTime)
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
