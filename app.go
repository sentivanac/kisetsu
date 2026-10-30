package main

import (
	"context"

	"github.com/sentivanac/kisetsu/internal/appinfo"
)

// App is the bridge between the frontend and the Go backend.
// Its exported methods are callable from TypeScript via the generated
// bindings in frontend/wailsjs. Keep it thin: business logic belongs
// in internal/ packages.
type App struct {
	ctx context.Context
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is kept for
// Wails runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetAppInfo returns information about the running build.
func (a *App) GetAppInfo() appinfo.Info {
	return appinfo.Get()
}
