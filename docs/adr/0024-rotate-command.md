# ADR 0024: The `kisetsu rotate` command

- **Status:** accepted
- **Date:** 2026-11-02
- Second step of Phase 6; the systemd timer (next step) runs this command.

## Decision

### One binary, two modes

`main` opens the window when started without arguments and runs a command
otherwise. The window and the commands share `openCore` (`core.go`):
directories, log, database, library store and file server. The library
directories are created there too, so a command works before the first
download.

```
kisetsu                     open the window
kisetsu rotate              change the wallpaper to the next library image
kisetsu rotate --scheduled  the same, but only if rotation is enabled
kisetsu help
```

`--scheduled` is what the timer uses: a timer left behind after rotation
was turned off does nothing. A plain `kisetsu rotate` always rotates,
because the user asked for it explicitly.

### Exit status

| Status | Meaning |
|---|---|
| 0 | wallpaper changed, or nothing to do (rotation off, no images, another rotation running) |
| 1 | failure (database, gsettings, …) — systemd shows the unit as failed |
| 2 | invalid usage |

"No images" is not a failure: it is a state the user can fix, not a fault
to alert about. It is logged as a warning.

### Locking

`rotation.Lock` takes an exclusive, non-blocking `flock` on
`~/.local/state/kisetsu/rotate.lock`, shared by the timer, manual runs and
(next steps) the window's "Next wallpaper" button. The kernel releases a
flock when the process ends, so a crash cannot leave a stale lock — tested
with a helper process that exits while holding it. A run that finds the
lock taken skips and exits with 0.

### Logging

Commands and the window append to the same `kisetsu.log`, so every record
now carries `component=app` or `component=rotate`. The window also mirrors
the log to stderr (useful with `wails dev`); commands do not, so their
terminal and journal output is one short sentence. Errors still go to
stderr.

### Environment

On the development machine the MATE session exports
`XDG_CURRENT_DESKTOP`, the D-Bus address and `DISPLAY` to systemd user
services (`systemctl --user show-environment`), which `gsettings` and
desktop detection need. Other systems may not; the timer unit will
include the desktop found when rotation is enabled as a fallback.

## Verification

Command tests run in temporary XDG directories with a fake wallpaper
setter. The real binary was run in a sandbox with an empty library (so no
wallpaper could change): no window, 73 ms, the expected exit codes and
messages.
