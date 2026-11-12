package libfiles

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFolder adds a local folder (ID 5) with a photo, a file outside it
// and a link leading out.
func withFolder(t *testing.T) (dataDir, folder string, s *Server, made *[]string) {
	t.Helper()
	dataDir, s = newLibrary(t)
	folder = filepath.Join(t.TempDir(), "Pictures")
	os.MkdirAll(filepath.Join(folder, "Trips"), 0o755)
	os.WriteFile(filepath.Join(folder, "Trips", "sea.jpg"), []byte("photo"), 0o644)
	secret := filepath.Join(filepath.Dir(folder), "secret.txt")
	os.WriteFile(secret, []byte("secret"), 0o600)
	os.Symlink(secret, filepath.Join(folder, "out.jpg"))

	var calls []string
	s.SetFolders(func(id int64) (string, error) {
		if id != 5 {
			return "", errors.New("no such folder")
		}
		return folder, nil
	}, func(rel string) error {
		calls = append(calls, rel)
		p := filepath.Join(dataDir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o700)
		return os.WriteFile(p, []byte("made thumbnail"), 0o600)
	})
	return dataDir, folder, s, &calls
}

func fget(t *testing.T, s *Server, path string) (int, string, http.Header) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, URLPrefix+path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body), rec.Header()
}

func TestServesFolderFiles(t *testing.T) {
	_, _, s, _ := withFolder(t)
	code, body, h := fget(t, s, "folders/5/Trips/sea.jpg")
	if code != http.StatusOK || body != "photo" {
		t.Fatalf("folder file: %d %q", code, body)
	}
	if !strings.Contains(h.Get("Cache-Control"), "no-cache") {
		t.Errorf("Cache-Control %q: a folder file may change under the same name", h.Get("Cache-Control"))
	}
	for _, bad := range []string{
		"folders/5/../secret.txt", // out of the folder
		"folders/5/out.jpg",       // a link out of the folder
		"folders/6/Trips/sea.jpg", // not a folder
		"folders/x/Trips/sea.jpg",
		"folders/5",
		"folders/5/Trips", // a directory
	} {
		if code, body, _ := fget(t, s, bad); code == http.StatusOK {
			t.Errorf("%s served %q", bad, body)
		}
	}
}

func TestFolderFilesAreNeverRemoved(t *testing.T) {
	_, folder, s, _ := withFolder(t)
	if err := s.Remove("folders/5/Trips/sea.jpg"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Remove = %v, want ErrReadOnly", err)
	}
	if _, err := os.Stat(filepath.Join(folder, "Trips", "sea.jpg")); err != nil {
		t.Errorf("the file is gone: %v", err)
	}
	got, err := s.OriginalPath("folders/5/Trips/sea.jpg")
	if err != nil || got != filepath.Join(folder, "Trips", "sea.jpg") {
		t.Errorf("OriginalPath = %q, %v", got, err)
	}
	if _, err := s.OriginalPath("folders/5/out.jpg"); err == nil {
		t.Error("OriginalPath followed a link out of the folder")
	}
}

func TestMakesAFolderThumbnailWhenAskedFor(t *testing.T) {
	_, _, s, calls := withFolder(t)
	code, body, _ := fget(t, s, "thumbs/folders/5/12-abc.jpg")
	if code != http.StatusOK || body != "made thumbnail" || len(*calls) != 1 {
		t.Fatalf("thumbnail: %d %q after %d calls", code, body, len(*calls))
	}
	// Made once; then served like any thumbnail.
	fget(t, s, "thumbs/folders/5/12-abc.jpg")
	if len(*calls) != 1 {
		t.Errorf("made %d times", len(*calls))
	}
	// Only folder thumbnails are made on request.
	if code, _, _ := fget(t, s, "thumbs/wallhaven/missing.jpg"); code != http.StatusNotFound || len(*calls) != 1 {
		t.Errorf("missing library thumbnail: %d, %d calls", code, len(*calls))
	}
}
