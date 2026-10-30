# ADR 0015: Library and collections schema

- **Status:** accepted
- **Date:** 2026-10-30
- First step of Phase 4 (local image library).

## Decision

### Tables (migration `0002_library.sql`)

- **`images`**: one downloaded image. It keeps **all attribution exactly as
  the provider gave it** — provider and provider ID, page URL, download
  URL, original source, author name and URL, license status, name, URL and
  notice — plus dimensions, MIME type, file size, file and thumbnail paths,
  SHA-256, dHash and the time it was added.
  - `UNIQUE (provider, provider_id)`: a provider image is stored once.
  - `UNIQUE (sha256)`: identical files are stored once.
  - `CHECK` constraints reject records without a valid license status,
    dimensions or size; tables are `STRICT`.
  - Paths are relative to the library directory, so the data directory
    can move.
  - The provider's thumbnail and preview URLs are not stored; the library
    has its own thumbnails.
  - **The search text that found an image is not stored** (privacy).
- **`collections`**: name unique case-insensitively (`COLLATE NOCASE`),
  1–100 characters after whitespace is collapsed.
- **`collection_images`**: membership, with `ON DELETE CASCADE` on both
  sides: deleting an image or a collection removes the membership, never
  the other side.

### Hashes

- SHA-256 identifies exact duplicates.
- dHash (64-bit difference hash, ADR to follow with the image-processing
  step) finds visually similar images. SQLite integers are signed, so the
  `uint64` is stored as its `int64` bit pattern and converted back on read.
  Near-duplicate search compares hashes in Go (`DHashes`); a library of
  thousands of images needs no index for that.

### `internal/library.Store`

- Records only. Files are written and removed by the download and library
  services in later steps; `Delete` does not touch files.
- `Add` checks for an existing provider image (`ErrAlreadyInLibrary`) and an
  identical file (`ErrDuplicateFile`) inside its transaction, so callers
  get a precise error; the `UNIQUE` constraints remain as a safety net.
- `List` returns newest first, optionally limited to one collection, with
  paging. `Stats` reports the image count and total size for the disk-usage
  display.
- Adding an image to a collection twice, or removing one that is not in
  it, is not an error.
