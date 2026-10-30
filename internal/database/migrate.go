package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
)

// Migration is one forward-only schema change.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// migrationFile matches names such as 0001_create_images.sql.
var migrationFile = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// LoadMigrations reads *.sql files from the root of fsys. Versions must
// start at 1 and have no gaps, so a missing or misnumbered file fails
// loudly instead of silently skipping a schema change. Other files are
// ignored.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	var migrations []Migration
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".sql" {
			continue
		}
		m := migrationFile.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migration %q: name must look like 0001_description.sql", e.Name())
		}
		version, _ := strconv.Atoi(m[1])
		content, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", e.Name(), err)
		}
		migrations = append(migrations, Migration{Version: version, Name: m[2], SQL: string(content)})
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	for i, m := range migrations {
		if want := i + 1; m.Version != want {
			return nil, fmt.Errorf("migration %04d_%s: expected version %04d (versions must start at 1 without gaps or duplicates)", m.Version, m.Name, want)
		}
	}
	return migrations, nil
}

// SchemaVersion returns the schema version stored in the database.
func SchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}

// Migrate applies pending migrations in order and returns how many were
// applied. Each migration and its version bump run in one transaction,
// so a failed migration leaves the database at the previous version.
//
// A database newer than the known migrations is rejected rather than
// modified: it was created by a newer Kisetsu and may hold data this
// version does not understand.
func Migrate(ctx context.Context, db *sql.DB, migrations []Migration) (int, error) {
	current, err := SchemaVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	if latest := len(migrations); current > latest {
		return 0, fmt.Errorf("database schema version %d is newer than this version of Kisetsu supports (%d)", current, latest)
	}

	applied := 0
	for _, m := range migrations[current:] {
		if err := apply(ctx, db, m); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

func apply(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration %04d_%s: begin: %w", m.Version, m.Name, err)
	}
	defer tx.Rollback() // no-op after a successful commit

	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("migration %04d_%s: %w", m.Version, m.Name, err)
	}
	// PRAGMA does not accept bound parameters; Version is an int.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
		return fmt.Errorf("migration %04d_%s: set version: %w", m.Version, m.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %04d_%s: commit: %w", m.Version, m.Name, err)
	}
	return nil
}
