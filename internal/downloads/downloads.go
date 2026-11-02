// Package downloads fetches images into the local library.
//
// A download goes through these steps; a failure or cancellation at any
// step removes the temporary file, so no partial file is ever left behind:
//
//  1. wait for a free slot (at most Parallel downloads at once);
//  2. stream the file into tmp/ with a size limit;
//  3. validate it and derive thumbnail and hashes (package imaging);
//  4. skip exact duplicates, note visually similar images;
//  5. move the file and thumbnail into images/ and thumbs/ (never
//     overwriting an existing file) and record it in the library.
package downloads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
	"github.com/sentivanac/kisetsu/internal/library"
)

const (
	// Parallel is the number of downloads that run at the same time.
	Parallel = 3
	// MaxFileSize rejects files larger than this many bytes.
	MaxFileSize = 100 << 20

	// Directories inside the library root.
	ImagesDir = "images"
	ThumbsDir = "thumbs"
	tmpDir    = "tmp"
	tmpPrefix = "dl-"

	progressInterval = 100 * time.Millisecond
	filePerm         = 0o600
	dirPerm          = 0o700
)

var (
	// ErrFileTooLarge means the file exceeds MaxFileSize.
	ErrFileTooLarge = errors.New("file is too large")
	// ErrHTTP means the server did not return the file.
	ErrHTTP = errors.New("download failed")
	// ErrInFetched means the image was already fetched automatically; the
	// user can keep it instead of downloading it again.
	ErrInFetched = errors.New("image is among the fetched images")
)

// FetchOptions make a download a fetched image (ADR 0028): stored in the
// fetched root (the cache), reduced to cover the screen, and linked to the
// saved search it came from.
type FetchOptions struct {
	SavedSearchID int64
	// Fit is the size the stored image must cover; larger images are
	// reduced, smaller ones kept as they are.
	Fit imaging.Size
}

// State is the stage of a download.
type State string

const (
	Queued      State = "queued"
	Downloading State = "downloading"
	Processing  State = "processing"
	Done        State = "done"
	// Duplicate means an identical file was already in the library; nothing
	// new was stored. Progress.EntryID is the existing entry.
	Duplicate State = "duplicate"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

// Finished reports whether s is a final state.
func (s State) Finished() bool {
	return s == Done || s == Duplicate || s == Failed || s == Canceled
}

// Progress reports the state of one download.
type Progress struct {
	Key      string // provider "/" provider ID, as in the frontend
	State    State
	Received int64 // bytes downloaded so far
	Total    int64 // expected size; 0 when unknown
	// EntryID is the library entry when State is Done or Duplicate.
	EntryID int64
	// SimilarIDs lists library entries that look the same (Done only).
	SimilarIDs []int64
	// Err is set when State is Failed.
	Err error
}

// Config holds the dependencies of a Manager.
type Config struct {
	Client    *http.Client
	UserAgent string
	Root      string // library root: images/, thumbs/ and tmp/ live here
	// FetchedRoot holds fetched images in the same layout; stored paths of
	// fetched images start with "fetched/". Needed only for EnqueueFetched.
	FetchedRoot string
	Store       *library.Store
	Log         *slog.Logger
	// Notify receives progress updates. It is called from download
	// goroutines and must not block for long.
	Notify func(Progress)
}

// Manager runs downloads. It is safe for concurrent use.
type Manager struct {
	cfg    Config
	slots  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu   sync.Mutex
	jobs map[string]context.CancelFunc // running downloads by key
}

// New creates a Manager and removes temporary files left by an earlier run.
func New(cfg Config) (*Manager, error) {
	roots := []string{cfg.Root}
	if cfg.FetchedRoot != "" {
		roots = append(roots, cfg.FetchedRoot)
	}
	for _, root := range roots {
		for _, dir := range []string{ImagesDir, ThumbsDir, tmpDir} {
			if err := os.MkdirAll(filepath.Join(root, dir), dirPerm); err != nil {
				return nil, fmt.Errorf("downloads: %w", err)
			}
		}
		removeStaleTemp(filepath.Join(root, tmpDir), cfg.Log)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		cfg:    cfg,
		slots:  make(chan struct{}, Parallel),
		ctx:    ctx,
		cancel: cancel,
		jobs:   map[string]context.CancelFunc{},
	}, nil
}

// removeStaleTemp deletes only Kisetsu's own temporary files.
func removeStaleTemp(dir string, log *slog.Logger) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasPrefix(e.Name(), tmpPrefix) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			log.Warn("cannot remove stale temporary file", "file", e.Name(), "err", err)
		}
	}
}

// Key identifies an image across providers.
func Key(img images.Image) string {
	return img.Provider + "/" + img.ProviderID
}

// Enqueue starts downloading img into the library and returns its key. An
// image that is already being downloaded is not started twice. An image
// that is already in the library fails with library.ErrAlreadyInLibrary,
// one that was fetched automatically with ErrInFetched.
func (m *Manager) Enqueue(ctx context.Context, img images.Image) (string, error) {
	return m.enqueue(ctx, img, nil)
}

