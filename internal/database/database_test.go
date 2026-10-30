package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedMigrationsAreValid(t *testing.T) {
	if _, err := embeddedMigrations(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCreatesDatabaseWithPragmas(t *testing.T) {
	ctx := context.Background()
	// A space in the path makes sure the DSN is built correctly.
	path := filepath.Join(t.TempDir(), "my data", FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("database file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("database file permissions = %o, want 600", perm)
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}
}

func TestOpenKeepsExistingData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), FileName)

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE notes (body TEXT); INSERT INTO notes VALUES ('keep me');"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var body string
	if err := db.QueryRow("SELECT body FROM notes").Scan(&body); err != nil {
		t.Fatal(err)
	}
	if body != "keep me" {
		t.Errorf("body = %q, want %q", body, "keep me")
	}
}
