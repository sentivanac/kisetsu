package main

import (
	"github.com/godbus/dbus/v5"
)

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
