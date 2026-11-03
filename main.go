package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/schedule"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

//go:embed all:frontend/dist
var assets embed.FS

// Window icon, shown where the desktop does not use the installed
// .desktop file (for example a binary started from build/bin).
//
//go:embed build/linux/icons/256.png
var windowIcon []byte

func main() {
	// A command (for example `kisetsu rotate`) runs without a window.
	if len(os.Args) > 1 {
		os.Exit(runCommand(os.Args[1], os.Args[2:]))
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu:", err)
		os.Exit(1)
	}
}

// run opens the Kisetsu window.
func run() error {
	c, err := openCore(context.Background(), "app", os.Stderr)
	if err != nil {
		return err
	}
	defer c.Close()
	log := c.log

	client := httpClient()
	searcher, err := c.searchService(client)
	if err != nil {
		return err
	}

	// The manager reports progress through app, which is created below;
	// no download can start before the app exists.
	var app *App
	downloader, err := c.downloader(client, func(p downloads.Progress) { app.notifyDownload(p) })
	if err != nil {
		return err
	}
	// Runs before the database is closed: deferred calls run in reverse.
	defer downloader.Close()

	timer, err := schedule.New()
	if err != nil {
		return err
	}

	gpuPolicy, gpuErr := parseGPUPolicy(os.Getenv(gpuPolicyEnv))
	if gpuErr != nil {
		log.Warn("using default WebView GPU policy", "err", gpuErr)
	}

	app = NewApp(Deps{
		Log:       log,
		DB:        c.db,
		DBPath:    c.dbPath,
		Search:    searcher,
		Settings:  settings.NewStore(c.db),
		Downloads: downloader,
		Library:   c.lib,
		Files:     c.files,
		Wallpaper: wallpaper.New(),
		Rotation:  c.rotator(c.fetcher(searcher, downloader)),

		Schedule:      timer,
		SavedSearches: searches.NewStore(c.db),
		Fetched:       c.fetchedService(),
		Executable:    timer.Executable,
		RotationLock:  c.rotationLockPath(),
	})

	err = wails.Run(&options.App{
		Title:     appinfo.Name,
		Width:     1200,
		Height:    800,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: c.files.Middleware,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		Linux: &linux.Options{
			WebviewGpuPolicy: gpuPolicy,
			// Matches StartupWMClass in the .desktop file.
			ProgramName: "kisetsu",
			Icon:        windowIcon,
		},
		OnStartup: app.startup,
		// Log the full error, send only the user-safe part to the frontend.
		ErrorFormatter: formatError(log),
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
