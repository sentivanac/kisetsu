package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/legacytimer"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// appID names the .desktop file and the icon (build/linux/, APP_ID in the
// Makefile). The window carries the same name, so desktops on Wayland,
// which match a window to its .desktop file by name only, show the icon.
const appID = "io.github.sentivanac.Kisetsu"

//go:embed all:frontend/dist
var assets embed.FS

// Window icon, shown where the desktop does not use the installed
// .desktop file (for example a binary started from build/bin).
//
//go:embed build/linux/icons/256.png
var windowIcon []byte

func main() {
	showAbout := len(os.Args) == 2 && os.Args[1] == aboutArg
	// A command (for example `kisetsu rotate`) runs without a window.
	if len(os.Args) > 1 && !showAbout {
		attachConsole()
		os.Exit(runCommand(os.Args[1], os.Args[2:]))
	}
	if err := run(showAbout); err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu:", err)
		os.Exit(1)
	}
}

// run opens the Kisetsu window, with About shown if showAbout.
func run(showAbout bool) error {
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

	entry, err := autostart.New()
	if err != nil {
		return err
	}
	// The 0.1.0 timer is only looked for and removed.
	var legacy LegacyTimer
	if timer, err := legacytimer.New(); err == nil {
		legacy = timer
	}

	gpuPolicy, gpuErr := parseGPUPolicy(os.Getenv(gpuPolicyEnv))
	if gpuErr != nil {
		log.Warn("using default WebView GPU policy", "err", gpuErr)
	}

	// The window hands images to Variety when it runs; rotation does not.
	setter := wallpaper.New()
	app = NewApp(Deps{
		Log:       log,
		DB:        c.db,
		DBPath:    c.dbPath,
		Search:    searcher,
		Settings:  settings.NewStore(c.db),
		Downloads: downloader,
		Library:   c.lib,
		Files:     c.files,
		Wallpaper: setter,
		Screen:    c.screen(setter),
		Rotation:  windowRotator(c, c.fetcher(searcher, downloader)),

		Agent:         &agentControl{entry: entry, lockPath: c.agentLockPath(), log: log},
		LegacyTimer:   legacy,
		TrayHost:      trayHostPresent,
		SavedSearches: searches.NewStore(c.db),
		Fetched:       c.fetchedService(),
		Executable:    entry.Executable,
		RotationLock:  c.rotationLockPath(),
		ShowAbout:     showAbout,
		OwnDirs:       c.ownDirs(),
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
			// The Wayland app_id and X11 WM_CLASS: desktops find the
			// window's icon and menu entry by it (ADR 0032).
			ProgramName: appID,
			Icon:        windowIcon,
		},
		Windows: &windows.Options{
			// WebView2 keeps its cache under %LocalAppData%\Kisetsu, not
			// in the roaming profile (Wails' default); the uninstaller
			// removes it.
			WebviewUserDataPath: filepath.Join(c.paths.Cache, "webview"),
		},
		OnStartup: app.startup,
		// One window per user: starting Kisetsu again (from the menu or the
		// tray) brings the open window to the front.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               appID,
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
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

// windowRotator changes the wallpaper for the window: new images for a
// saved search are fetched after a change, in the background, so "Next"
// does not wait for them (ADR 0054).
func windowRotator(c *core, f *fetched.Fetcher) *rotation.Rotator {
	r := c.rotator(f, c.rotationScreen())
	r.TopUpInBackground = true
	return r
}
