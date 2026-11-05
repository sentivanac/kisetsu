package downloads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
)

// picture returns a JPEG; variant changes the content, quality only the
// encoding (so different qualities give different files that look alike).
func picture(t *testing.T, variant, quality int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 400))
	for y := range 400 {
		for x := range 640 {
			v := 127 + 127*math.Sin(float64(x*(variant+1))/40+float64(y)/60)
			img.Set(x, y, color.RGBA{uint8(v), uint8(255 - v), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// recorder collects progress updates.
type recorder struct {
	mu      sync.Mutex
	updates []Progress
	done    chan Progress
}

func newRecorder() *recorder {
	return &recorder{done: make(chan Progress, 16)}
}

func (r *recorder) notify(p Progress) {
	r.mu.Lock()
	r.updates = append(r.updates, p)
	r.mu.Unlock()
	if p.State.Finished() {
		r.done <- p
	}
}

func (r *recorder) wait(t *testing.T) Progress {
	t.Helper()
	select {
	case p := <-r.done:
		return p
	case <-time.After(10 * time.Second):
		t.Fatal("download did not finish")
		return Progress{}
	}
}

func (r *recorder) states(key string) []State {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []State
	for _, p := range r.updates {
		if p.Key == key && (len(out) == 0 || out[len(out)-1] != p.State) {
			out = append(out, p.State)
		}
	}
	return out
}

type env struct {
	root  string
	store *library.Store
	rec   *recorder
	mgr   *Manager
	srv   *httptest.Server
}

func newEnv(t *testing.T, handler http.HandlerFunc) *env {
	t.Helper()
	root := t.TempDir()
	db, err := database.Open(context.Background(), filepath.Join(root, database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	e := &env{root: root, store: library.NewStore(db), rec: newRecorder(), srv: srv}
	e.mgr, err = New(Config{
		Client:      srv.Client(),
		UserAgent:   "Kisetsu/test",
		Root:        root,
		FetchedRoot: filepath.Join(root, "cache", "fetched"),
		Store:       e.store,
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Notify:      e.rec.notify,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.mgr.Close)
	return e
}

// image returns provider metadata pointing at path on the test server.
func (e *env) image(id, path string) images.Image {
	return images.Image{
		Provider:     "wallhaven",
		ProviderID:   id,
		PageURL:      "https://wallhaven.cc/w/" + id,
		ThumbnailURL: e.srv.URL + "/thumb.jpg",
		DownloadURL:  e.srv.URL + path,
		Width:        9999, // providers can be wrong; the decoded size wins
		Height:       9999,
		Author:       images.Author{Name: "Someone"},
		SourceURL:    "https://example.org/source",
		License:      images.License{Status: images.LicenseUnknown, Notice: "Property of the owners."},
	}
}

func (e *env) download(t *testing.T, img images.Image) Progress {
	t.Helper()
	if _, err := e.mgr.Enqueue(context.Background(), img); err != nil {
		t.Fatal(err)
	}
	return e.rec.wait(t)
}

func tmpFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, tmpDir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func serve(files map[string][]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(data)
	}
}

func TestDownloadStoresFileThumbnailAndAttribution(t *testing.T) {
	data := picture(t, 0, 90)
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": data}))
	img := e.image("abc123", "/a.jpg")

	p := e.download(t, img)
	if p.State != Done || p.EntryID == 0 || len(p.SimilarIDs) != 0 {
		t.Fatalf("progress = %+v", p)
	}
	if got := e.rec.states(Key(img)); !slices.Equal(got, []State{Queued, Downloading, Processing, Done}) {
		t.Errorf("states = %v", got)
	}

	entry, err := e.store.Get(context.Background(), p.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.FilePath != filepath.Join("images", "wallhaven", "abc123.jpg") ||
		entry.ThumbnailPath != filepath.Join("thumbs", "wallhaven", "abc123.jpg") {
		t.Errorf("paths = %q, %q", entry.FilePath, entry.ThumbnailPath)
	}
	if entry.Image.Width != 640 || entry.Image.Height != 400 || entry.Image.FileSize != int64(len(data)) {
		t.Errorf("stored %dx%d, %d bytes; want the decoded 640x400, %d bytes",
			entry.Image.Width, entry.Image.Height, entry.Image.FileSize, len(data))
	}
	if entry.Image.Author.Name != "Someone" || entry.Image.License.Notice != "Property of the owners." ||
		entry.Image.SourceURL != "https://example.org/source" {
		t.Errorf("attribution lost: %+v", entry.Image)
	}

	stored, err := os.ReadFile(filepath.Join(e.root, entry.FilePath))
	if err != nil || !bytes.Equal(stored, data) {
		t.Errorf("stored file differs from download (err %v)", err)
	}
	info, err := os.Stat(filepath.Join(e.root, entry.FilePath))
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != filePerm {
		t.Errorf("file permissions = %v (err %v), want %o", info.Mode().Perm(), err, filePerm)
	}
	thumb, err := os.ReadFile(filepath.Join(e.root, entry.ThumbnailPath))
	if err != nil {
		t.Fatal(err)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb)); err != nil || cfg.Width != imaging.ThumbnailSize {
		t.Errorf("thumbnail %+v, %v", cfg, err)
	}
	if left := tmpFiles(t, e.root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestEnqueueRejectsImagesAlreadyInLibrary(t *testing.T) {
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": picture(t, 0, 90)}))
	img := e.image("abc", "/a.jpg")
	e.download(t, img)

	if _, err := e.mgr.Enqueue(context.Background(), img); !errors.Is(err, library.ErrAlreadyInLibrary) {
		t.Errorf("err = %v, want ErrAlreadyInLibrary", err)
	}
}

func TestEnqueueRejectsInvalidImages(t *testing.T) {
	e := newEnv(t, serve(nil))
	img := e.image("abc", "/a.jpg")
	img.DownloadURL = "file:///etc/passwd"

	if _, err := e.mgr.Enqueue(context.Background(), img); err == nil {
		t.Error("a file:// download URL was accepted")
	}
}

func TestExactDuplicateIsNotStoredTwice(t *testing.T) {
	data := picture(t, 0, 90)
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": data, "/same.jpg": data}))

	first := e.download(t, e.image("first", "/a.jpg"))
	second := e.download(t, e.image("second", "/same.jpg"))

	if second.State != Duplicate || second.EntryID != first.EntryID {
		t.Errorf("second = %+v, want duplicate of %d", second, first.EntryID)
	}
	if st, _ := e.store.Stats(context.Background()); st.Images != 1 {
		t.Errorf("library has %d images", st.Images)
	}
	if _, err := os.Stat(filepath.Join(e.root, "images", "wallhaven", "second.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("duplicate file was written (err %v)", err)
	}
	if left := tmpFiles(t, e.root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestSimilarImageIsStoredAndReported(t *testing.T) {
	e := newEnv(t, serve(map[string][]byte{
		"/hq.jpg":    picture(t, 0, 95),
		"/lq.jpg":    picture(t, 0, 30), // same picture, other compression
		"/other.jpg": picture(t, 3, 95),
	}))

	hq := e.download(t, e.image("hq", "/hq.jpg"))
	lq := e.download(t, e.image("lq", "/lq.jpg"))
	other := e.download(t, e.image("other", "/other.jpg"))

	if lq.State != Done || !slices.Equal(lq.SimilarIDs, []int64{hq.EntryID}) {
		t.Errorf("re-compressed copy = %+v, want stored and similar to %d", lq, hq.EntryID)
	}
	if other.State != Done || len(other.SimilarIDs) != 0 {
		t.Errorf("different picture = %+v, want no similar images", other)
	}
}

func TestFailedDownloadsLeaveNothingBehind(t *testing.T) {
	big := strings.Repeat("x", 1024)
	tests := map[string]struct {
		handler http.HandlerFunc
		want    error
	}{
		"not found": {func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, ErrHTTP},
		"declared too large": {func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(MaxFileSize+1))
			w.WriteHeader(http.StatusOK)
		}, ErrFileTooLarge},
		"streamed too large": {func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg") // no Content-Length
			for range MaxFileSize/len(big) + 2 {
				if _, err := io.WriteString(w, big); err != nil {
					return
				}
			}
		}, ErrFileTooLarge},
		"not an image": {func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<html>Please log in</html>"))
		}, imaging.ErrUnsupportedFormat},
		"damaged image": {func(w http.ResponseWriter, r *http.Request) {
			data := picture(t, 0, 90)
			w.Write(data[:len(data)/3])
		}, imaging.ErrCorrupt},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, tt.handler)

			p := e.download(t, e.image("abc", "/a.jpg"))

			if p.State != Failed || !errors.Is(p.Err, tt.want) {
				t.Fatalf("progress = %+v, want failed with %v", p, tt.want)
			}
			if left := tmpFiles(t, e.root); len(left) != 0 {
				t.Errorf("temporary files left: %v", left)
			}
			if st, _ := e.store.Stats(context.Background()); st.Images != 0 {
				t.Errorf("library has %d images", st.Images)
			}
		})
	}
}

func TestCancelStopsDownloadAndRemovesTemporaryFile(t *testing.T) {
	started := make(chan struct{})
	e := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.Write(make([]byte, 1000))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done() // never finish
	})
	img := e.image("abc", "/a.jpg")

	key, err := e.mgr.Enqueue(context.Background(), img)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	e.mgr.Cancel(key)

	if p := e.rec.wait(t); p.State != Canceled || p.Err != nil {
		t.Errorf("progress = %+v, want canceled", p)
	}
	if left := tmpFiles(t, e.root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestSameImageIsNotDownloadedTwiceAtOnce(t *testing.T) {
	var requests atomic.Int32
	release := make(chan struct{})
	data := picture(t, 0, 90)
	e := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		<-release
		w.Write(data)
	})
	img := e.image("abc", "/a.jpg")

	k1, err1 := e.mgr.Enqueue(context.Background(), img)
	k2, err2 := e.mgr.Enqueue(context.Background(), img)
	close(release)
	e.rec.wait(t)

	if err1 != nil || err2 != nil || k1 != k2 {
		t.Errorf("keys %q, %q; errors %v, %v", k1, k2, err1, err2)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
}

func TestAtMostParallelDownloadsRun(t *testing.T) {
	var running, peak atomic.Int32
	data := picture(t, 0, 90)
	e := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
		w.Write(data)
	})

	const n = Parallel + 3
	for i := range n {
		if _, err := e.mgr.Enqueue(context.Background(), e.image("img"+strconv.Itoa(i), "/a.jpg")); err != nil {
			t.Fatal(err)
		}
	}
	for range n {
		e.rec.wait(t)
	}
	if p := peak.Load(); p > Parallel {
		t.Errorf("%d downloads ran at once, want at most %d", p, Parallel)
	}
}

func TestExistingFilesAreNeverReplaced(t *testing.T) {
	data := picture(t, 0, 90)
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": data}))
	// A file left over from an earlier run, not in the database.
	orphan := filepath.Join(e.root, "images", "wallhaven", "abc.jpg")
	if err := os.MkdirAll(filepath.Dir(orphan), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, []byte("keep me"), filePerm); err != nil {
		t.Fatal(err)
	}

	p := e.download(t, e.image("abc", "/a.jpg"))

	entry, err := e.store.Get(context.Background(), p.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.FilePath != filepath.Join("images", "wallhaven", "abc-2.jpg") {
		t.Errorf("FilePath = %q, want the -2 name", entry.FilePath)
	}
	if content, _ := os.ReadFile(orphan); string(content) != "keep me" {
		t.Errorf("existing file was replaced: %q", content)
	}
}

func TestUnsafeProviderIDsStayInsideTheLibrary(t *testing.T) {
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": picture(t, 0, 90)}))
	img := e.image("../../../etc/evil", "/a.jpg")

	p := e.download(t, img)
	entry, err := e.store.Get(context.Background(), p.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	full, _ := filepath.Abs(filepath.Join(e.root, entry.FilePath))
	if !strings.HasPrefix(full, filepath.Join(e.root, "images")+string(filepath.Separator)) {
		t.Errorf("file written outside the library: %s", full)
	}
	if entry.Image.ProviderID != "../../../etc/evil" {
		t.Errorf("provider ID changed in the database: %q", entry.Image.ProviderID)
	}
}

func TestNewRemovesOnlyOwnStaleTemporaryFiles(t *testing.T) {
	root := t.TempDir()
	tmp := filepath.Join(root, tmpDir)
	os.MkdirAll(tmp, dirPerm)
	os.WriteFile(filepath.Join(tmp, tmpPrefix+"123"), []byte("partial"), filePerm)
	old := time.Now().Add(-2 * staleAfter)
	os.Chtimes(filepath.Join(tmp, tmpPrefix+"123"), old, old)
	// A download another Kisetsu process is running right now.
	os.WriteFile(filepath.Join(tmp, tmpPrefix+"busy"), []byte("in progress"), filePerm)
	os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("not ours"), filePerm)

	m, err := New(Config{Root: root, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	if left := tmpFiles(t, root); !slices.Equal(left, []string{tmpPrefix + "busy", "notes.txt"}) {
		t.Errorf("temporary directory = %v, want the running download and notes.txt", left)
	}
}

func TestSafeName(t *testing.T) {
	tests := map[string]string{
		"abc123":      "abc123",
		"125216708":   "125216708",
		"a/b":         "a_b",
		"..":          "__",
		"":            "image",
		"雪":           "_",
		"name.with.x": "name_with_x",
		"con":         "con_",
		"Nul":         "Nul_",
		"com1":        "com1_",
		"console":     "console",
	}
	for in, want := range tests {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchedDownloadIsStoredSmallerInTheCache(t *testing.T) {
	data := picture(t, 0, 90) // 640×400
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": data}))
	ctx := context.Background()
	insertSavedSearch(t, e) // the saved search must exist for the foreign key

	img := e.image("fetched1", "/a.jpg")
	if _, err := e.mgr.EnqueueFetched(ctx, img, FetchOptions{SavedSearchID: 1, Fit: imaging.Size{Width: 320, Height: 180}}); err != nil {
		t.Fatal(err)
	}
	p := e.rec.wait(t)
	if p.State != Done {
		t.Fatalf("progress = %+v", p)
	}

	entry, err := e.store.Get(ctx, p.EntryID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Origin != library.Fetched || entry.SavedSearchID != 1 {
		t.Errorf("origin %q, search %d", entry.Origin, entry.SavedSearchID)
	}
	if entry.FilePath != filepath.Join("fetched", "images", "wallhaven", "fetched1.jpg") {
		t.Errorf("FilePath = %q", entry.FilePath)
	}
	// 640×400 (16:10) covering 320×180 is 320×200.
	if entry.Image.Width != 320 || entry.Image.Height != 200 {
		t.Errorf("stored %dx%d, want 320x200", entry.Image.Width, entry.Image.Height)
	}
	stored, err := os.ReadFile(filepath.Join(e.root, "cache", entry.FilePath))
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(stored)) != entry.Image.FileSize || len(stored) >= len(data) {
		t.Errorf("stored %d bytes (recorded %d), original %d", len(stored), entry.Image.FileSize, len(data))
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(stored)); err != nil || cfg.Width != 320 {
		t.Errorf("stored file %+v, %v", cfg, err)
	}
	sum := sha256.Sum256(data)
	if entry.SHA256 != hex.EncodeToString(sum[:]) {
		t.Error("SHA-256 must be the hash of the downloaded original")
	}
	if left := tmpFiles(t, filepath.Join(e.root, "cache", "fetched")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}

	// Downloading it by hand points to Keep instead.
	if _, err := e.mgr.Enqueue(ctx, img); !errors.Is(err, ErrInFetched) {
		t.Errorf("manual download of a fetched image: %v", err)
	}
	// Fetching it again is a plain "already there".
	if _, err := e.mgr.EnqueueFetched(ctx, img, FetchOptions{SavedSearchID: 1}); !errors.Is(err, library.ErrAlreadyInLibrary) {
		t.Errorf("fetching it twice: %v", err)
	}
}

func TestFetchedSmallImageIsKeptAsIs(t *testing.T) {
	data := picture(t, 0, 90) // 640×400, smaller than the screen
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": data}))
	insertSavedSearch(t, e)

	e.mgr.EnqueueFetched(context.Background(), e.image("small", "/a.jpg"), FetchOptions{SavedSearchID: 1, Fit: imaging.Size{Width: 1920, Height: 1080}})
	p := e.rec.wait(t)
	entry, _ := e.store.Get(context.Background(), p.EntryID)
	if entry.Image.Width != 640 || entry.Image.FileSize != int64(len(data)) {
		t.Errorf("small image changed: %dx%d, %d bytes", entry.Image.Width, entry.Image.Height, entry.Image.FileSize)
	}
}

func insertSavedSearch(t *testing.T, e *env) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(e.root, database.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO saved_searches (id, name, query) VALUES (1, 'Test', '{}')`); err != nil {
		t.Fatal(err)
	}
}

func TestFetchNowWaitsForTheResult(t *testing.T) {
	e := newEnv(t, serve(map[string][]byte{"/a.jpg": picture(t, 0, 90)}))
	insertSavedSearch(t, e)
	img := e.image("now", "/a.jpg")

	p := e.mgr.FetchNow(context.Background(), img, FetchOptions{SavedSearchID: 1, Fit: imaging.Size{Width: 320, Height: 180}})
	if p.State != Done || p.EntryID == 0 {
		t.Fatalf("FetchNow = %+v", p)
	}
	if entry, _ := e.store.Get(context.Background(), p.EntryID); entry.Origin != library.Fetched {
		t.Errorf("origin = %q", entry.Origin)
	}
	if again := e.mgr.FetchNow(context.Background(), img, FetchOptions{SavedSearchID: 1}); again.State != Duplicate {
		t.Errorf("second FetchNow = %+v, want duplicate", again)
	}
}

// copyNew is the fallback for file systems without hard links.
func TestCopyNewNeverReplaces(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	os.WriteFile(src, []byte("image"), 0o600)
	if err := copyNew(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "image" {
		t.Errorf("copy = %q", data)
	}
	os.WriteFile(src, []byte("other"), 0o600)
	if err := copyNew(src, dst); !errors.Is(err, os.ErrExist) {
		t.Errorf("err = %v, want ErrExist", err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "image" {
		t.Errorf("existing file replaced: %q", data)
	}
}
