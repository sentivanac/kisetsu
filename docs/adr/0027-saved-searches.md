# ADR 0027: Saved searches

- **Status:** accepted
- **Date:** 2026-11-02
- First step of "Fetched" (Phase 6): saved searches are what Kisetsu will
  fetch new images from.

## Context (user decisions, 2026-11-02)

- Fetched images live in the XDG cache (`~/.cache/kisetsu/fetched`), capped
  at 200 MB; the oldest fetched images that were not kept are removed,
  never library images. "Keep" moves an image into the library.
- A saved search as rotation source uses only that search's fetched
  images.

## Decision

- Migration `0005_saved_searches.sql`: `saved_searches (id, name, query,
  created_at)`; names unique case-insensitively, `query` is JSON.
- `searches.Query` keeps the text (with `-word` exclusions), the sources,
  the minimum size and the orientation — the same as the search form. The
  search text is stored only because the user saves it explicitly; normal
  searches are still not recorded.
- `Query.Validate` is strict because the timer will run saved searches
  unattended: at least one source, no repeats, and a valid
  `providers.Query`. `Query.Provider(page)` returns the normalized provider
  query.
- Names follow the collection rules (whitespace collapsed, 1–100
  characters, unique case-insensitively); the frontend reuses `NameForm`
  with a message naming "saved search".
- A stored query that can no longer be read (written by another version)
  is left out of the list instead of breaking it.

### UI (Search tab)

- A row of saved searches above the form; clicking one fills the form and
  runs it. Sources that no longer exist are skipped; if none remain, a
  message says so.
- **+ Save this search** saves the current form (suggested name: the search
  text, or "Popular images" when it is empty).
- The saved search that is shown can be renamed or deleted (with
  confirmation). Changing the form — text, sources, size, orientation or
  "Use my screen size" — means it no longer shows that saved search, so
  the rename and delete buttons disappear.

## Not yet

Fetching images from saved searches and rotating them are the next steps.
