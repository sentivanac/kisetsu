// Package libfiles serves library images and thumbnails to the WebView.
//
// The WebView may not load file:// URLs, so files are served by the Wails
// asset server under URLPrefix. Only the images/ and thumbs/ directories
// are reachable; the database, temporary files and anything else in the
// data directory are not.
package libfiles

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// URLPrefix is where library files appear to the frontend, for example
// /library/thumbs/wallhaven/abc.jpg.
const URLPrefix = "/library/"

// servedDirs are the only directories under the library root that can be
// served.
var servedDirs = []string{"images", "thumbs"}

// Server serves library files. Each directory is opened with os.Root,
// which refuses paths and symbolic links that lead outside it.
type Server struct {
	roots map[string]*os.Root
}

// New opens the served directories under root. They must exist.
func New(root string) (*Server, error) {
	s := &Server{roots: map[string]*os.Root{}}
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

// URL returns the frontend URL of a library path such as
// "thumbs/wallhaven/abc.jpg", or "" for an empty path.
func URL(relPath string) string {
	if relPath == "" {
		return ""
	}
	return URLPrefix + strings.TrimPrefix(path.Clean("/"+relPath), "/")
}
