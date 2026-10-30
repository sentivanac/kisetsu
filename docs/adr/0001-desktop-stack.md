# ADR 0001: Desktop stack — Wails v2, WebKitGTK 4.1, React + TypeScript

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Kisetsu is a Linux-first desktop app with a Go core and an image-heavy UI.
We need a native window, a modern web-based UI, and a simple, typed way for
the UI to call Go code.

## Decision

1. **Wails v2** (v2.17) as the desktop framework.
   - Uses the system WebKitGTK instead of bundling Chromium, so binaries stay
     small (~10 MB, compared with ~150 MB for Electron).
   - Generates TypeScript bindings for exported methods of bound Go structs.
   - v2 is the stable, documented release line. We will revisit v3
     (multi-window, system tray) if those features become necessary.
2. **WebKitGTK 4.1** (build tag `webkit2_41`) instead of the default 4.0.
   - WebKitGTK 4.0 (libsoup2) is no longer shipped by Ubuntu 24.04+ and other
     current distributions; 4.1 is what users will have.
   - Cost: every Go build needs the tag. It is set in `wails.json`
     (`build:tags`) and in the `Makefile`.
3. **React + TypeScript + Vite** frontend (Wails `react-ts` template).
4. **Node.js 24 LTS**, pinned with `.nvmrc`. Vite 7 requires a recent Node.

## Structure

- `main.go` and `wails.json` stay at the repository root, where Wails expects them.
- `app.go` is a thin adapter: it only translates frontend calls into calls
  to `internal/` packages.
- All business logic lives in `internal/`, so it can be tested without a GUI
  and cannot be imported by other modules.

## Consequences

- `wails doctor` falsely reports `libwebkit2gtk-4.0-dev` as missing.
- Go tooling (`go test`, `go vet`, gopls) must use `-tags webkit2_41` when it
  compiles the root `main` package.
- UI rendering depends on the WebKitGTK version installed on the user's system.
