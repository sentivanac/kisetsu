# ADR 0018: Serving library files and the download UI

- **Status:** accepted
- **Date:** 2026-10-30

## Serving local files

The WebView may not load `file://` URLs, so library files are served by
the Wails asset server under `/library/` (`internal/libfiles`):

- Implemented as an `assetserver.Middleware`, not a fallback `Handler`: in
  `wails dev` the Vite dev server answers every unknown path with
  `index.html`, so a fallback handler would never be reached. The
  middleware sees each request first in both modes.
- Only `images/` and `thumbs/` are reachable, each opened with **`os.Root`**
  (Go 1.24+), which refuses `..` and symbolic links leading outside the
  directory. `fs.ValidPath` rejects malformed names first. The database,
  `tmp/`, WebKit's files and directory listings return 404; only GET and
  HEAD are allowed.
- Responses carry `X-Content-Type-Options: nosniff` and a one-day private
  cache, since a library path always refers to the same file.
- `libfiles.URL(relPath)` builds the frontend URL from a stored path.

Tests cover each refusal, including a symbolic link inside `images/` that
points at the database.

## Download UI

- `LibraryStatus(keys)` returns the entry IDs of shown search results that
  are already downloaded, in one query per 400 images (`IDsBySource`, SQL
  row values) instead of one call per image. The search panel calls it for
  every page of results; a failure only means results are not marked.
- The preview has a **Download** button (`DownloadButton`) with a progress
  bar and a cancel button while active, and a message afterwards: saved,
  similar to N images, identical file already present, or the user-safe
  error. "Already in your library" from `Download` marks the image instead
  of showing an error.
- Gallery thumbnails show ✓ for images in the library and a thin progress
  bar while downloading.
- Download events are kept in one store (`useDownloads`, built on
  `useSyncExternalStore`), so every component sees the same state and the
  Wails listener is registered once. The state logic is in tested pure
  functions (`src/lib/downloads.ts`).

## Verification

`make test-live` now also downloads a real Wallhaven image through the
whole pipeline; on 2026-10-30 a 3888 × 2592 JPEG (1.8 MB) was fetched,
validated, thumbnailed and recorded in 1.0 s.

## Limitations

- Download state lives in memory; after a restart only "in your library"
  is known (from the database), not past failures.
- The library itself is not browsable yet (next step).
