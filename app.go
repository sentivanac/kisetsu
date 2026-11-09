package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/appinfo"
	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/fetched"
	"github.com/sentivanac/kisetsu/internal/libfiles"
	"github.com/sentivanac/kisetsu/internal/library"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/search"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/settings"
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
	ctx       context.Context
	log       *slog.Logger
	db        *sql.DB
	dbPath    string
	search    *search.Service
	settings  *settings.Store
	downloads *downloads.Manager
	library   *library.Store
	files     *libfiles.Server
	wallpaper WallpaperSetter
	// screen shows images through wallpaper, with text when turned on.
	screen   *onscreen.Service
	rotation *rotation.Rotator
	agent    AgentControl
	// legacyTimer is the 0.1.0 systemd timer, removed when found.
	legacyTimer LegacyTimer
	// trayHost reports whether the desktop shows tray icons; nil: no.
	trayHost func() bool

	savedSearches *searches.Store
	fetched       *fetched.Service

	// executable is this Kisetsu binary, to detect an autostart entry
	// that starts another one.
	executable   string
	rotationLock string

	// aboutRequested: the window was opened to show About (the tray's
	// "About Kisetsu"); the frontend asks once, with TakeAboutRequest.
	aboutRequested atomic.Bool

	// emit sends a Wails event. It is a no-op until startup, because the
	// Wails runtime exits the process when called without its context.
	emit func(name string, data any)

	// The running multi-source search, so a new search or CancelSearch
	// can stop it. Wails runs every frontend call in its own goroutine.
	searchMu     sync.Mutex
	searchSeq    uint64
	cancelSearch context.CancelFunc
}

// Deps are the dependencies of App, created in main.go.
type Deps struct {
	Log           *slog.Logger
	DB            *sql.DB
	DBPath        string
	Search        *search.Service
	Settings      *settings.Store
	Downloads     *downloads.Manager
	Library       *library.Store
	Files         *libfiles.Server
	Wallpaper     WallpaperSetter
	Screen        *onscreen.Service // uses Wallpaper
	Rotation      *rotation.Rotator
	Agent         AgentControl
	LegacyTimer   LegacyTimer // may be nil
	TrayHost      func() bool // may be nil
	SavedSearches *searches.Store
	Fetched       *fetched.Service
	Executable    string
	RotationLock  string
	// ShowAbout opens the window with About shown (`kisetsu --about`).
	ShowAbout bool
}

// NewApp creates a new App.
func NewApp(d Deps) *App {
	a := &App{
		ctx:         context.Background(),
		log:         d.Log,
		db:          d.DB,
		dbPath:      d.DBPath,
		search:      d.Search,
		settings:    d.Settings,
		downloads:   d.Downloads,
		library:     d.Library,
		files:       d.Files,
		wallpaper:   d.Wallpaper,
		screen:      d.Screen,
		rotation:    d.Rotation,
		agent:       d.Agent,
		legacyTimer: d.LegacyTimer,
		trayHost:    d.TrayHost,

		savedSearches: d.SavedSearches,
		fetched:       d.Fetched,

		executable:   d.Executable,
		rotationLock: d.RotationLock,
		emit:         func(string, any) {},
	}
	a.aboutRequested.Store(d.ShowAbout)
	return a
}

// startup is called when the app starts. The context is kept for
// Wails runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.emit = func(name string, data any) { runtime.EventsEmit(ctx, name, data) }
	a.retireTimer()
}

// onSecondInstance brings the window to the front when Kisetsu is started
// again while it runs, and shows About when that was asked for.
func (a *App) onSecondInstance(data options.SecondInstanceData) {
	a.log.Info("Kisetsu started again; showing the open window")
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	if slices.Contains(data.Args, aboutArg) {
		a.emit(aboutEvent, nil)
	}
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
