package libfiles

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newLibrary creates a data directory with library files and files that
// must never be served.
func newLibrary(t *testing.T) (string, *Server) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"images/wallhaven/abc.jpg": "original",
		"thumbs/wallhaven/abc.jpg": "thumbnail",
		"kisetsu.db":               "database",
		"tmp/dl-1":                 "partial download",
	}
	for name, content := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return root, s
}

func get(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://wails.localhost/", nil)
	// Set the raw path directly: httptest.NewRequest would clean "..".
	req.URL.Path = target
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServesLibraryFiles(t *testing.T) {
	_, s := newLibrary(t)

	for target, want := range map[string]string{
		"/library/images/wallhaven/abc.jpg": "original",
		"/library/thumbs/wallhaven/abc.jpg": "thumbnail",
	} {
		rec := get(t, s, http.MethodGet, target)
		body, _ := io.ReadAll(rec.Body)
		if rec.Code != http.StatusOK || string(body) != want {
			t.Errorf("%s: %d %q, want 200 %q", target, rec.Code, body, want)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff header", target)
		}
	}
}

func TestRefusesEverythingElse(t *testing.T) {
	root, s := newLibrary(t)
	// A symbolic link inside images/ that points at the database.
	if err := os.Symlink(filepath.Join(root, "kisetsu.db"), filepath.Join(root, "images", "link.db")); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{
		"/library/kisetsu.db",
		"/library/tmp/dl-1",
		"/library/images/../kisetsu.db",
		"/library/images/../../../../etc/passwd",
		"/library/images//etc/passwd",
		"/library/images/link.db",
		"/library/images/wallhaven",  // directory listing
		"/library/images/wallhaven/", // directory listing
		"/library/images/missing.jpg",
		"/library/",
		"/library/other/abc.jpg",
	} {
		if rec := get(t, s, http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
	}
}

func TestOnlyReadMethods(t *testing.T) {
	_, s := newLibrary(t)

	if rec := get(t, s, http.MethodHead, "/library/images/wallhaven/abc.jpg"); rec.Code != http.StatusOK {
		t.Errorf("HEAD: %d", rec.Code)
	}
	if rec := get(t, s, http.MethodDelete, "/library/images/wallhaven/abc.jpg"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: %d, want 405", rec.Code)
	}
}

func TestMiddlewarePassesOtherRequestsOn(t *testing.T) {
	_, s := newLibrary(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "app") })
	h := s.Middleware(next)

	if body := get(t, h, http.MethodGet, "/index.html").Body.String(); body != "app" {
		t.Errorf("app request got %q", body)
	}
	if body := get(t, h, http.MethodGet, "/library/thumbs/wallhaven/abc.jpg").Body.String(); body != "thumbnail" {
		t.Errorf("library request got %q", body)
	}
}

func TestURL(t *testing.T) {
	tests := map[string]string{
		"thumbs/wallhaven/abc.jpg": "/library/thumbs/wallhaven/abc.jpg",
		"images/a/../b.jpg":        "/library/images/b.jpg",
		"":                         "",
	}
	for in, want := range tests {
		if got := URL(in); got != want {
			t.Errorf("URL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemove(t *testing.T) {
	root, s := newLibrary(t)
	link := filepath.Join(root, "images", "link.db")
	if err := os.Symlink(filepath.Join(root, "kisetsu.db"), link); err != nil {
		t.Fatal(err)
	}

	if err := s.Remove(filepath.Join("images", "wallhaven", "abc.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "images", "wallhaven", "abc.jpg")); !os.IsNotExist(err) {
		t.Errorf("file still exists (err %v)", err)
	}
	if err := s.Remove("images/wallhaven/abc.jpg"); err != nil {
		t.Errorf("removing a missing file: %v", err)
	}

	for _, rel := range []string{"kisetsu.db", "tmp/dl-1", "images/../kisetsu.db", "images/link.db", "images/wallhaven", ""} {
		if err := s.Remove(rel); err == nil {
			t.Errorf("Remove(%q) succeeded", rel)
		}
	}
	for _, name := range []string{"kisetsu.db", "tmp/dl-1", "thumbs/wallhaven/abc.jpg"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s was removed: %v", name, err)
		}
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symbolic link was removed: %v", err)
	}
}

func TestOriginalPath(t *testing.T) {
	root, s := newLibrary(t)
	os.Symlink(filepath.Join(root, "kisetsu.db"), filepath.Join(root, "images", "link.jpg"))

	got, err := s.OriginalPath("images/wallhaven/abc.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "images", "wallhaven", "abc.jpg"); got != want || !filepath.IsAbs(got) {
		t.Errorf("OriginalPath = %q, want %q", got, want)
	}

	for _, rel := range []string{
		"thumbs/wallhaven/abc.jpg", // thumbnails are not wallpapers
		"kisetsu.db", "images/../kisetsu.db", "images/link.jpg", "images/wallhaven", "images/missing.jpg", "",
	} {
		if _, err := s.OriginalPath(rel); err == nil {
			t.Errorf("OriginalPath(%q) succeeded", rel)
		}
	}
}
