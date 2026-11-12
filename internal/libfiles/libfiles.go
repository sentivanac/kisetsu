// Package libfiles serves library images and thumbnails to the WebView,
// removes them when the user deletes an image, and moves fetched images
// into the library when the user keeps them.
//
// The WebView may not load file:// URLs, so files are served by the Wails
// asset server under URLPrefix. Stored paths look like
//
//	images/<provider>/<file>          the user's library (data directory)
//	thumbs/<provider>/<file>
//	fetched/images/<provider>/<file>  fetched images (cache directory)
//	fetched/thumbs/<provider>/<file>
//	folders/<folder id>/<file>        a local folder's files (ADR 0053)
//	thumbs/folders/<folder id>/<file> thumbnails Kisetsu made for them
//
// Only these places are reachable, for every operation; the database,
// temporary files and anything else are not. Each directory is opened with
// os.Root, which refuses paths and symbolic links that lead outside it.
// A local folder's files are only ever read: Remove refuses them.
package libfiles

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// URLPrefix is where library files appear to the frontend, for example
// /library/thumbs/wallhaven/abc.jpg.
const URLPrefix = "/library/"

// ErrOutsideLibrary means a path does not name a library file.
var ErrOutsideLibrary = errors.New("path is not a library file")

// ErrReadOnly means the file belongs to the user's own folder, which
// Kisetsu never changes.
var ErrReadOnly = errors.New("file in a local folder is never changed")

// Server serves and manages library files.
type Server struct {
	dataDir    string // holds images/ and thumbs/
	fetchedDir string // holds fetched images/ and thumbs/
	roots      map[string]*os.Root

	// folderPath looks up a local folder; nil until SetFolders.
	folderPath func(id int64) (string, error)
	// makeThumbnail makes a missing folder thumbnail on request; nil until
	// SetFolders.
	makeThumbnail func(relPath string) error
	mu            sync.Mutex
	folderRoots   map[int64]*os.Root
}

// location is a checked library path.
type location struct {
	root    *os.Root
	rootKey string // "images", "thumbs", "fetched"
	name    string // path inside the root
	abs     string // absolute path on disk
	thumb   bool
	folder  bool // in a local folder: read only
}

// New opens the library under dataDir and the fetched images under
// fetchedDir. Both must contain images/ and thumbs/ directories.
func New(dataDir, fetchedDir string) (*Server, error) {
	s := &Server{dataDir: dataDir, fetchedDir: fetchedDir, roots: map[string]*os.Root{}, folderRoots: map[int64]*os.Root{}}
	for key, dir := range map[string]string{
		"images":  filepath.Join(dataDir, "images"),
		"thumbs":  filepath.Join(dataDir, "thumbs"),
		"fetched": fetchedDir,
	} {
		r, err := os.OpenRoot(dir)
		if err != nil {
			s.Close()
			return nil, err
		}
		s.roots[key] = r
	}
	return s, nil
}

// SetFolders lets the server reach local folders: path returns a folder's
// absolute path by its ID, and makeThumbnail, if not nil, makes a missing
// thumbnail under "thumbs/folders/" when the window asks for it.
func (s *Server) SetFolders(path func(id int64) (string, error), makeThumbnail func(relPath string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folderPath, s.makeThumbnail = path, makeThumbnail
}

// ForgetFolder closes a folder that was removed.
func (s *Server) ForgetFolder(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.folderRoots[id]; r != nil {
		r.Close()
		delete(s.folderRoots, id)
	}
}

// folderRoot opens a local folder once, with os.Root like the library.
func (s *Server) folderRoot(id int64) (*os.Root, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.folderPath == nil {
		return nil, "", fmt.Errorf("%w: no local folders", ErrOutsideLibrary)
	}
	dir, err := s.folderPath(id)
	if err != nil {
		return nil, "", fmt.Errorf("%w: folder %d: %v", ErrOutsideLibrary, id, err)
	}
	if r := s.folderRoots[id]; r != nil {
		return r, dir, nil
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, "", err
	}
	s.folderRoots[id] = r
	return r, dir, nil
}

// Close releases the directories.
func (s *Server) Close() error {
	var errs []error
	for _, r := range s.roots {
		errs = append(errs, r.Close())
	}
	s.mu.Lock()
	for _, r := range s.folderRoots {
		errs = append(errs, r.Close())
	}
	s.mu.Unlock()
	return errors.Join(errs...)
}

// resolve checks a stored path and says where it lives.
func (s *Server) resolve(relPath string) (location, error) {
	rel := filepath.ToSlash(relPath)
	key, name, ok := strings.Cut(rel, "/")
	if key == "folders" {
		return s.resolveFolder(relPath, name)
	}
	root := s.roots[key]
	if !ok || root == nil || !fs.ValidPath(name) {
		return location{}, fmt.Errorf("%w: %q", ErrOutsideLibrary, relPath)
	}
	loc := location{root: root, rootKey: key, name: name, thumb: key == "thumbs"}
	switch key {
	case "fetched":
		kind, rest, ok := strings.Cut(name, "/")
		if !ok || (kind != "images" && kind != "thumbs") || rest == "" {
			return location{}, fmt.Errorf("%w: %q", ErrOutsideLibrary, relPath)
		}
		loc.thumb = kind == "thumbs"
		loc.abs = filepath.Join(s.fetchedDir, filepath.FromSlash(name))
	default:
		loc.abs = filepath.Join(s.dataDir, key, filepath.FromSlash(name))
	}
	return loc, nil
}

// resolveFolder checks "<folder id>/<file>" in a local folder.
func (s *Server) resolveFolder(relPath, name string) (location, error) {
	idText, inner, ok := strings.Cut(name, "/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if !ok || err != nil || id <= 0 || !fs.ValidPath(inner) {
		return location{}, fmt.Errorf("%w: %q", ErrOutsideLibrary, relPath)
	}
	root, dir, err := s.folderRoot(id)
	if err != nil {
		return location{}, err
	}
	return location{root: root, rootKey: "folders", name: inner, abs: filepath.Join(dir, filepath.FromSlash(inner)), folder: true}, nil
}

// regular checks that loc is a regular file (not a link or directory).
func (loc location) regular() error {
	info, err := loc.root.Lstat(loc.name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q is not a regular file", ErrOutsideLibrary, loc.name)
	}
	return nil
}

// Middleware serves URLPrefix requests and passes all others to next. It
// is a Wails assetserver.Middleware, so it works in production builds and
// with the Vite dev server alike.
func (s *Server) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, URLPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		s.ServeHTTP(w, r)
	})
}

