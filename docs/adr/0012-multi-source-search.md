# ADR 0012: Concurrent multi-source search

- **Status:** accepted
- **Date:** 2026-10-30
- Extends ADR 0007.

## Decision

### `search.Service.SearchMany(ctx, sources, query)`

- `sources` is a list of `{providerId, page}`: every provider pages
  independently, so "Load more" asks each source for its own next page and
  only sources that still have results are asked again.
- The request is checked **before any provider is called**: at least one
  source, every source registered, no source twice, and a valid query for
  every requested page. The query is normalized once (ADR 0010).
- Each source runs in its own goroutine through `Search`, so each keeps its
  own timeout, rate limiter and logging. A slow source does not delay the
  start of the others; the call returns when all have finished or timed out.
- **A failing source is not an error of the search.** Its error is stored in
  its `SourceOutcome`; the other sources' images are still returned.
- Results are **interleaved round-robin** in request order (first image of
  each source, then the second, …), so a source with many results cannot
  bury the others. Duplicates across sources are not detected yet
  (Phase 4, perceptual hashing).

### Bound method `SearchSources(sources, query)`

Returns `{images, sources}`, where every `SourceStatus` has the source's
display name, page, `hasMore`, image count and, when it failed, the
user-safe `apperr.Public` error (same messages as ADR 0007, naming the
source). The call itself fails only for invalid requests
(`invalid_input`: no source, unknown or repeated source, invalid query).

`Search(providerID, query)` stays until the UI switches to `SearchSources`
(Phase 3, step 5).

## Verification

`make test-live` runs a multi-source search against the real Wallhaven
and Wikimedia Commons APIs; on 2026-10-30 it returned 24 + 24 interleaved
images in 2.8 s.
