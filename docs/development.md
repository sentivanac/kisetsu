# Development

How to build, test and package Kisetsu. For using it, see the
[README](../README.md). Architecture decisions are in [adr/](adr/).

## Requirements

Developed on Ubuntu 22.04; the Windows build is cross-compiled from
Linux.

| Tool | Version | Notes |
|---|---|---|
| Go | 1.26+ | required by the SQLite driver |
| Node.js | 24 LTS | pinned in `.nvmrc` |
| Wails CLI | v2.17+ | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| GTK 3 / WebKitGTK 4.1 | | `sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev` |
| NSIS | 3 | only for `make windows`: `sudo apt install nsis` |

Kisetsu builds against **WebKitGTK 4.1** using the Go build tag `webkit2_41`.
The tag is set in `wails.json` and in the `Makefile`. `wails doctor` reports
`libwebkit2gtk-4.0-dev` as missing; that can be ignored.

## Development

```sh
nvm use          # switch to the Node version from .nvmrc
make dev         # run with hot reload
make check       # go vet (also for Windows), Go tests, TypeScript type check, frontend tests
make test        # Go tests only
make test-frontend  # frontend tests only (Vitest)
make test-live   # check providers against their real APIs (uses the network)
make build       # production binary in build/bin/kisetsu
```

When calling Go tools directly, pass the tag: `go test -tags webkit2_41 ./...`.

Code that differs between Linux and Windows lives in files named
`*_unix.go` and `*_windows.go` ([ADR 0043](adr/0043-platform-split.md)).
`make check` type-checks the Windows code too; the tests run on Windows
in CI (`windows-latest`).

## Ideas

Discussed but unplanned features are collected in [ideas.md](ideas.md).

## Releasing

See [releasing.md](releasing.md).

## Release package

```sh
make deb         # release binary + build/deb/kisetsu_<version>_amd64.deb
```

The version is set once, as `VERSION` in the `Makefile` (and, for the
Windows file version, `productVersion` in `wails.json`; a test keeps them
equal). `make deb` installs nothing; it needs `dpkg-dev` (for
`dpkg-shlibdeps`). See [ADR 0033](adr/0033-deb-package.md).

```sh
make windows     # build/bin/kisetsu.exe + kisetsu-<version>-windows-amd64-setup.exe
```

Cross-compiled; needs NSIS. See
[ADR 0046](adr/0046-windows-installer.md). To try it, use a Windows
virtual machine with a snapshot of a clean system, and go back to it
before each installer test.

After editing the icon source `build/linux/io.github.sentivanac.Kisetsu.svg`,
`make icons` renders the PNG sizes and the Windows icon (needs Inkscape
and ImageMagick).

## Safe testing of the wallpaper code

Tests use fakes and never change the real wallpaper. For an end-to-end
run of `kisetsu rotate` without touching your desktop, point the XDG
variables at a temporary directory and use a `PATH` without `gsettings`.
Setting `XDG_CURRENT_DESKTOP` is not enough: the desktop is read from the
login session ([ADR 0031](adr/0031-desktop-from-login-session.md)).

## Project layout

```
main.go            entry point: opens the window, or runs a command
core.go            setup shared by the window and commands
cmd.go             command-line commands (rotate, fetch, version, help)
cmd_agent.go       `kisetsu agent`: the background process
agent_control.go   the window starts, stops and checks the agent
app.go             thin bridge exposing Go methods to the frontend
internal/          business logic, independent of the UI
  agent/           background agent: changes the wallpaper when rotation is due
  autostart/       XDG autostart entry that starts the agent at login
  apperr/          errors with user-safe messages and stable codes
  appinfo/         build and platform information
  config/          XDG directory resolution
  logging/         slog logger setup
  database/        SQLite connection and schema migrations
    migrations/    numbered SQL migration files (embedded)
  downloads/       download queue into the library or the fetched cache
  fetched/         keeping and pruning fetched images
  filelock/        exclusive file locks between Kisetsu processes
  images/          provider-independent image model
  imaging/         image validation, thumbnails, SHA-256 and dHash
  providers/       Provider interface, query, errors
    wallhaven/     Wallhaven API (SFW only)
    commons/       Wikimedia Commons API (curated collections, best-effort SFW)
  library/         downloaded images and collections (database records)
  libfiles/        serves (and removes) library files under /library/
  rotation/        chooses and sets the next wallpaper, rotation history
  legacytimer/     removes the systemd timer that rotated in 0.1.0
  search/          runs searches against registered providers
  searches/        saved searches
  settings/        user preferences stored in the database
  wallpaper/       sets the wallpaper (Variety, MATE, GNOME, Cinnamon, KDE, Xfce)
app_search.go      multi-source search, cancellation, error messages
app_settings.go    settings methods exposed to the frontend
app_downloads.go   downloads and their progress events
app_library.go     library listing, statistics and deletion
app_collections.go collections
app_wallpaper.go   set a library image as the desktop wallpaper
app_rotation.go    rotation settings, background agent, tray setting, "Next wallpaper"
tray.go            the tray icon and its menu (shown by the agent)
app_saved_searches.go  saved searches
app_fetched.go     fetched images, Keep, screen size
gpu.go             WebView GPU policy (KISETSU_WEBVIEW_GPU)
frontend/          React + TypeScript + Vite
  src/components/  reusable UI components
  src/lib/         frontend helpers (backend errors, images, filters, screens)
  wailsjs/         TypeScript bindings generated by Wails (do not edit)
build/             icons and packaging resources
  linux/           .desktop file and icon sources
scripts/           packaging scripts
docs/adr/          architecture decision records
```

