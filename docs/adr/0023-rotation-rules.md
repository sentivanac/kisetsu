# ADR 0023: Wallpaper rotation rules and history

- **Status:** accepted
- **Date:** 2026-11-02
- First step of Phase 6. Scheduling (`kisetsu rotate`, systemd timer) and
  the UI follow.

## Context (user decisions, 2026-11-02)

- Rotation runs from a systemd user timer, also when the window is closed.
- Images come from the library or one collection; automatic fetching
  ("Fetched") comes later.
- **Kisetsu rotates directly** with gsettings, not through Variety. The
  manual "Set as wallpaper" button keeps going through Variety when it runs.

## Decision

### Settings (`settings` key `rotation`)

`enabled`, `interval` (15m, 30m, 1h, 3h, 24h), `collectionId` (0 = whole
library), `order` (random or sequential). Defaults: off, hourly, whole
library, random. Invalid saved values fall back to the defaults, so a bad
preference never stops the wallpaper from changing.

### History (migration `0004_wallpaper_history.sql`)

Every wallpaper Kisetsu sets is recorded with its rotation **cycle** and
reason (`rotation` or `manual`). Rows of deleted images are removed with
them (cascade). The history will also give "previous wallpaper" and
statistics later.

### Choosing the next image (`rotation.Pick`, a pure function)

- **Random**: picks among the images not yet shown by rotation in the
  current cycle. When all have been shown, a new cycle starts. A simpler
  "least recently shown" rule was rejected: after the first cycle it would
  repeat the same order forever.
- **Sequential**: the image added after the current one, wrapping around;
  the oldest when the current image is not in the source.
- **The image on screen is never chosen while any other candidate
  exists** — also when the user set it by hand and it is the only image
  left in the cycle; then a new cycle starts without it. A test found
  that case.

### Unattended behaviour (`rotation.Rotator.Next`)

- A rotation collection that no longer exists falls back to the whole
  library (logged).
- Images whose file is missing are skipped (up to 10), logged.
- An empty source returns `ErrNoImages`; nothing is changed.
- The wallpaper is set by `wallpaper.Setter` with `Direct: true`
  (gsettings; Variety is skipped). `Setter.VarietyRunning` lets the UI warn
  that Variety would change the wallpaper as well.
- `App.SetWallpaper` records manual changes (`RecordManual`), so rotation
  knows what is on screen.

## Tests

`Pick` is tested over 20 simulated cycles: every image exactly once per
cycle, at least 10 different orders, never the same image twice in a row.
`Rotator` is tested against a real database with fake files and a fake
setter: cycles, collection fallback, missing files, empty library, and that
an image the user just set is not chosen again.
