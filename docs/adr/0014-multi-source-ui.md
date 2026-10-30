# ADR 0014: Multi-source search in the UI

- **Status:** accepted
- **Date:** 2026-10-30
- Completes Phase 3 together with ADRs 0010–0013.

## Decision

### Opt-in sources

- New optional interface `providers.ContentWarner { ContentWarning() string }`
  for providers that cannot guarantee safe-for-work results. The main
  `Provider` interface is unchanged.
- `ListProviders` reports `contentWarning` and `enabledByDefault`. **A source
  with a content warning is never enabled by default**; the warning is
  shown next to its checkbox. Wikimedia Commons implements it (ADR 0011).

### Search form

- Sources are checkboxes (registration order from `main.go`: Wallhaven,
  Wikimedia Commons). Searching needs at least one source.
- While a search runs, the Search button becomes **Cancel**
  (`CancelSearch`, ADR 0013); "Load more" has its own Cancel.
- Under the form, one status per source: image count, "(all shown)" when it
  has no more results, or its error message (with the retry delay for rate
  limits).
- The gallery labels every thumbnail with its source; the preview already
  names the source.

### Paging

`src/lib/sources.ts` decides what "Load more" asks for: the next page of
every source with more results, and the same page again for sources that
failed (not for canceled ones). Statuses of sources not asked again are
kept; image counts add up across pages.

### Remembered filters

`SearchFilters.ProviderID` became `ProviderIDs []string`. Values saved in
the old format (`providerId`) are read and converted; the next save writes
only the new field. Remembered sources that are no longer registered are
ignored; when none remain, the defaults apply.

### Removed

`App.Search(providerID, query)` was removed; `SearchSources` replaces it.
Its error-mapping tests now run through `SearchSources`.

## Limitations

- React components still have no automated tests; their logic lives in
  tested helpers (`lib/sources.ts`, `lib/filters.ts`, `lib/images.ts`).
- Duplicate images across sources are not detected yet (Phase 4).
