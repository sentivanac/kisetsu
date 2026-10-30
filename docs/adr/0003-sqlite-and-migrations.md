# ADR 0003: SQLite driver and schema migrations

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Kisetsu stores image metadata, collections, search history and settings
locally. The schema will change with every phase, and the app must upgrade
users' databases automatically without losing data.

## Decision

1. **Driver: `modernc.org/sqlite`** (pure Go translation of SQLite).
   - No extra C toolchain dependency, and cross-compiling stays simple.
   - Costs: about 4.5 MB larger binary (8.4 → 13 MB) and somewhat slower
     than the cgo driver `mattn/go-sqlite3`. Neither matters for a desktop
     app's local workload.
   - Requires Go 1.26.
2. **Connection settings** (applied to every pooled connection through the DSN):
   `foreign_keys=ON`, `journal_mode=WAL`, `busy_timeout=5000`, `synchronous=NORMAL`.
3. **Own minimal migrator** (`internal/database/migrate.go`) instead of
   `golang-migrate`.
   - Desktop apps only migrate forward, automatically, at startup. Down
     migrations and a separate CLI are not needed.
   - Migrations are `NNNN_description.sql` files embedded into the binary.
     Versions must start at 1 and have no gaps.
   - The schema version is stored in SQLite's built-in `PRAGMA user_version`;
     no bookkeeping table.
   - Each migration and its version update run in one transaction. SQLite
     DDL is transactional, so a failed migration leaves the database
     unchanged.
   - A database with a newer schema than the binary knows is refused, not
     modified, so downgrading Kisetsu cannot damage data.
4. **No tables yet.** The first tables arrive with the first features that
   use them (Phase 2), so the schema is never designed ahead of real needs.
5. The database file is created with `0600` permissions.

## Consequences

- Rules for writing migrations are in `internal/database/migrations/README.md`.
- WebKitGTK stores its own website data (`hsts-storage.sqlite`, `storage/`,
  `mediakeys/`) in the same `~/.local/share/kisetsu` directory, because it
  uses the program name. Wails v2 does not let us move it. Kisetsu's own
  files must therefore use explicit names (`kisetsu.db`, and later a
  dedicated subdirectory for images).
