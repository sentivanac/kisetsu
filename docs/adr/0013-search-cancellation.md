# ADR 0013: Search cancellation

- **Status:** accepted
- **Date:** 2026-10-30

## Context

A user often starts a new search before the previous one has finished,
or wants to stop a slow one. Letting old searches run wastes the
providers' rate limits and bandwidth, and their late results must not
replace newer ones.

## Decision

- `App` keeps the cancel function of the running `SearchSources` call,
  guarded by a mutex (Wails runs every frontend call in its own goroutine).
- **Starting `SearchSources` cancels the previous search.** Because the
  cancellation travels through `context.Context`, it interrupts HTTP
  requests and waits for rate limiters in every provider.
- `CancelSearch()` cancels the running search; it does nothing when idle.
- A canceled call returns `apperr` code **`canceled`**. The frontend ignores
  it (`isCanceled` in `src/lib/backend.ts`); the frontend's own
  "latest request wins" guard (ADR 0007) remains as a second line.
- A sequence number ensures that a finishing search only clears the cancel
  function if no newer search has replaced it.
- Cancellation is logged at debug level, not as a warning or error, both in
  `search.Service` and in the Wails `ErrorFormatter`.

## Verification

- Unit tests cover `CancelSearch`, a new search canceling the previous one,
  and an idle `CancelSearch`, under the race detector.
- `make test-live` cancels a real two-source search after 50 ms; on
  2026-10-30 it returned after 50 ms with both sources reporting
  `context.Canceled`.

## Limitations

- Only multi-source searches (`SearchSources`) are cancelable. The
  single-provider `Search` will be removed when the UI switches over.
