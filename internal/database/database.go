// Package database opens the Kisetsu SQLite database and keeps its
// schema up to date.
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver (pure Go, no cgo)
)

// FileName is the database file created in the data directory.
const FileName = "kisetsu.db"

//go:embed migrations
var migrationsFS embed.FS

// pragmas are applied to every connection in the pool.
var pragmas = []string{
	"foreign_keys(1)",     // enforce FOREIGN KEY constraints (off by default in SQLite)
	"journal_mode(WAL)",   // readers do not block the writer
	"busy_timeout(5000)",  // wait up to 5 s for a lock instead of failing immediately
	"synchronous(NORMAL)", // safe with WAL and much faster than FULL
}

// Open opens (or creates) the database at path and applies pending
// migrations. The caller must close the returned database.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	migrations, err := embeddedMigrations()
	if err != nil {
		return nil, err
	}
	return open(ctx, path, migrations)
}

func embeddedMigrations() ([]Migration, error) {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}
	return LoadMigrations(sub)
}

func open(ctx context.Context, path string, migrations []Migration) (*sql.DB, error) {
	if err := createPrivate(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if _, err := Migrate(ctx, db, migrations); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// createPrivate creates an empty database file readable only by the
// user, unless it already exists. SQLite gives its -wal and -shm files
// the same permissions as the database file.
func createPrivate(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create database %s: %w", path, err)
	}
	return f.Close()
}

func dsn(path string) string {
	q := url.Values{}
	for _, p := range pragmas {
		q.Add("_pragma", p)
	}
	return "file:" + uriPath(path) + "?" + q.Encode()
}

// uriPath escapes the characters that have a meaning in an SQLite URI, so a
// path containing them (for example in a user name) still opens the right
// file.
func uriPath(path string) string {
	return strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(path)
}
