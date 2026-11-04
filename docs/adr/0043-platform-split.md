# ADR 0043: Code split by operating system

## Status

Accepted (2026-11-04). First step of Windows support (0.3.0).

## Context

Kisetsu was written for Linux. For Windows, two things did not even
compile (`syscall.Flock` for file locks, `syscall.Stat_t` in the Variety
check), and several others compile but only make sense on Linux:
detached processes (`Setsid`), stopping the agent with SIGTERM, the D-Bus
session watch, the tray icon by name (Ayatana), gsettings and the other
desktop commands, XDG autostart and the 0.1.0 systemd timer.

## Decision

Go's file name suffixes and build constraints, no runtime checks and no
new interfaces:

- `internal/filelock`: `filelock_unix.go` (flock) and
  `filelock_windows.go` (`LockFileEx`). Windows byte-range locks are
  mandatory, so a byte far past the end of the file is locked; the file
  itself stays readable and writable (the agent keeps its PID in it).
- `internal/wallpaper`: shared types in `wallpaper.go`, the Linux desktops
  in `wallpaper_unix.go`, Windows in `wallpaper_windows.go`.
- Main package: `platform_unix.go` and `platform_windows.go` hold starting
  a detached process, stopping the agent, the session watch, the tray
  host check and how the tray icon is set.
- Tests that need Unix (`sh`, `flock(1)`, `/proc`) carry `!windows`.

`make check` now includes `check-windows`: `GOOS=windows go vet ./...`,
which type-checks the code and the tests for Windows on Linux. The tests
themselves run on `windows-latest` in CI; that job does not fail the run
until the port is complete.

## Consequences

- One code base; each platform-specific file is small and named after
  its platform.
- Windows behaviour is checked by CI and by hand in a Windows 10 VM; until
  a feature is seen working there, it is not listed as supported.
