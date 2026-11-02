package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func file(content string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(content)}
}

func openTestDB(t *testing.T, migrations []Migration) *sql.DB {
	t.Helper()
	db, err := open(context.Background(), filepath.Join(t.TempDir(), FileName), migrations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestLoadMigrations(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_second.sql": file("CREATE TABLE b (id INTEGER);"),
		"0001_first.sql":  file("CREATE TABLE a (id INTEGER);"),
		"README.md":       file("ignored"),
	}

	got, err := LoadMigrations(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d migrations, want 2", len(got))
	}
	if got[0].Version != 1 || got[0].Name != "first" || got[1].Version != 2 || got[1].Name != "second" {
		t.Errorf("unexpected order or names: %+v", got)
	}
}

func TestLoadMigrationsEmpty(t *testing.T) {
	got, err := LoadMigrations(fstest.MapFS{"README.md": file("only docs")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d migrations, want 0", len(got))
	}
}

func TestLoadMigrationsRejectsInvalidSets(t *testing.T) {
	tests := map[string]fstest.MapFS{
		"bad name":         {"1_first.sql": file("")},
		"uppercase name":   {"0001_First.sql": file("")},
		"gap":              {"0001_a.sql": file(""), "0003_c.sql": file("")},
		"duplicate":        {"0001_a.sql": file(""), "0001_b.sql": file("")},
		"does not start 1": {"0002_b.sql": file("")},
	}
	for name, fsys := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadMigrations(fsys); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestMigrateAppliesInOrderAndIsIdempotent(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Name: "a", SQL: "CREATE TABLE a (id INTEGER PRIMARY KEY);"},
		// Several statements in one file.
		{Version: 2, Name: "b", SQL: "CREATE TABLE b (id INTEGER PRIMARY KEY);\nINSERT INTO b (id) VALUES (1);"},
	}
	db := openTestDB(t, migrations)
	ctx := context.Background()

	if v, _ := SchemaVersion(ctx, db); v != 2 {
		t.Errorf("schema version = %d, want 2", v)
	}
	if !tableExists(t, db, "a") || !tableExists(t, db, "b") {
		t.Error("tables were not created")
	}

	applied, err := Migrate(ctx, db, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if applied != 0 {
		t.Errorf("second run applied %d migrations, want 0", applied)
	}
}

func TestMigrateAppliesOnlyPending(t *testing.T) {
	ctx := context.Background()
	first := []Migration{{Version: 1, Name: "a", SQL: "CREATE TABLE a (id INTEGER);"}}
	db := openTestDB(t, first)

	all := append(first, Migration{Version: 2, Name: "b", SQL: "CREATE TABLE b (id INTEGER);"})
	applied, err := Migrate(ctx, db, all)
	if err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Errorf("applied %d migrations, want 1", applied)
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, []Migration{{Version: 1, Name: "a", SQL: "CREATE TABLE a (id INTEGER);"}})

	broken := []Migration{
		{Version: 1, Name: "a", SQL: "CREATE TABLE a (id INTEGER);"},
		{Version: 2, Name: "broken", SQL: "CREATE TABLE b (id INTEGER);\nTHIS IS NOT SQL;"},
	}
	if _, err := Migrate(ctx, db, broken); err == nil || !strings.Contains(err.Error(), "0002_broken") {
		t.Fatalf("expected an error naming the migration, got %v", err)
	}

	if v, _ := SchemaVersion(ctx, db); v != 1 {
		t.Errorf("schema version = %d, want 1", v)
	}
	if tableExists(t, db, "b") {
		t.Error("table from the failed migration was not rolled back")
	}
}

func TestMigrateRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, nil)
	if _, err := db.Exec("PRAGMA user_version = 5"); err != nil {
		t.Fatal(err)
	}

	migrations := []Migration{{Version: 1, Name: "a", SQL: "CREATE TABLE a (id INTEGER);"}}
	if _, err := Migrate(ctx, db, migrations); err == nil {
		t.Fatal("expected an error for a newer database")
	}
	if v, _ := SchemaVersion(ctx, db); v != 5 {
		t.Errorf("schema version changed to %d", v)
	}
	if tableExists(t, db, "a") {
		t.Error("newer database was modified")
	}
}

// Migration 0003 moves credit texts that were stored as author names into
// the new author_credit column, without losing them.
func TestMigration0003MovesLongAuthorNames(t *testing.T) {
	ctx := context.Background()
	all, err := embeddedMigrations()
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, all[:2]) // the schema before 0003

	long := strings.Repeat("Please mention me as the author. ", 5)
	insert := `INSERT INTO images (provider, provider_id, page_url, download_url, author_name,
		license_status, width, height, mime_type, file_size, file_path, sha256, dhash)
		VALUES ('p', ?, 'https://x', 'https://x', ?, 'unknown', 1, 1, 'image/jpeg', 1, ?, ?, 0)`
	for i, name := range []string{"Short Name", long} {
		if _, err := db.Exec(insert, i, name, fmt.Sprint("f", i), fmt.Sprint("s", i)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Migrate(ctx, db, all); err != nil {
		t.Fatal(err)
	}

	rows := map[string][2]string{}
	r, err := db.Query(`SELECT provider_id, author_name, author_credit FROM images`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for r.Next() {
		var id, name, credit string
		r.Scan(&id, &name, &credit)
		rows[id] = [2]string{name, credit}
	}
	if rows["0"] != [2]string{"Short Name", ""} {
		t.Errorf("short name changed: %q", rows["0"])
	}
	if rows["1"] != [2]string{"", long} {
		t.Errorf("long name not moved to credit: %q", rows["1"])
	}
}