// EnqueueFetched starts downloading img as a fetched image.
func (m *Manager) EnqueueFetched(ctx context.Context, img images.Image, opts FetchOptions) (string, error) {
	if m.cfg.FetchedRoot == "" {
		return "", errors.New("downloads: no fetched root configured")
	}
	return m.enqueue(ctx, img, &opts)
}

func (m *Manager) enqueue(ctx context.Context, img images.Image, fetch *FetchOptions) (string, error) {
	if err := img.Validate(); err != nil {
		return "", err
	}
	key := Key(img)
	if e, found, err := m.cfg.Store.FindBySource(ctx, img.Provider, img.ProviderID); err != nil {
		return "", err
	} else if found && e.Origin == library.Fetched && fetch == nil {
		return "", fmt.Errorf("%w: entry %d", ErrInFetched, e.ID)
	} else if found {
		return "", fmt.Errorf("%w: entry %d", library.ErrAlreadyInLibrary, e.ID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil {
		return "", errors.New("downloads: manager is closed")
	}
	if _, running := m.jobs[key]; running {
		return key, nil
	}
	jobCtx, cancel := context.WithCancel(m.ctx)
	m.jobs[key] = cancel

	m.notify(Progress{Key: key, State: Queued, Total: img.FileSize})
	m.wg.Go(func() {
		defer cancel()
		p := m.run(jobCtx, key, img, fetch)
		m.mu.Lock()
		delete(m.jobs, key)
		m.mu.Unlock()
		m.notify(p)
	})
	return key, nil
}

// Cancel stops a download. Canceling an unknown or finished download does
// nothing.
func (m *Manager) Cancel(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.jobs[key]; ok {
		cancel()
	}
}

// Close cancels all downloads and waits for them to stop.
func (m *Manager) Close() {
	m.mu.Lock()
	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) notify(p Progress) {
	if m.cfg.Notify != nil {
		m.cfg.Notify(p)
	}
}

// run performs one download and returns its final progress.
func (m *Manager) run(ctx context.Context, key string, img images.Image, fetch *FetchOptions) Progress {
	final := func(state State, err error) Progress {
		if errors.Is(err, context.Canceled) {
			state, err = Canceled, nil
		}
		if err != nil {
			m.cfg.Log.Warn("download failed", "key", key, "err", err)
		}
		return Progress{Key: key, State: state, Err: err}
	}

	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		return final(Canceled, ctx.Err())
	}

	root := m.cfg.Root
	var opts imaging.Options
	if fetch != nil {
		root, opts.Fit = m.cfg.FetchedRoot, fetch.Fit
	}
	tmp, err := m.fetch(ctx, key, img, root)
	if err != nil {
		return final(Failed, err)
	}
	defer os.Remove(tmp.Name()) // no-op once moved into the library
	defer tmp.Close()

	m.notify(Progress{Key: key, State: Processing})
	result, err := imaging.Process(tmp, opts)
	if err != nil {
		return final(Failed, err)
	}
	if err := ctx.Err(); err != nil {
		return final(Canceled, err)
	}

	if existing, found, err := m.cfg.Store.FindBySHA256(ctx, result.SHA256); err != nil {
		return final(Failed, err)
	} else if found {
		m.cfg.Log.Info("download is an exact duplicate", "key", key, "entry", existing.ID)
		return Progress{Key: key, State: Duplicate, EntryID: existing.ID}
	}

	similar, err := m.similar(ctx, result.DHash)
	if err != nil {
		return final(Failed, err)
	}
	file, err := m.storedFile(tmp, result, root)
	if err != nil {
		return final(Failed, err)
	}
	defer os.Remove(file.path) // no-op once moved into the library

	id, err := m.store(ctx, img, file, result, root, fetch)
	if errors.Is(err, library.ErrDuplicateFile) {
		// Another download with the same content finished first.
		if existing, found, ferr := m.cfg.Store.FindBySHA256(ctx, result.SHA256); ferr == nil && found {
			return Progress{Key: key, State: Duplicate, EntryID: existing.ID}
		}
	}
	if err != nil {
		return final(Failed, err)
	}
	m.cfg.Log.Info("download done", "key", key, "entry", id, "bytes", file.size,
		"fetched", fetch != nil, "fitted", result.Fitted != nil, "similar", len(similar))
	return Progress{Key: key, State: Done, EntryID: id, SimilarIDs: similar}
}

// fetch streams img into a new temporary file in root, reporting progress.
func (m *Manager) fetch(ctx context.Context, key string, img images.Image, root string) (*os.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, img.DownloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", m.cfg.UserAgent)

	resp, err := m.cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrHTTP, resp.StatusCode)
	}
	if resp.ContentLength > MaxFileSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrFileTooLarge, resp.ContentLength)
	}
	total := max(resp.ContentLength, 0)

	tmp, err := os.CreateTemp(filepath.Join(root, tmpDir), tmpPrefix+"*")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	pw := &progressWriter{key: key, total: total, notify: m.notify}
	// Read one byte past the limit to detect oversized bodies without a
	// Content-Length.
	n, err := io.Copy(io.MultiWriter(tmp, pw), io.LimitReader(resp.Body, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if n > MaxFileSize {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrFileTooLarge, MaxFileSize)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return tmp, nil
}

// similar returns library entries whose dHash is close to h.
func (m *Manager) similar(ctx context.Context, h uint64) ([]int64, error) {
	refs, err := m.cfg.Store.DHashes(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, ref := range refs {
		if imaging.Similar(h, ref.DHash) {
			ids = append(ids, ref.ID)
		}
	}
	return ids, nil
}

// stored describes the file that will be kept: the download itself, or
// the copy reduced to screen size.
type stored struct {
	path          string // temporary file
	size          int64
	width, height int
	mime          string
}

func (m *Manager) storedFile(tmp *os.File, res imaging.Result, root string) (stored, error) {
	if res.Fitted == nil {
		size, err := tmp.Seek(0, io.SeekEnd)
		return stored{path: tmp.Name(), size: size, width: res.Width, height: res.Height, mime: res.MIMEType}, err
	}
	path, err := writeTemp(root, res.Fitted)
	return stored{
		path: path, size: int64(len(res.Fitted)),
		width: res.FittedWidth, height: res.FittedHeight, mime: "image/jpeg",
	}, err
}

func writeTemp(root string, data []byte) (string, error) {
	f, err := os.CreateTemp(filepath.Join(root, tmpDir), tmpPrefix+"*")
	if err != nil {
		return "", err
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Close()); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// store moves the file and thumbnail into root and records them. On
// failure it removes only the files it created.
func (m *Manager) store(ctx context.Context, img images.Image, file stored, res imaging.Result,
	root string, fetch *FetchOptions) (int64, error) {
	name := safeName(img.ProviderID)
	provider := safeName(img.Provider)

	filePath, err := place(root, file.path, filepath.Join(ImagesDir, provider), name, extension(file.mime))
	if err != nil {
		return 0, err
	}
	created := []string{filePath}
	cleanup := func() {
		for _, p := range created {
			os.Remove(filepath.Join(root, p))
		}
	}

	thumbTmp, err := os.CreateTemp(filepath.Join(root, tmpDir), tmpPrefix+"*")
	if err != nil {
		cleanup()
		return 0, err
	}
	_, werr := thumbTmp.Write(res.Thumbnail)
	cerr := thumbTmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(thumbTmp.Name())
		cleanup()
		return 0, err
	}
	thumbPath, err := place(root, thumbTmp.Name(), filepath.Join(ThumbsDir, provider), name, ".jpg")
	if err != nil {
		os.Remove(thumbTmp.Name())
		cleanup()
		return 0, err
	}
	created = append(created, thumbPath)

	// The stored file's dimensions and format are the truth; the
	// provider's attribution is kept as it was.
	meta := img
	meta.Width, meta.Height, meta.MIMEType, meta.FileSize = file.width, file.height, file.mime, file.size
	meta.ThumbnailURL, meta.PreviewURL = "", ""

	entry := library.Entry{
		Image:         meta,
		FilePath:      filePath,
		ThumbnailPath: thumbPath,
		SHA256:        res.SHA256,
		DHash:         res.DHash,
	}
	if fetch != nil {
		entry.Origin, entry.SavedSearchID = library.Fetched, fetch.SavedSearchID
		entry.FilePath = filepath.Join("fetched", filePath)
		entry.ThumbnailPath = filepath.Join("fetched", thumbPath)
	}
	id, err := m.cfg.Store.Add(ctx, entry)
	if err != nil {
		cleanup()
		return 0, err
	}
	return id, nil
}

// place moves src to dir/name+ext inside root and returns the path
// relative to root. An existing file is never replaced: the name gets a
// numeric suffix instead.
func place(root, src, dir, name, ext string) (string, error) {
	if err := os.MkdirAll(filepath.Join(root, dir), dirPerm); err != nil {
		return "", err
	}
	if err := os.Chmod(src, filePerm); err != nil {
		return "", err
	}
	for i := 1; i <= 100; i++ {
		file := name + ext
		if i > 1 {
			file = fmt.Sprintf("%s-%d%s", name, i, ext)
		}
		rel := filepath.Join(dir, file)
		dst := filepath.Join(root, rel)
		// Link fails if dst exists, unlike Rename, so nothing is replaced.
		err := os.Link(src, dst)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return rel, os.Remove(src)
	}
	return "", fmt.Errorf("no free file name for %s%s in %s", name, ext, dir)
}

// safeName turns a provider value into a file name: letters, digits, "-"
// and "_" only, so it can never contain a path separator or "..".
func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 || b.Len() > 100 {
		return "image"
	}
	return b.String()
}

func extension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ".jpg"
}

// progressWriter reports download progress at most every progressInterval.
type progressWriter struct {
	key      string
	total    int64
	received int64
	last     time.Time
	notify   func(Progress)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.received += int64(len(p))
	if now := time.Now(); now.Sub(w.last) >= progressInterval {
		w.last = now
		w.notify(Progress{Key: w.key, State: Downloading, Received: w.received, Total: w.total})
	}
	return len(p), nil
}
