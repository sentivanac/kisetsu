package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrFolderExists means the folder was added before.
var ErrFolderExists = errors.New("folder already added")

// Folder is a local folder the user added; its images are used where they
// are (ADR 0053).
type Folder struct {
	ID      int64
	Path    string // absolute
	AddedAt time.Time
}

// AddFolder records a folder. The caller checks that path is an absolute
// directory.
func (s *Store) AddFolder(ctx context.Context, path string) (Folder, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM folders WHERE path = ?`, path).Scan(&n); err != nil {
		return Folder{}, fmt.Errorf("add folder: %w", err)
	}
	if n > 0 {
		return Folder{}, fmt.Errorf("%w: %s", ErrFolderExists, path)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO folders (path) VALUES (?)`, path)
	if err != nil {
		return Folder{}, fmt.Errorf("add folder: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Folder{}, fmt.Errorf("add folder: %w", err)
	}
	return s.Folder(ctx, id)
}

// Folder returns one folder.
func (s *Store) Folder(ctx context.Context, id int64) (Folder, error) {
	var f Folder
	var added string
	err := s.db.QueryRowContext(ctx, `SELECT id, path, added_at FROM folders WHERE id = ?`, id).Scan(&f.ID, &f.Path, &added)
	if errors.Is(err, sql.ErrNoRows) {
		return Folder{}, fmt.Errorf("%w: folder %d", ErrNotFound, id)
	}
	if err != nil {
		return Folder{}, fmt.Errorf("read folder %d: %w", id, err)
	}
	if f.AddedAt, err = time.Parse(timeLayout, added); err != nil {
		return Folder{}, fmt.Errorf("read folder %d: added_at: %w", id, err)
	}
	return f, nil
}

// Folders returns all folders, in the order they were added.
func (s *Store) Folders(ctx context.Context) ([]Folder, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM folders ORDER BY added_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("list folders: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	out := make([]Folder, 0, len(ids))
	for _, id := range ids {
		f, err := s.Folder(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// RemoveFolder forgets a folder and its images (their collection
// memberships and history go with them). The files in the folder are
// never touched; the caller removes the thumbnails Kisetsu made, whose
// paths are returned.
func (s *Store) RemoveFolder(ctx context.Context, id int64) (thumbnails []string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("remove folder %d: %w", id, err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT thumbnail_path FROM images WHERE folder_id = ? AND thumbnail_path != ''`, id)
	if err != nil {
		return nil, fmt.Errorf("remove folder %d: %w", id, err)
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, fmt.Errorf("remove folder %d: %w", id, err)
		}
		thumbnails = append(thumbnails, p)
	}
	rows.Close()
	res, err := tx.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("remove folder %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: folder %d", ErrNotFound, id)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("remove folder %d: %w", id, err)
	}
	return thumbnails, nil
}

// FolderFile is what syncing needs to know about a folder image.
type FolderFile struct {
	ID            int64
	Size          int64
	ModTime       int64
	ThumbnailPath string
}

// FolderFiles returns the folder's images by file path.
func (s *Store) FolderFiles(ctx context.Context, folderID int64) (map[string]FolderFile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file_path, file_size, coalesce(file_mod_time, 0), thumbnail_path FROM images WHERE folder_id = ?`, folderID)
	if err != nil {
		return nil, fmt.Errorf("list folder files: %w", err)
	}
	defer rows.Close()
	out := map[string]FolderFile{}
	for rows.Next() {
		var f FolderFile
		var path string
		if err := rows.Scan(&f.ID, &path, &f.Size, &f.ModTime, &f.ThumbnailPath); err != nil {
			return nil, fmt.Errorf("list folder files: %w", err)
		}
		out[path] = f
	}
	return out, rows.Err()
}

// FolderChanges is the result of reading a folder: files to add, files
// whose content changed, and images whose files are gone.
type FolderChanges struct {
	Add     []Entry
	Update  []Entry // by ID: size, dimensions, type and modification time
	Removed []int64
}

// Empty reports whether nothing changed.
func (c FolderChanges) Empty() bool {
	return len(c.Add) == 0 && len(c.Update) == 0 && len(c.Removed) == 0
}

// ApplyFolderChanges stores the changes of one folder in one transaction.
// A changed file's thumbnail path is cleared, so a new one is made.
func (s *Store) ApplyFolderChanges(ctx context.Context, folderID int64, c FolderChanges) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync folder %d: %w", folderID, err)
	}
	defer tx.Rollback()
	for _, e := range c.Add {
		e.Origin, e.FolderID, e.SHA256, e.SavedSearchID = FromFolder, folderID, "", 0
		if _, err := insert(ctx, tx, e); err != nil {
			return fmt.Errorf("sync folder %d: add %s: %w", folderID, e.FilePath, err)
		}
	}
	for _, e := range c.Update {
		_, err := tx.ExecContext(ctx, `UPDATE images
			SET width = ?, height = ?, mime_type = ?, file_size = ?, file_mod_time = ?, thumbnail_path = ''
			WHERE id = ? AND folder_id = ?`,
			e.Image.Width, e.Image.Height, e.Image.MIMEType, e.Image.FileSize, e.FileModTime, e.ID, folderID)
		if err != nil {
			return fmt.Errorf("sync folder %d: update %s: %w", folderID, e.FilePath, err)
		}
	}
	for _, id := range c.Removed {
		if _, err := tx.ExecContext(ctx, `DELETE FROM images WHERE id = ? AND folder_id = ?`, id, folderID); err != nil {
			return fmt.Errorf("sync folder %d: remove image %d: %w", folderID, id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sync folder %d: %w", folderID, err)
	}
	return nil
}

// SetThumbnail records the thumbnail made for an image.
func (s *Store) SetThumbnail(ctx context.Context, id int64, path string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE images SET thumbnail_path = ? WHERE id = ?`, path, id); err != nil {
		return fmt.Errorf("set thumbnail of image %d: %w", id, err)
	}
	return nil
}

// FolderIDs returns the IDs of a folder's images, oldest first.
func (s *Store) FolderIDs(ctx context.Context, folderID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM images WHERE folder_id = ? ORDER BY added_at, id`, folderID)
	if err != nil {
		return nil, fmt.Errorf("list folder image IDs: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list folder image IDs: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FolderCounts returns the number of images of each folder.
func (s *Store) FolderCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT folder_id, count(*) FROM images WHERE folder_id IS NOT NULL GROUP BY folder_id`)
	if err != nil {
		return nil, fmt.Errorf("count folder images: %w", err)
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("count folder images: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}
