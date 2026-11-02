# ADR 0026: The Wallpaper tab

- **Status:** accepted
- **Date:** 2026-11-02
- Fourth step of Phase 6; brings together ADRs 0023–0025.

## Decision

### Bound methods (`app_rotation.go`)

- `GetRotation()` returns the settings, the offered intervals, the timer
  status (installed, active, next run, outdated, or a read error), the last
  change from the history and whether Variety is running.
- `SaveRotation(settings)` validates, then **installs, updates or removes
  the timer first, and saves the settings after**. If installing the timer
  fails, rotation is saved as off: the settings never claim a timer that
  does not exist. Timer errors are specific: a foreign unit file with
  Kisetsu's name, or a temporary build.
- `RepairRotationTimer()` reinstalls the timer for the running binary when
  the installed one starts another copy (moved or rebuilt elsewhere).
- `NextWallpaper()` changes the wallpaper now with the same rotation rules
  and the same lock as the timer; while another change runs it asks to try
  again.

`App` receives the scheduler, its own executable path and the lock path
through `Deps`; tests use a fake scheduler and never call systemd.

### Manual changes restart the countdown

The timer counts from its own start and its own runs, so originally a
manual change did not move it: "Last changed 5 min ago" could sit next to
"Next change in 14 min" with a 15-minute interval (noticed by the user on
2026-11-02). Now, when rotation is on, every manual change — "Next
wallpaper", "Set as wallpaper" and `kisetsu rotate` without `--scheduled` —
restarts the timer (`schedule.Manager.Restart`), so the next automatic
change always comes one full interval after the last change, as in
Variety. A failed restart is only logged; the wallpaper has already
changed.

### UI

A third tab, **Wallpaper**:

- **Next wallpaper** and when Kisetsu last changed it ("5 min ago").
- **Automatic rotation**: on/off, how often, images from (whole library or a
  collection, with counts; a deleted collection is shown as such), and the
  order. Every change applies immediately — there is no Save button —
  because each one maps to one timer operation.
- Status: "Next change at 09:21 (in 34 min)", refreshed every 30 seconds
  while the tab is visible; an error if rotation is on but the timer is not
  running; a note that rotation runs while the window is closed and stops
  at logout.
- **Warnings**: the timer runs another copy of Kisetsu ("Use this copy"),
  and Variety is running and would change the wallpaper too, with how to
  turn its automatic change off.

### Tests

App tests cover enabling and disabling, a failing timer leaving rotation
off, invalid settings, outdated timer and repair, "Next wallpaper" with an
empty library and while locked, and the Variety warning. Frontend helpers
(`src/lib/rotation.ts`) format intervals and relative times; a test caught
`Math.round(-0.3)` being `-0`, for which `-0 >= 0` is true.

## Limitations

- The same image is used on all monitors.
- No "previous wallpaper" yet, although the history makes it possible.
