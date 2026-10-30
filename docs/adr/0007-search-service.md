# ADR 0007: Search service and search errors in the UI

- **Status:** accepted
- **Date:** 2026-10-30

## Decision

### `internal/search.Service`

- Built in `main.go` from the registered providers; rejects empty or
  duplicate provider IDs at startup.
- `Search(ctx, providerID, query)` validates the query before calling the
  provider, so an invalid query never costs an API call.
- Each search is bounded by `DefaultTimeout` (20 s), which includes waiting
  for the provider's rate limiter. The shared `http.Client` has a 15 s
  timeout as a safety net.
- Every search is logged with provider, page, result count and duration.
  **The search text is logged only at debug level**, because search
  history is private.
- Phase 2 searches one provider chosen by the user. Concurrent
  multi-provider search (Phase 3) will be added here.

### Bound methods

- `ListProviders()` returns `{id, name}` for the source selector.
- `Search(providerID, query)` returns `providers.Page`.

### Error mapping (`app_search.go`)

| Failure | Code | Message |
|---|---|---|
| unknown provider, invalid query | `invalid_input` | generic, no internals |
| `ErrRateLimited` | `rate_limited` | names the provider; `retryAfterSeconds` set |
| timeout (`context.DeadlineExceeded`) | `provider_unavailable` | "did not respond in time" |
| other `ErrUnavailable` | `provider_unavailable` | "check your internet connection" |
| anything else | `internal` | generic |

`apperr.Public` gained `retryAfterSeconds` (rounded up, omitted when 0).
The frontend shows it via `describeError`.

### Frontend

A temporary `SearchPanel` (text, source, minimum size, orientation) lists
result sizes and license status. Only the most recent search may update the
UI, so a slow earlier response cannot overwrite a newer one. The gallery
replaces the list in the next step.

## Not done yet

- Cancelling a running search from the UI (Phase 3).
- Search history in the database.
