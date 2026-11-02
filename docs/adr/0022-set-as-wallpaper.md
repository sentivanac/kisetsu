# ADR 0022: Set as wallpaper

- **Status:** accepted
- **Date:** 2026-11-02
- Last step of Phase 4. Rotation and other desktops follow in Phase 6.

## Context

The development machine runs **MATE** (X11), with GNOME 42 installed, and
**Variety** manages the wallpaper. User decision (2026-10-30): when Variety
runs, go through it, so it keeps rotating and applying its effects (for
example its clock).

## Decision

`internal/wallpaper.Setter.Set(ctx, absolutePath)`:

1. **Variety running?** Checked by reading `/proc/<pid>/comm == "variety"`
   for processes owned by the current user — not by calling `variety`,
   because `variety --get`/`--set` start a new Variety when none runs. If
   it runs: `variety --set <path>`.
2. Otherwise the desktop of the session the user is logged into
   (since 2026-11-02 found via `loginctl`, see ADR 0031; before that
   `XDG_CURRENT_DESKTOP`, colon-separated, e.g. `ubuntu:GNOME`):
   - **MATE**: `org.mate.background picture-filename` (a plain path);
   - **GNOME**: `org.gnome.desktop.background picture-uri` and
     `picture-uri-dark` (GNOME 42+ has a separate dark picture) as a
     percent-encoded `file://` URI.
   - Anything else: `ErrUnsupportedDesktop`, no command is run.
3. `gsettings` values are written as GVariant string literals (quotes and
   backslashes escaped) and **read back** after setting; a value that did
   not change is `ErrNotApplied`.

Commands are run directly with `exec`, never through a shell, so a path
cannot inject commands. `picture-options` (zoom, scaled, …) is left as
the user configured it.

### App

`SetWallpaper(imageID)` resolves the entry's file through
`libfiles.Server.OriginalPath`, which only returns regular files inside
`images/` (os.Root, no thumbnails, no symbolic links), so external
programs only ever receive library originals. A file removed outside
Kisetsu is reported instead of being passed on. Messages: "Set as your
wallpaper." or, through Variety, that Variety may replace it at its next
change. Unsupported desktops get code `unsupported`.

The library preview has a **Set as wallpaper** button with the result
shown below it.

### Tests

Tests never touch the real desktop: `Runner` is replaced by a fake that
records commands and acts as a gsettings store, `/proc` by a temporary
directory, and the App tests use a fake `WallpaperSetter`. Detection was
checked once against the real system (read-only): Variety running, MATE.

## Limitations

- Variety may replace the image at its next scheduled change; pausing it
  was deliberately not chosen.
- Only MATE and GNOME (and Variety); KDE, Cinnamon, XFCE and others come in
  Phase 6.
- The same image is used on all monitors.
