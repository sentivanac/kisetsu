# ADR 0019: Library view and deleting images

- **Status:** accepted
- **Date:** 2026-11-02

## Decision

### Navigation

- Two tabs, **Search** and **Library** (WAI-ARIA tabs: `role="tab"`,
  arrow keys move between them). Both views stay mounted and are hidden
  with `hidden`, so switching tabs keeps search results and scroll state.
- The library reloads when its tab becomes visible, so new downloads
  appear. The search view rechecks which results are in the library when
  it becomes visible, so images deleted meanwhile lose their ✓.

### Library view

- `ListLibrary(offset, limit)` returns entries newest first, at most 200 per
  call, with `hasMore` (one extra row is fetched to know). The view loads
  60 at a time with "Load more".
- Each `LibraryItem` keeps the full attribution from the database; its
  thumbnail and preview URLs point at the local thumbnail, and `fileUrl` at
  the local original. The preview therefore works offline and never
  downloads the image again (`PreviewDialog.fullUrl`).
- `GetLibraryStats` gives "N images · size" for the header.

### Deleting

- Only on explicit request, confirmed in two steps inside the preview
  ("Delete…" → "Delete permanently" / "Keep", with focus on Keep). No
  browser `confirm()` dialog, which looks foreign in a desktop app.
- `DeleteFromLibrary(id)` removes the **database record first**, then the
  file and thumbnail. If a file cannot be removed, an unused file remains
  (logged); there is never a record pointing at a missing file.
- Files are removed through `libfiles.Server.Remove`, the same `os.Root`
  handles that serve them: only regular files inside `images/` and
  `thumbs/` can be removed. A damaged record pointing at `kisetsu.db` or
  `../anything` removes nothing (tested).
- Deleting an image removes it from all collections (database cascade).

### Verification in `wails dev`

ADR 0018 chose an asset-server middleware so that `/library/` works under
the Vite dev server too. Verified with `curl` against `wails dev` on
2026-11-02: a library file returned 200, `/library/kisetsu.db` and a
`../` path returned 404, and the app itself was still served by Vite.

### Binding name

The Wails binding generator warned that `Public` is a reserved word, so
`apperr.Public` was renamed to `apperr.PublicError`. The JSON sent to the
frontend is unchanged.

## Limitations

- No collection filter or sorting options yet (collections are the next
  step).
- React components are still tested only through their helper functions
  (`src/lib/library.ts`).
