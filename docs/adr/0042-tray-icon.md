# ADR 0042: Tray icon

## Status

Accepted (2026-11-04).

## Context

The background agent (ADR 0041) runs while the user is logged in, so it
can also show an icon in the system tray, as Variety does. Wails v2 has
no tray. Desktops show tray icons through StatusNotifierItem over D-Bus
(KDE, Xfce, MATE, Cinnamon, Ubuntu's GNOME); plain GNOME needs the
"AppIndicator and KStatusNotifierItem Support" extension.

## Decision

- **Library:** `fyne.io/systray` v1.12.2 (Apache-2.0). On Linux it is pure
  Go over D-Bus (`godbus/dbus`, already used by Wails); C code exists only
  for macOS. Without a tray host it logs that registering failed and
  registers again when a host appears. Its Windows part serves 0.3.0.
- **Setting:** "Show Kisetsu in the system tray" in the Wallpaper tab
  (`agent` settings key), independent of rotation. The agent runs while
  rotation **or** the tray icon is on; the autostart entry follows the
  same rule.
- **Menu:** Next wallpaper (under the rotation lock; the next scheduled
  change counts from it), Change automatically (a check mark for
  rotation on or off), Open Kisetsu, Quit. Quit ends the agent until the
  next login or until it is started from the Wallpaper tab; the
  autostart entry stays. The check mark follows changes made in the
  window within 5 seconds.
- **Icon:** the application icon in color (64 px), as in the menu; chosen
  by the user over a monochrome silhouette, which panels of unknown
  brightness can make hard to see.
- **Showing or hiding the icon** restarts the agent. The agent writes its
  PID into `agent.lock`; the window sends it SIGTERM (only while the lock
  is held, so the PID is current) and starts a new one.
- **Tray runs on the main goroutine**, the schedule beside it; either
  ending ends the other. Windows will require the main thread.
- **One window:** Wails' single-instance lock. Starting Kisetsu again
  (menu, tray) brings the open window to the front; the second process
  exits.
- **End of session:** the agent ends when its D-Bus session bus closes.
  Found while testing: without this, an agent whose session was gone
  kept running and logged errors every few seconds.
- **The window tells** when the tray icon is on but the desktop has no
  tray host (no `org.kde.StatusNotifierWatcher` on the session bus).

## Verification

Unit tests for the setting, the "wanted" rule, restarts and a failed
setup; a test that starts and stops a real process holding the agent
lock. In a private D-Bus session (`dbus-run-session`, so the user's panel
never sees it): the agent exports its icon and menu; clicking the items
over D-Bus (`com.canonical.dbusmenu.Event`) changed the wallpaper (a fake
`gsettings`), turned rotation on and off in the database, and Quit ended
the agent; closing the session ended it as well. A second window process
exited and the first one logged it. Seeing the icon on a real panel is
left to the user's desktop.
