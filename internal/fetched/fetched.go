// Package fetched manages images fetched automatically from saved
// searches: keeping them in the library and holding them to a size limit.
// They live in the cache, so removing old ones is expected (ADR 0028).
package fetched

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/sentivanac/kisetsu/internal/library"
)

// MaxBytes is the space fetched images may use (user decision: 200 MB).
const MaxBytes = 200 << 20

// ErrNotFetched means the image is not a fetched image.
var ErrNotFetched = errors.New("not a fetched image")

// Files moves and removes library files; *libfiles.Server.
type Files interface {
	CopyFetched(relPath string) (string, error)
	Remove(relPath string) error
}

// Service keeps and prunes fetched images.
type Service struct {
	Library  *library.Store
	Files    Files
	Log      *slog.Logger
	MaxBytes int64 // 0 means MaxBytes
}

func (s *Service) limit() int64 {
	if s.MaxBytes > 0 {
		return s.MaxBytes
	}
	return MaxBytes
}

// Keep moves a fetched image into the user's library. The files are
// copied first, then the database points at the copies, and only then are
// the fetched files removed; if the database update fails, the copies are
// removed and the image stays fetched.
func (s *Service) Keep(ctx context.Context, id int64) (library.Entry, error) {
	e, err := s.Library.Get(ctx, id)
	if err != nil {
		return library.Entry{}, err
	}
	if e.Origin != library.Fetched {
		return library.Entry{}, fmt.Errorf("%w: image %d", ErrNotFetched, id)
	}

	file, err := s.Files.CopyFetched(e.FilePath)
	if err != nil {
		return library.Entry{}, fmt.Errorf("keep image %d: %w", id, err)
	}
	thumb, err := s.Files.CopyFetched(e.ThumbnailPath)
	if err != nil {
		s.Files.Remove(file)
		return library.Entry{}, fmt.Errorf("keep image %d: %w", id, err)
	}
	if err := s.Library.Keep(ctx, id, file, thumb); err != nil {
		s.Files.Remove(file)
		s.Files.Remove(thumb)
		return library.Entry{}, err
	}
	for _, old := range []string{e.FilePath, e.ThumbnailPath} {
		if err := s.Files.Remove(old); err != nil {
			s.Log.Warn("fetched file not removed after keeping", "entry", id, "path", old, "err", err)
		}
	}
	s.Log.Info("fetched image kept", "entry", id)
	return s.Library.Get(ctx, id)
}

// Prune removes the oldest fetched images until they fit in the limit.
// Images in protect (for example the wallpaper on screen) are never
// removed, and the user's library is never touched. It returns how many
// images were removed and how many bytes were freed.
func (s *Service) Prune(ctx context.Context, protect ...int64) (int, int64, error) {
	st, err := s.Library.FetchedStats(ctx)
	if err != nil || st.Bytes <= s.limit() {
		return 0, 0, err
	}
	oldest, err := s.Library.List(ctx, library.ListOptions{Origin: library.Fetched, OldestFirst: true})
	if err != nil {
		return 0, 0, err
	}

	used, removed, freed := st.Bytes, 0, int64(0)
	for _, e := range oldest {
		if used <= s.limit() {
			break
		}
		if isProtected(e.ID, protect) {
			continue
		}
		if err := s.Library.Delete(ctx, e.ID); err != nil {
			return removed, freed, err
		}
		for _, p := range []string{e.FilePath, e.ThumbnailPath} {
			if err := s.Files.Remove(p); err != nil {
				s.Log.Warn("fetched file not removed", "entry", e.ID, "path", p, "err", err)
			}
		}
		used -= e.Image.FileSize
		freed += e.Image.FileSize
		removed++
	}
	if removed > 0 {
		s.Log.Info("fetched images pruned", "removed", removed, "freed_bytes", freed, "limit_bytes", s.limit())
	}
	return removed, freed, nil
}

func isProtected(id int64, protect []int64) bool {
	for _, p := range protect {
		if p == id {
			return true
		}
	}
	return false
}
