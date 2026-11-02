# ADR 0029: Fetching new images for saved searches

- **Status:** accepted
- **Date:** 2026-11-02
- Third step of "Fetched" (Phase 6). Rotation from saved searches follows.

## Decision

`fetched.Fetcher.Fetch(savedSearch, want, protect…)`:

1. Runs the saved search on its registered sources (others are skipped),
   each **from its own cursor page** (migration `0007_fetch_cursors.sql`:
   `fetch_cursors (saved_search_id, provider, next_page)`, removed with the
   saved search).
2. Downloads results in order with `downloads.Manager.FetchNow` —
   synchronous, because the timer runs without a window — reduced to the
   stored display size (ADR 0028). Images already in the database, library
   or fetched, are skipped.
3. Advances each source's cursor; a source without more pages starts over
   at page 1 (older images may have been pruned in the meantime).
4. At most **3 rounds** (one API call per source per round) per Fetch, so
   one Fetch never uses many API calls.
5. Prunes to the 200 MB limit, protecting the given images (the wallpaper
   on screen) **and the images just fetched**. One Fetch may therefore go
   over the limit by its own size; the next one brings it back.
6. Failing sources are logged and do not fail the Fetch; with no network it
   fetches nothing and returns without error.

### Command

`kisetsu fetch NAME [--count N]` (N 1–24, default 5) fetches for the saved
search NAME (case-insensitive). It takes the rotation lock, so pruning
never races a wallpaper change, and protects the wallpaper on screen.
Exit status: 0 done, 1 failure, 2 invalid usage or unknown name.

### Shared setup

Provider registration and the download manager for fetching moved into
`core.go` (`searchService`, `fetcher`), used by the window and the
commands. The shared HTTP client timeout went from 15 s to 60 s: it also
downloads images of up to 100 MB; searches keep their own 20 s limit.

## Verification

In a sandbox (temporary XDG directories), against the live Wallhaven API,
a saved search "mountains -snow", at least 2560 × 1440, landscape:

- `fetch --count 3`: 3 images in 11.6 s, stored at 1920 × 1080 – 2555 × 1080,
  372–581 KB each; the next `fetch --count 2` continued on page 2.
- No temporary files left, files `0600`, unknown name → exit 2.

## Limitations

- Decoding and reducing large originals is CPU-heavy: about 3 s of CPU per
  image on the development machine (i7-10510U). Acceptable for a background
  timer; fetch counts stay small.
- Pages, not single images, are the unit of the cursor: unused results of
  a page are not revisited until the source starts over.
