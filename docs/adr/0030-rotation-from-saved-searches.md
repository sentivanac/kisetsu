# ADR 0030: Rotation from saved searches

- **Status:** accepted
- **Date:** 2026-11-02
- Fourth step of "Fetched" (Phase 6); builds on ADRs 0023 and 0027–0029.

## Decision

### Source

`rotation.Settings.SavedSearchID` makes rotation use the images fetched for
that saved search — and only those (user decision). A collection and a
saved search cannot both be chosen. A deleted saved search falls back to
the whole library, as a deleted collection does.

### Top-up before choosing

When fewer than **3** of the saved search's fetched images are unshown in
the current cycle, `Rotator.Next` first fetches **5** new ones (at most
3 minutes), then chooses. New images are unshown, so they come up soon.

- `rotation` does not import `fetched`; it receives a `TopUp` function,
  which `core.go` builds from `fetched.Fetcher`. The image on screen is
  protected from pruning.
- Without network the top-up fails, is logged, and rotation continues with
  the images it has. Only when there are none at all does it report "no
  images".
- `kisetsu rotate` now builds a search service and a download manager, and
  its timeout grew from 1 to 5 minutes. The window's "Next wallpaper" uses
  the window's download manager.

### Temporary files of other processes

`downloads.New` removed every leftover `dl-*` file at startup. With the
window and the rotation timer both downloading into the same directories,
a timer starting during a download in the window would have deleted that
download's temporary file. Only files older than an hour are removed now.

### UI

The Wallpaper tab's "Images from" list has two groups: **Your library**
(whole library and collections) and **New images from a saved search**.
Under it: how many images the source has, and for a saved search that
new ones are downloaded at screen size when fewer than 3 are left unshown.

## Verification

- Rotation tests: only the search's fetched images are used, top-up when
  running low (and not again while enough are left), offline use of
  existing images, no images, deleted search fallback, both sources
  rejected.
- In a sandbox with `XDG_CURRENT_DESKTOP=NONE` (so no wallpaper can be
  set), `kisetsu rotate` with a saved search "forest -snow" as source
  fetched 5 images from the live Wallhaven API (3.5 MB in total) in 4.5 s
  and then stopped at setting the wallpaper, as intended.
