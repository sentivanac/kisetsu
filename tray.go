package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"fyne.io/systray"

	"github.com/sentivanac/kisetsu/internal/rotation"
)

// trayIcon is the application icon in color, like in the menu (ADR 0042).
//
//go:embed build/linux/icons/64.png
var trayIcon []byte

// writeTrayIcon puts the icon into dir as <app ID>.png, where panels find
// it through IconThemePath.
func writeTrayIcon(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, appID+".png")
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, trayIcon) {
		return nil
	}
	tmp, err := os.CreateTemp(dir, ".tray-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.Write(trayIcon); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// traySync is how often the menu picks up changes made in the window.
const traySync = 5 * time.Second

// trayActions is what the tray menu does; runAgent connects them.
type trayActions struct {
	next        func(ctx context.Context) error
	rotating    func(ctx context.Context) (bool, error)
	setRotating func(ctx context.Context, on bool) error
	open        func() error
}

// runTray shows the icon and its menu until ctx ends or the user chooses
// Quit, which calls quit. It blocks, and runs on the main goroutine (the
// Windows tray will need that).
//
// Without a tray host (GNOME without the AppIndicator extension), the
// icon is simply not shown; systray registers again when a host appears.
func runTray(ctx context.Context, log *slog.Logger, iconDir string, act trayActions, quit func()) {
	// The icon goes out twice: by name, with a directory that holds it, for
	// panels that only look icons up by name (Ayatana indicators, the
	// default on Ubuntu MATE, show "image missing" otherwise), and as image
	// data for the others. The installed package also puts it in the
	// system icon theme. Set before Run, so the first query finds it.
	if err := writeTrayIcon(iconDir); err != nil {
		log.Warn("tray icon file not written; panels that need it show a placeholder", "err", err)
	} else {
		systray.SetIconThemePath(iconDir)
	}
	systray.SetIconName(appID)
	systray.SetIcon(trayIcon)
	systray.SetTitle("Kisetsu")
	systray.SetTooltip("Kisetsu")

	systray.Run(func() {
		next := systray.AddMenuItem("Next wallpaper", "Change the wallpaper now")
		rotate := systray.AddMenuItemCheckbox("Change automatically", "Turn automatic rotation on or off", false)
		systray.AddSeparator()
		open := systray.AddMenuItem("Open Kisetsu", "Open the Kisetsu window")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Hide the icon and stop changing the wallpaper until the next login")

		sync := func() {
			on, err := act.rotating(ctx)
			if err != nil {
				return
			}
			if on {
				rotate.Check()
			} else {
				rotate.Uncheck()
			}
		}
		sync()

		go func() {
			ticker := time.NewTicker(traySync)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					systray.Quit()
					return
				case <-ticker.C:
					sync()
				case <-next.ClickedCh:
					next.Disable()
					go func() {
						defer next.Enable()
						if err := act.next(ctx); err != nil && !errors.Is(err, rotation.ErrLocked) {
							log.Warn("wallpaper change from the tray failed", "err", err)
						}
					}()
				case <-rotate.ClickedCh:
					if err := act.setRotating(ctx, !rotate.Checked()); err != nil {
						log.Warn("rotation not changed from the tray", "err", err)
					}
					sync()
				case <-open.ClickedCh:
					if err := act.open(); err != nil {
						log.Warn("window not opened from the tray", "err", err)
					}
				case <-quitItem.ClickedCh:
					log.Info("quit from the tray")
					quit()
				}
			}
		}()
	}, func() {})
}
