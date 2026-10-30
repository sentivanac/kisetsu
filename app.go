package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/search"
)

// App is the bridge between the frontend and the Go backend.
// Its exported methods are callable from TypeScript via the generated
// bindings in frontend/wailsjs. Keep it thin: business logic belongs
// in internal/ packages.
//
// Every exported method must return an error and start with
//
//	defer a.recoverPanic("MethodName", &err)
//
// Wails recovers panics but never answers the frontend call, which
// would leave the JavaScript promise pending forever.
type App struct {
	ctx    context.Context
	log    *slog.Logger
	db     *sql.DB
	dbPath string
	search *search.Service
}

// NewApp creates a new App.
func NewApp(log *slog.Logger, db *sql.DB, dbPath string, searcher *search.Service) *App {
	return &App{
		ctx:    context.Background(),
		log:    log,
		db:     db,
		dbPath: dbPath,
		search: searcher,
	}
}

// startup is called when the app starts. The context is kept for
// Wails runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// recoverPanic turns a panic in a bound method into an internal error.
// It must be deferred directly.
func (a *App) recoverPanic(method string, err *error) {
	if r := recover(); r != nil {
		a.log.Error("panic in bound method", "method", method, "panic", r, "stack", string(debug.Stack()))
		*err = apperr.InternalError(fmt.Errorf("panic in %s: %v", method, r))
	}
}

// Status describes the running application.
type Status struct {
	App      appinfo.Info   `json:"app"`
	Database DatabaseStatus `json:"database"`
}

// DatabaseStatus describes the local database.
type DatabaseStatus struct {
	Path          string `json:"path"`
	SchemaVersion int    `json:"schemaVersion"`
}

// GetStatus returns build information and the state of the database.
func (a *App) GetStatus() (status Status, err error) {
	defer a.recoverPanic("GetStatus", &err)

	version, err := database.SchemaVersion(a.ctx, a.db)
	if err != nil {
		return Status{}, apperr.New(apperr.Unavailable, "The local database is not available.", err)
	}

	return Status{
		App:      appinfo.Get(),
		Database: DatabaseStatus{Path: a.dbPath, SchemaVersion: version},
	}, nil
}
