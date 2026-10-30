package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/database"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	path := filepath.Join(t.TempDir(), database.FileName)
	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), db, path)
}

func TestGetStatus(t *testing.T) {
	app := newTestApp(t)

	status, err := app.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.App != appinfo.Get() {
		t.Errorf("App = %+v, want %+v", status.App, appinfo.Get())
	}
	if status.Database.Path != app.dbPath {
		t.Errorf("Database.Path = %q, want %q", status.Database.Path, app.dbPath)
	}
}

func TestGetStatusReportsUnavailableDatabase(t *testing.T) {
	app := newTestApp(t)
	app.db.Close()

	_, err := app.GetStatus()

	if appErr, ok := errors.AsType[*apperr.Error](err); !ok || appErr.Code != apperr.Unavailable {
		t.Fatalf("err = %v, want an %q apperr.Error", err, apperr.Unavailable)
	}
}

func TestBoundMethodRecoversPanic(t *testing.T) {
	app := newTestApp(t)
	app.db = nil // makes GetStatus panic inside database/sql

	_, err := app.GetStatus()

	if got := apperr.ToPublic(err).Code; got != apperr.Internal {
		t.Errorf("code = %q, want %q (err: %v)", got, apperr.Internal, err)
	}
}
