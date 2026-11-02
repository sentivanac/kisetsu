package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/config"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/logging"
	"github.com/sentivanac/kisetsu/internal/rotation"
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
func (c *core) rotator() *rotation.Rotator {
	setter := wallpaper.New()
	setter.Direct = true
	return &rotation.Rotator{
		Library:  c.lib,
		Settings: settings.NewStore(c.db),
		History:  rotation.NewHistory(c.db),
		Files:    c.files,
		Setter:   setter,
		Log:      c.log,
	}
}

// rotationLockPath is shared by the timer, the window and manual commands.
func (c *core) rotationLockPath() string {
	return filepath.Join(c.paths.State, "rotate.lock")
}

// fetchedService builds the service that keeps and prunes fetched images.
func (c *core) fetchedService() *fetched.Service {
	return &fetched.Service{Library: c.lib, Files: c.files, Log: c.log}
}
