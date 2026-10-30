# ADR 0017: Download queue

- **Status:** accepted
- **Date:** 2026-10-30

## Decision

### Layout

Inside the data directory (`~/.local/share/kisetsu`):

```
images/<provider>/<id>.<ext>   original files (0600)
thumbs/<provider>/<id>.jpg     480 px thumbnails
tmp/dl-*                       downloads in progress
```

Directories are `0700`. Thumbnails stay next to the originals rather than
in the XDG cache, so a cleared cache never leaves the library without
thumbnails. Provider names and IDs are reduced to `[A-Za-z0-9_-]` for file
names, so a provider ID such as `../../etc` cannot leave the library; the
database keeps the original ID.

### `internal/downloads.Manager`

1. `Enqueue` validates the image (`images.Image.Validate`: absolute http(s)
   URLs only), refuses images already in the library
   (`library.ErrAlreadyInLibrary`) and returns the running download if the
   same image is already being fetched. The key is `provider/providerId`,
   the same as the frontend's `imageKey`.
2. At most **3 downloads** run at once.
3. The file streams into `tmp/` with a **100 MB** limit, enforced both on
   `Content-Length` and on the bytes actually received. Progress is
   reported at most ten times per second.
4. `imaging.Process` validates it and derives thumbnail and hashes
   (ADR 0016). The decoded dimensions and format replace the provider's;
   the attribution is kept as it was.
5. **Exact duplicate** (same SHA-256): nothing is stored; the result is
   `duplicate` with the existing entry. Two simultaneous downloads of the
   same content end the same way. **Similar images** (dHash) are stored and
   reported in `SimilarIDs`, for the user to decide. Nothing is deleted
   automatically.
6. The file and thumbnail are moved into place with `os.Link` + remove,
   which, unlike `os.Rename`, **fails instead of replacing an existing
   file**; an occupied name gets a `-2`, `-3`, … suffix. Then the entry is
   added to the library. If that fails, only the files this download
   created are removed.

A failure or cancellation at any step removes the temporary file. At
startup, leftover `tmp/dl-*` files from a crash are deleted — only those,
only in Kisetsu's own `tmp/` directory. `Close` cancels all downloads and
waits; `main.go` calls it before closing the database.

### Frontend contract

- `Download(image) → key`; errors: `already_in_library`, `invalid_input`.
- `CancelDownload(key)`.
- Event `download:progress` with `{key, state, received, total, entryId?,
  similarIds?, error?}`; states `queued`, `downloading`, `processing`,
  `done`, `duplicate`, `failed`, `canceled`. Errors carry user-safe
  messages (file too large, not an image, damaged, source did not provide
  the file, network problem).
- The Wails runtime exits the process when `EventsEmit` is called without
  its context, so `App.emit` is a no-op until `startup` installs the real
  emitter. Tests replace it to capture events.

## Limitations

- Downloads are not resumed after a restart; an interrupted download must
  be started again.
- Image servers (not the provider APIs) have no client-side rate limit
  beyond the three parallel downloads.
