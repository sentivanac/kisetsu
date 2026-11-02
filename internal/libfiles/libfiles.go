// Package libfiles serves library images and thumbnails to the WebView
// and removes them when the user deletes an image.
//
// The WebView may not load file:// URLs, so files are served by the Wails
// asset server under URLPrefix. Only the images/ and thumbs/ directories
// are reachable, for serving and for removal; the database, temporary
// files and anything else in the data directory are not.
package libfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// URLPrefix is where library files appear to the frontend, for example
// /library/thumbs/wallhaven/abc.jpg.
const URLPrefix = "/library/"

// ErrOutsideLibrary means a path does not name a file in images/ or thumbs/.
var ErrOutsideLibrary = errors.New("path is not a library file")

// servedDirs are the only directories under the library root that can be
// served or removed.
var servedDirs = []string{"images", "thumbs"}

// Server serves library files. Each directory is opened with os.Root,
// which refuses paths and symbolic links that lead outside it.
type Server struct {
	dir   string // the library root, as given to New
	roots map[string]*os.Root
}

// New opens the served directories under root. They must exist.
func New(root string) (*Server, error) {
	s := &Server{dir: root, roots: map[string]*os.Root{}}
	for _, dir := range servedDirs {
		r, err := os.OpenRoot(path.Join(root, dir))
		if err != nil {
			s.Close()
			return nil, err
		}
		s.roots[dir] = r
	}
	return s, nil
}

// Close releases the directories.
func (s *Server) Close() error {
	var errs []error
	for _, r := range s.roots {
		errs = append(errs, r.Close())
	}
	return errors.Join(errs...)
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

	dir, name, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, URLPrefix), "/")
	root := s.roots[dir]
	// fs.ValidPath rejects "..", empty and absolute names; os.Root also
	// blocks symbolic links that point outside the directory.
	if !ok || root == nil || !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}

	f, err := root.Open(name)
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
	// A library path always refers to the same file.
	h.Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// Remove deletes a library file given by its stored path, such as
// "images/wallhaven/abc.jpg". Paths outside images/ and thumbs/ are
// refused, so a damaged database record cannot delete other files. A file
// that is already gone is not an error.
func (s *Server) Remove(relPath string) error {
	dir, name, ok := strings.Cut(filepath.ToSlash(relPath), "/")
	root := s.roots[dir]
	if !ok || root == nil || !fs.ValidPath(name) {
		return fmt.Errorf("%w: %q", ErrOutsideLibrary, relPath)
	}
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %q is not a regular file", ErrOutsideLibrary, relPath)
	}
	return root.Remove(name)
}

// OriginalPath returns the absolute path of an original image given by its
// stored path, such as "images/wallhaven/abc.jpg", after checking that it
// is a regular file inside images/. Thumbnails and anything else are
// refused, so external programs only ever receive library originals.
func (s *Server) OriginalPath(relPath string) (string, error) {
	dir, name, ok := strings.Cut(filepath.ToSlash(relPath), "/")
	if !ok || dir != "images" || !fs.ValidPath(name) {
		return "", fmt.Errorf("%w: %q", ErrOutsideLibrary, relPath)
	}
	info, err := s.roots[dir].Lstat(name)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %q is not a regular file", ErrOutsideLibrary, relPath)
	}
	abs, err := filepath.Abs(filepath.Join(s.dir, dir, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	return abs, nil
}

// URL returns the frontend URL of a library path such as
// "thumbs/wallhaven/abc.jpg", or "" for an empty path.
func URL(relPath string) string {
	if relPath == "" {
		return ""
	}
	return URLPrefix + strings.TrimPrefix(path.Clean("/"+relPath), "/")
}
