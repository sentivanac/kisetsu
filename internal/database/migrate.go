package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Migration is one forward-only schema change.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// foreignKeysOff marks, on a migration's first line, a migration that
// rebuilds a table other tables refer to. SQLite's procedure for that
// needs foreign keys off: with them on, dropping the old table would
// delete the rows that refer to it (ON DELETE CASCADE). The links are
// checked before the migration is committed.
const foreignKeysOff = "-- kisetsu: foreign keys off"

func (m Migration) needsForeignKeysOff() bool {
	first, _, _ := strings.Cut(m.SQL, "\n")
	return strings.TrimSpace(first) == foreignKeysOff
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
	if m.needsForeignKeysOff() {
		return applyWithoutForeignKeys(ctx, db, m)
	}
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

// applyWithoutForeignKeys runs m with foreign keys off, on one connection
// (the setting is per connection and cannot change inside a transaction),
// and commits only if every foreign key still points to an existing row.
func applyWithoutForeignKeys(ctx context.Context, db *sql.DB, m Migration) (err error) {
	fail := func(step string, err error) error {
		return fmt.Errorf("migration %04d_%s: %s: %w", m.Version, m.Name, step, err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fail("connect", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fail("foreign keys off", err)
	}
	defer func() {
		// The connection goes back to the pool: it must enforce foreign
		// keys again, as every other connection does.
		if _, onErr := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); onErr != nil {
			err = errors.Join(err, fail("foreign keys on", onErr))
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fail("begin", err)
	}
	defer tx.Rollback() // no-op after a successful commit
	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fail("run", err)
	}
	if err := checkForeignKeys(ctx, tx); err != nil {
		return fail("check", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
		return fail("set version", err)
	}
	if err := tx.Commit(); err != nil {
		return fail("commit", err)
	}
	return nil
}

// checkForeignKeys fails if any row refers to a row that does not exist.
func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fk int
		if err := rows.Scan(&table, &rowid, &parent, &fk); err != nil {
			return err
		}
		return fmt.Errorf("row %d of %s refers to a missing row of %s", rowid.Int64, table, parent)
	}
	return rows.Err()
}
