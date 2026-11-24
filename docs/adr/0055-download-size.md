# ADR 0055: Download size and Get original

- **Status:** accepted, for 0.9.0
- **Date:** 2026-11-24

## Context

Download stored the original, Keep a copy reduced to the screen, and after
Keep the original could no longer be downloaded. Two names for "save this
image" were confusing, and a 17 MB original is more than a wallpaper needs.

## Decision (user decisions)

- One action, **Download**, with a setting: the original (default) or screen
  size. `DownloadAs` overrides it for one image.
- The reduced version is called **screen copy**: it covers the screen and is
  not cropped, so it is rarely exactly the screen's size.
- It uses the fetched copy when there is one that covers the
  present screen; otherwise the image is downloaded and reduced.
- When a download fails and a fetched copy exists, it can be kept instead
  (`Progress.FetchedID`). Favorites keep the fetched copy, as before.
- **Get original** replaces a reduced copy by its original, under the same
  entry.

### Database (migration `0011_original_size.sql`)

`images.original_width` and `original_height`; equal to the stored size for
an original, NULL for images stored before 0.9.0. For those, Get original
first compares the file's SHA-256 with the recorded one (always that of the
original), and downloads only if they differ. The preview of such an image
does the same check (`CheckOriginal`); an original is recorded, a reduced
file is checked again next time, because its original's size is not known.
A fetched file that is the original is kept without a download.

### Safety

Get original refuses an image that looks different from the copy, or is
smaller (`downloads.ErrChanged`): sources replace files. The earlier file is
removed only after the entry points at the new one, and not while it is the
wallpaper on screen.

## Limitations

- A screen copy covers the largest screen when the image is downloaded.
- Reduced images are JPEG, so a PNG loses its transparency.
- A database opened by 0.9.0 cannot be opened by earlier versions.
