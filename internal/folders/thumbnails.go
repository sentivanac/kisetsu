package folders

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
)

// ThumbnailPath is where the thumbnail of a folder image is stored, under
// Kisetsu's data directory. The file's modification time is part of the
// name, so a changed file gets a new thumbnail and the window does not
// show a cached old one.
func ThumbnailPath(e library.Entry) string {
	return fmt.Sprintf("thumbs/folders/%d/%d-%x.jpg", e.FolderID, e.ID, e.FileModTime)
}

// maxThumbnailing is how many thumbnails are made at once: the window asks
// for a whole page of them, and each decodes a full photo.
const maxThumbnailing = 2

// Thumbnails makes the thumbnails of folder images when the window first
// shows them (libfiles calls Make for a missing one).
type Thumbnails struct {
	Store *library.Store
	// Original returns the absolute path of an image file (libfiles).
	Original func(relPath string) (string, error)
	// Dir is the data directory, which holds thumbs/.
	Dir string

	once     sync.Once
	slots    chan struct{}
	mu       sync.Mutex
	inFlight map[string]*sync.WaitGroup
}

// Make makes the thumbnail at relPath, as returned by ThumbnailPath, and
// records it. Requests for the same thumbnail wait for one another.
func (t *Thumbnails) Make(relPath string) error {
	t.once.Do(func() {
		t.slots = make(chan struct{}, maxThumbnailing)
		t.inFlight = map[string]*sync.WaitGroup{}
	})
	t.mu.Lock()
	if wg := t.inFlight[relPath]; wg != nil {
		t.mu.Unlock()
		wg.Wait()
		return nil // the first request made it, or failed for both
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	t.inFlight[relPath] = wg
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		delete(t.inFlight, relPath)
		t.mu.Unlock()
		wg.Done()
	}()

	t.slots <- struct{}{}
	defer func() { <-t.slots }()
	return t.make(relPath)
}

func (t *Thumbnails) make(relPath string) error {
	ctx := context.Background()
	folderID, imageID, modHex, ok := parseThumbnailPath(relPath)
	if !ok {
		return fmt.Errorf("not a folder thumbnail: %q", relPath)
	}
	e, err := t.Store.Get(ctx, imageID)
	if err != nil {
		return err
	}
	// Only the thumbnail of this file as it is now.
	if e.FolderID != folderID || strconv.FormatInt(e.FileModTime, 16) != modHex {
		return fmt.Errorf("thumbnail %q is out of date", relPath)
	}
	src, err := t.Original(e.FilePath)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	data, err := imaging.Thumbnail(f)
	f.Close()
	if err != nil {
		return err
	}

	dst := filepath.Join(t.Dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".thumb-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	return t.Store.SetThumbnail(ctx, imageID, relPath)
}

// parseThumbnailPath reads "thumbs/folders/<folder>/<image>-<hex>.jpg".
func parseThumbnailPath(relPath string) (folderID, imageID int64, modHex string, ok bool) {
	rest, found := strings.CutPrefix(filepath.ToSlash(relPath), "thumbs/folders/")
	if !found {
		return 0, 0, "", false
	}
	folder, file, found := strings.Cut(rest, "/")
	name, found2 := strings.CutSuffix(file, ".jpg")
	id, hex, found3 := strings.Cut(name, "-")
	if !found || !found2 || !found3 || strings.Contains(file, "/") {
		return 0, 0, "", false
	}
	var err1, err2 error
	folderID, err1 = strconv.ParseInt(folder, 10, 64)
	imageID, err2 = strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil || folderID <= 0 || imageID <= 0 {
		return 0, 0, "", false
	}
	if _, err := strconv.ParseUint(hex, 16, 64); err != nil {
		return 0, 0, "", false
	}
	return folderID, imageID, hex, true
}
