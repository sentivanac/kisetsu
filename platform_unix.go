//go:build !windows

package main

import (
	"context"
	"log/slog"
	"os/exec"
	"syscall"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
)

// detach makes cmd run in its own session: it keeps running when the
// window closes, and a Ctrl+C in the terminal that started the window
// does not reach it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// terminate asks the process to end (SIGTERM); the agent stops cleanly.
func terminate(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// watchSession ends the agent when its login session's D-Bus goes away
// (logout), instead of letting it linger. The returned function stops
// watching.
func watchSession(ctx context.Context, cancel context.CancelFunc, log *slog.Logger) (stop func()) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return func() {}
	}
	go func() {
		select {
		case <-conn.Context().Done():
			log.Info("session bus closed; agent exits")
			cancel()
		case <-ctx.Done():
		}
	}()
	return func() { conn.Close() }
}

// trayHostPresent reports whether the session has a tray host, that is a
// StatusNotifierWatcher on D-Bus (KDE, Xfce, MATE, Cinnamon, Ubuntu's
// GNOME; plain GNOME only with the AppIndicator extension).
func trayHostPresent() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()
	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&has)
	return err == nil && has
}

// setTrayIcon sends the icon three ways: by name, with a directory that
// holds it, for panels that only look icons up by name (Ayatana
// indicators, the default on Ubuntu MATE, show "image missing"
// otherwise), and as image data for the others. The installed package
// also puts it in the system icon theme. Called before the tray starts,
// so the first query finds it.
func setTrayIcon(log *slog.Logger, iconDir string) {
	if err := writeTrayIcon(iconDir); err != nil {
		log.Warn("tray icon file not written; panels that need it show a placeholder", "err", err)
	} else {
		systray.SetIconThemePath(iconDir)
	}
	systray.SetIconName(appID)
	systray.SetIcon(trayIcon)
}
