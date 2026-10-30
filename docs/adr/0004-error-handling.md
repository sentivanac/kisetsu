# ADR 0004: Error handling between Go and the frontend

- **Status:** accepted
- **Date:** 2026-10-30

## Context

How Wails v2.17 handles errors from bound Go methods:

- A returned `error` rejects the JavaScript promise with `err.Error()`,
  which can expose internal details (file paths, SQL) and gives the
  frontend nothing stable to branch on.
- `options.App.ErrorFormatter` can replace that string with any JSON value.
- A **panic** in a bound method is recovered and logged, but the call is
  never answered: the promise stays pending forever.

## Decision

1. **`internal/apperr`** defines `Error{Code, Message, Err}`:
   - `Code` is a stable identifier the UI can act on (`internal`, `unavailable`;
     more are added as features need them, existing ones are never renamed).
   - `Message` is written for users.
   - `Err` is the underlying cause, only logged.
2. **`ErrorFormatter`** (in `main.go`) logs the full error and sends
   `apperr.Public{code, message}` to the frontend. Any error that is not an
   `apperr.Error` becomes a generic `internal` error.
3. **Every bound method returns an `error` and starts with
   `defer a.recoverPanic("Name", &err)`.** A panic is logged with its stack
   trace and returned as an `internal` error, so the promise always settles.
4. **`App` receives its dependencies (logger, database, paths) through
   `NewApp`**, so bound methods can be tested without a window.
5. **Frontend:** `src/lib/backend.ts` converts every rejection, including
   plain strings Wails sends for its own failures, into a `BackendError`.
   `src/components/ErrorBoundary.tsx` shows a recovery screen instead of a
   blank window when rendering fails.

`GetAppInfo` was replaced by `GetStatus`, which returns the same build
information plus the database status.

## Limitations

- Errors before the window opens (for example, a database created by a
  newer Kisetsu) are only written to stderr and the log file. A user who
  starts Kisetsu from the desktop sees nothing. Showing a native dialog is
  left for later.
- Frontend tests use Vitest in the Node environment (`*.test.ts` next to
  the code). `ErrorBoundary` is not tested yet: component tests need
  `jsdom` and Testing Library, which will be added when more components exist.
- Frontend `console.error` output is not forwarded to the Go log.
