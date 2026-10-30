# ADR 0005: Image model and provider architecture

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Kisetsu searches several image sources. Each has its own API, filters,
pagination and licensing. Search, the UI and later the local library need
one representation of a result, and adding a source must not require
changes to the search core.

## Decision

### Packages

```
internal/images/      Image: provider-independent model
internal/providers/   Provider interface, Query, Page, errors
  <name>/             one subpackage per provider, registered in main.go
internal/search/      uses []providers.Provider (Phase 2: a single provider)
```

`Image` has its own package because the library (Phase 4) will use it too
and must not depend on providers.

### Image

- Keeps provider, provider ID, page URL (for attribution), thumbnail,
  preview and download URLs, dimensions, MIME type, file size, author,
  original source and license.
- Aspect ratio and orientation are computed from the dimensions, not stored.
- **Licensing is never implicit.** `License.Status` must be `known` or
  `unknown`; it has no usable zero value, so a provider cannot forget it.
  `unknown` means the image must not be treated as free to redistribute.
- **Authors are never guessed.** Empty means unknown.
- `Validate()` rejects incomplete results. URLs must be absolute
  `http(s)`, so a provider response cannot make the UI load `file:` or
  `javascript:` URLs.

### Provider

```go
type Provider interface {
    ID() string    // stable, stored in the database
    Name() string  // for the UI
    Search(ctx context.Context, q Query) (Page, error)
}
```

- `ctx` carries cancellation and timeouts (needed for Phase 3).
- `Query` is provider-independent (text, page, minimum size, orientation).
  Each provider translates it into its API parameters. `Query.Matches`
  applies filters an API cannot, and serves as a final check.
- Errors are `*providers.Error` with a kind (`ErrInvalidQuery`,
  `ErrUnavailable`, `ErrRateLimited`), an optional `RetryAfter` and the
  cause. The `App` layer maps kinds to `apperr` codes for the UI.

### Content policy

Only safe-for-work content is returned. Providers request SFW content from
their API **and** drop any result their API marks otherwise.

## Consequences

- A new provider is a new subpackage plus one registration line.
- Shared HTTP concerns (client with timeout, User-Agent, client-side rate
  limiting with `golang.org/x/time/rate`) arrive with the first provider.
