# ADR 0028: Storage for fetched images

- **Status:** accepted
- **Date:** 2026-11-02
- Second step of "Fetched" (Phase 6). Fetching itself follows.

## Context (user decisions, 2026-11-02)

- Fetched images live in the XDG cache, capped at **200 MB**; the oldest
  are removed, never library images; **Keep** moves an image into the
  library.
- Fetched images are **reduced to screen size**: a 7360 × 4912 Commons photo
  is 17.8 MB, while 1920 × 1280 covers the user's 1920 × 1080 screens at
  roughly 0.5–1 MB. Library downloads stay original.

## Decision

### Database (migration `0006_fetched_images.sql`)

`images.origin` (`library` | `fetched`) and `images.saved_search_id`
(`ON DELETE SET NULL`: deleting a saved search keeps its images,
unassigned). The user's library stays the user's: rotation from the
library, library statistics, collections and the search ✓ only look at
`origin = 'library'`.

### Files

```
~/.cache/kisetsu/fetched/images/<provider>/<id>.jpg   stored path fetched/images/…
~/.cache/kisetsu/fetched/thumbs/<provider>/<id>.jpg   stored path fetched/thumbs/…
~/.cache/kisetsu/fetched/tmp/                         downloads in progress
```

`libfiles` serves, removes and resolves these with the same `os.Root`
checks as the library (`/library/fetched/images/…`); `fetched/tmp` and
anything else is refused.

### Screen size

The window stores the largest screen width and height
(`settings` key `display.size`, from Wails `ScreenGetAll`, physical pixels)
at startup, because the rotation timer runs without a window. Default
1920 × 1080 until then.

### Fitting

`imaging.Process(r, Options{Fit})` also returns a JPEG (quality 90) just
large enough to **cover** the fit size with the image's aspect ratio
(3:2 → 1920 × 1280 for 1920 × 1080). Smaller images are kept as they are,
never enlarged. The stored dimensions, size and MIME type are those of the
stored file; **the SHA-256 stays that of the downloaded original**, so
exact duplicates with the library are still recognized.

`downloads.Manager.EnqueueFetched(img, {SavedSearchID, Fit})` stores into
the fetched root with `origin = fetched`. A manual download of an image that
was fetched returns `ErrInFetched`; the UI says to open it in Library →
Fetched and keep it.

### Keep

`fetched.Service.Keep` copies file and thumbnail into the library
(`O_EXCL`, numeric suffix instead of replacing), then updates the record
(`origin = library`, new paths, added now), and only then removes the
fetched files. If the update fails, the copies are removed and the image
stays fetched. Copying instead of renaming works when cache and data are on
different file systems. The kept image is the screen-sized copy; the
original remains on its source page.

### Limit

`fetched.Service.Prune(protect…)` removes the oldest fetched images until
they use at most 200 MB, skipping protected images (the wallpaper on
screen). It never touches the library. It runs after fetching (next step).

### UI

Library has a **Fetched** chip with its count; the header shows "12
images · 84 MB of 200 MB" and explains the limit and screen-size storage.
A fetched image's preview offers **Keep in my library**, Set as wallpaper
and Delete; collections apply to library images only.
