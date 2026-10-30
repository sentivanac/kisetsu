# ADR 0002: File locations and logging

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Kisetsu needs places to keep settings, a database, downloaded images,
thumbnails and logs. It also needs logs that help diagnose problems
reported by users.

## Decision

1. **XDG Base Directory Specification** (`internal/config`).
   - Config, data, cache and state directories under `kisetsu/`, honouring
     the `XDG_*_HOME` variables. Relative values are ignored, as the
     specification requires.
   - Logs go to the **state** directory, which XDG defines for logs and
     history that are not portable user data.
   - Directories are created with `0700` and the log file with `0600`:
     they will contain search history, collections and provider API keys.
   - Creating directories never touches existing files.
2. **`log/slog` from the standard library** (`internal/logging`).
   - No extra dependency; structured key/value records.
   - Text format, appended to `kisetsu.log` and mirrored to stderr.
   - Level from `KISETSU_LOG_LEVEL`. An invalid value logs a warning and
     falls back to `info` instead of preventing startup.
3. `main.go` runs everything through `run() error`, so deferred cleanup
   (closing the log file) runs before the process exits with an error.

## Not decided yet

- A settings file format. It will be introduced together with the first
  real setting. (Resolved by ADR 0009: settings are stored in SQLite.)
- Log rotation. The log file grows without limit for now.
- Routing Wails' internal log output into the same logger.
