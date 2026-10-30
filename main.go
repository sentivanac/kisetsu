package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/config"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/logging"
	"github.com/sentivanac/kisetsu/internal/providers/wallhaven"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/settings"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu:", err)
		os.Exit(1)
	}
}

func run() error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}

	level, levelErr := logging.ParseLevel(os.Getenv(logging.LevelEnv))
	log, logFile, err := logging.Open(paths.State, level)
	if err != nil {
		return err
	}
	defer logFile.Close()

	if levelErr != nil {
		log.Warn("using default log level", "err", levelErr)
	}

	info := appinfo.Get()
	log.Info("starting",
		"version", info.Version,
		"go", info.GoVersion,
		"platform", info.Platform,
		"config_dir", paths.Config,
		"data_dir", paths.Data,
		"cache_dir", paths.Cache,
		"state_dir", paths.State,
	)

	ctx := context.Background()
	dbPath := filepath.Join(paths.Data, database.FileName)
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		log.Error("cannot open database", "path", dbPath, "err", err)
		return err
	}
	defer db.Close()

	schema, err := database.SchemaVersion(ctx, db)
	if err != nil {
		return err
	}
	log.Info("database ready", "path", dbPath, "schema_version", schema)

	// One HTTP client shared by all providers. The timeout is a safety net;
	// searches are also bounded by search.DefaultTimeout.
	httpClient := &http.Client{Timeout: 15 * time.Second}
	searcher, err := search.New(log,
		wallhaven.New(httpClient, appinfo.UserAgent()),
	)
	if err != nil {
		return err
	}

	gpuPolicy, gpuErr := parseGPUPolicy(os.Getenv(gpuPolicyEnv))
	if gpuErr != nil {
		log.Warn("using default WebView GPU policy", "err", gpuErr)
	}

	app := NewApp(Deps{
		Log:      log,
		DB:       db,
		DBPath:   dbPath,
		Search:   searcher,
		Settings: settings.NewStore(db),
	})

	err = wails.Run(&options.App{
		Title:     appinfo.Name,
		Width:     1200,
		Height:    800,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		Linux: &linux.Options{
			WebviewGpuPolicy: gpuPolicy,
			ProgramName:      "kisetsu",
		},
		OnStartup: app.startup,
		// Log the full error, send only the user-safe part to the frontend.
		ErrorFormatter: func(err error) any {
			public := apperr.ToPublic(err)
			if public.Code == apperr.Canceled {
				log.Debug("frontend call canceled", "err", err)
			} else {
				log.Error("frontend call failed", "err", err)
			}
			return public
		},
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Error("application stopped with an error", "err", err)
		return err
	}

	log.Info("stopped")
	return nil
}