// ServeHTTP serves one library file.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	loc, err := s.resolve(strings.TrimPrefix(r.URL.Path, URLPrefix))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := loc.root.Open(loc.name)
	if errors.Is(err, fs.ErrNotExist) && loc.rootKey == "thumbs" && strings.HasPrefix(loc.name, "folders/") {
		// A local folder's thumbnail is made when it is first shown.
		s.mu.Lock()
		makeThumb := s.makeThumbnail
		s.mu.Unlock()
		if makeThumb != nil && makeThumb(strings.TrimPrefix(r.URL.Path, URLPrefix)) == nil {
			f, err = loc.root.Open(loc.name)
		}
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() { // no directory listings
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	if loc.folder {
		// The user may replace a file in a folder under the same name:
		// check with the modification time every time.
		h.Set("Cache-Control", "private, no-cache")
	} else {
		// A library path always refers to the same file.
		h.Set("Cache-Control", "private, max-age=86400")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// Remove deletes a library file given by its stored path. Paths outside
// the library are refused, so a damaged database record cannot delete
// other files. A file that is already gone is not an error.
func (s *Server) Remove(relPath string) error {
	loc, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if loc.folder {
		return fmt.Errorf("%w: %q", ErrReadOnly, relPath)
	}
	if err := loc.regular(); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return loc.root.Remove(loc.name)
}

// OriginalPath returns the absolute path of an image (library, fetched or
// in a local folder)
// after checking that it is a regular file. Thumbnails are refused, so
// external programs only ever receive images.
func (s *Server) OriginalPath(relPath string) (string, error) {
	loc, err := s.resolve(relPath)
	if err != nil {
		return "", err
	}
	if loc.thumb {
		return "", fmt.Errorf("%w: %q is a thumbnail", ErrOutsideLibrary, relPath)
	}
	if err := loc.regular(); err != nil {
		return "", err
	}
	return filepath.Abs(loc.abs)
}

// CopyFetched copies a fetched file ("fetched/images/…" or
// "fetched/thumbs/…") into the library and returns the new stored path.
// It copies with O_EXCL, so an existing library file is never replaced (a
// numeric suffix is added instead). The fetched file stays; the caller
// removes it once the database points at the copy. Copying, not renaming,
// works when the cache and the data directory are on different file
// systems.
func (s *Server) CopyFetched(relPath string) (string, error) {
	src, err := s.resolve(relPath)
	if err != nil {
		return "", err
	}
	if src.rootKey != "fetched" {
		return "", fmt.Errorf("%w: %q is not a fetched file", ErrOutsideLibrary, relPath)
	}
	if err := src.regular(); err != nil {
		return "", err
	}
	dstKey := "images"
	if src.thumb {
		dstKey = "thumbs"
	}
	dstRoot := s.roots[dstKey]
	_, inner, _ := strings.Cut(src.name, "/") // drop "images/" or "thumbs/"
	dir, file := path.Split(inner)
	if dir != "" {
		if err := dstRoot.MkdirAll(strings.TrimSuffix(dir, "/"), 0o700); err != nil {
			return "", err
		}
	}

	ext := path.Ext(file)
	base := strings.TrimSuffix(file, ext)
	for i := 1; i <= 100; i++ {
		name := dir + file
		if i > 1 {
			name = fmt.Sprintf("%s%s-%d%s", dir, base, i, ext)
		}
		err := copyExclusive(src.root, src.name, dstRoot, name)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return dstKey + "/" + name, nil
	}
	return "", fmt.Errorf("no free file name for %s in %s", file, dstKey)
}

// copyExclusive copies a file between roots, failing if dst exists. A
// partial copy is removed.
func copyExclusive(srcRoot *os.Root, srcName string, dstRoot *os.Root, dstName string) error {
	in, err := srcRoot.Open(srcName)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := dstRoot.OpenFile(dstName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, cerr := io.Copy(out, in)
	if err := errors.Join(cerr, out.Close()); err != nil {
		dstRoot.Remove(dstName)
		return err
	}
	return nil
}

// URL returns the frontend URL of a stored path such as
// "thumbs/wallhaven/abc.jpg", or "" for an empty path.
func URL(relPath string) string {
	if relPath == "" {
		return ""
	}
	return URLPrefix + strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(relPath)), "/")
}
