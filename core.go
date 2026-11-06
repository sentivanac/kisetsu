package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/config"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/logging"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/providers"
	"github.com/sentivanac/kisetsu/internal/providers/commons"
	"github.com/sentivanac/kisetsu/internal/providers/wallhaven"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// core is what both the window and the commands need: directories, the
// log, the database and the library files.
type core struct {
	paths  config.Paths
	log    *slog.Logger
	db     *sql.DB
	dbPath string
	lib    *library.Store
	files  *libfiles.Server
	// fetchedDir holds fetched images and thumbnails, in the cache.
	fetchedDir string

	closers []io.Closer
}

// openCore prepares the shared parts. component names the caller in the
// log ("app" or a command), because the window and the timer write to the
// same log file. logMirror receives a copy of the log (stderr for the
// window; nil for commands, which print their own short messages).
func openCore(ctx context.Context, component string, logMirror io.Writer) (*core, error) {
	c := &core{}
	ok := false
	defer func() {
		if !ok {
			c.Close()
		}
	}()

	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	if err := paths.Ensure(); err != nil {
		return nil, err
	}
	c.paths = paths

	level, levelErr := logging.ParseLevel(os.Getenv(logging.LevelEnv))
	log, logFile, err := logging.Open(paths.State, level, logMirror)
	if err != nil {
		return nil, err
	}
	c.closers = append(c.closers, logFile)
	c.log = log.With("component", component)
	if levelErr != nil {
		c.log.Warn("using default log level", "err", levelErr)
	}

	info := appinfo.Get()
	c.log.Info("starting",
		"version", info.Version,
		"go", info.GoVersion,
		"platform", info.Platform,
		"config_dir", paths.Config,
		"data_dir", paths.Data,
		"cache_dir", paths.Cache,
		"state_dir", paths.State,
	)

	c.dbPath = filepath.Join(paths.Data, database.FileName)
	db, err := database.Open(ctx, c.dbPath)
	if err != nil {
		c.log.Error("cannot open database", "path", c.dbPath, "err", err)
		return nil, err
	}
	c.closers = append(c.closers, db)
	c.db = db
	schema, err := database.SchemaVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	c.log.Info("database ready", "path", c.dbPath, "schema_version", schema)

	// The library directories may not exist yet when a command runs before
	// the first download. Fetched images live in the cache (ADR 0028).
	c.fetchedDir = filepath.Join(paths.Cache, "fetched")
	for _, dir := range []string{
		filepath.Join(paths.Data, downloads.ImagesDir),
		filepath.Join(paths.Data, downloads.ThumbsDir),
		filepath.Join(c.fetchedDir, downloads.ImagesDir),
		filepath.Join(c.fetchedDir, downloads.ThumbsDir),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create library directory: %w", err)
		}
	}
	files, err := libfiles.New(paths.Data, c.fetchedDir)
	if err != nil {
		return nil, err
	}
	c.closers = append(c.closers, files)
	c.files = files
	c.lib = library.NewStore(db)

	ok = true
	return c, nil
}

// Close releases everything in reverse order of opening.
func (c *core) Close() {
	for i := len(c.closers) - 1; i >= 0; i-- {
		c.closers[i].Close()
	}
	c.closers = nil
}

// rotator builds the rotation service. It sets wallpapers directly with
// gsettings, never through Variety: the user chose that Kisetsu rotates.
// fetcher, if not nil, tops up saved searches that run low.
func (c *core) rotator(fetcher *fetched.Fetcher) *rotation.Rotator {
	setter := wallpaper.New()
	setter.Direct = true
	r := &rotation.Rotator{
		Library:  c.lib,
		Settings: settings.NewStore(c.db),
		History:  rotation.NewHistory(c.db),
		Files:    c.files,
		Screen:   c.screen(setter),
		Log:      c.log,
	}
	if fetcher != nil {
		r.TopUp = func(ctx context.Context, id int64, want int, protect []int64) (int, error) {
			res, err := fetcher.Fetch(ctx, id, want, protect...)
			return res.Fetched, err
		}
	}
	return r
}

// screen builds the service that shows library images through setter,
// drawing text on them when the user turned that on (ADR 0047).
func (c *core) screen(setter onscreen.Setter) *onscreen.Service {
	return &onscreen.Service{
		Settings:     settings.NewStore(c.db),
		Setter:       setter,
		Dir:          filepath.Join(c.paths.Cache, "overlay"),
		ProviderName: providerName,
		Log:          c.log,
	}
}

// imageSources returns the image sources, in the order they appear in the UI.
func imageSources(client *http.Client) []providers.Provider {
	return []providers.Provider{
		wallhaven.New(client, appinfo.UserAgent()),
		commons.New(client, appinfo.UserAgent()),
	}
}

// providerName returns a source's display name, or its ID if unknown.
func providerName(id string) string {
	for _, p := range imageSources(nil) {
		if p.ID() == id {
			return p.Name()
		}
	}
	return id
}

// rotationLockPath is shared by the timer, the window and manual commands.
func (c *core) rotationLockPath() string {
	return filepath.Join(c.paths.State, "rotate.lock")
}

// fetchedService builds the service that keeps and prunes fetched images.
func (c *core) fetchedService() *fetched.Service {
	return &fetched.Service{Library: c.lib, Files: c.files, Log: c.log}
}

// searchService registers the providers. Every search leaves out the tags the user hid.
func (c *core) searchService(client *http.Client) (*search.Service, error) {
	s, err := search.New(c.log, imageSources(client)...)
	if err != nil {
		return nil, err
	}
	s.HiddenTags = settings.NewStore(c.db).HiddenTags
	return s, nil
}

// downloader builds a download manager for the library and fetched images.
// notify may be nil (commands have no window to report to).
func (c *core) downloader(client *http.Client, notify func(downloads.Progress)) (*downloads.Manager, error) {
	return downloads.New(downloads.Config{
		Client:      client,
		UserAgent:   appinfo.UserAgent(),
		Root:        c.paths.Data,
		FetchedRoot: c.fetchedDir,
		Store:       c.lib,
		Log:         c.log,
		Notify:      notify,
	})
}

// fetcher builds the service that downloads new images for saved
// searches. It needs no window: downloads wait for their result.
func (c *core) fetcher(searcher *search.Service, dl fetched.Downloader) *fetched.Fetcher {
	return &fetched.Fetcher{
		DB:         c.db,
		Searches:   searches.NewStore(c.db),
		Settings:   settings.NewStore(c.db),
		Search:     searcher,
		Downloader: dl,
		Service:    c.fetchedService(),
		Log:        c.log,
	}
}

// httpClient is shared by providers and downloads. The timeout is a
// safety net; searches and downloads have their own limits.
func httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}
