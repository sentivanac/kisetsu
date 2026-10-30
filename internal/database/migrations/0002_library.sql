-- Downloaded images with their full attribution, and user collections.

CREATE TABLE images (
    id             INTEGER PRIMARY KEY,
    provider       TEXT    NOT NULL,
    provider_id    TEXT    NOT NULL,
    page_url       TEXT    NOT NULL,
    download_url   TEXT    NOT NULL,
    source_url     TEXT    NOT NULL DEFAULT '',
    author_name    TEXT    NOT NULL DEFAULT '',
    author_url     TEXT    NOT NULL DEFAULT '',
    license_status TEXT    NOT NULL CHECK (license_status IN ('known', 'unknown')),
    license_name   TEXT    NOT NULL DEFAULT '',
    license_url    TEXT    NOT NULL DEFAULT '',
    license_notice TEXT    NOT NULL DEFAULT '',
    width          INTEGER NOT NULL CHECK (width > 0),
    height         INTEGER NOT NULL CHECK (height > 0),
    mime_type      TEXT    NOT NULL,
    file_size      INTEGER NOT NULL CHECK (file_size > 0),
    -- Paths are relative to the library directory.
    file_path      TEXT    NOT NULL UNIQUE,
    thumbnail_path TEXT    NOT NULL DEFAULT '',
    -- Exact duplicates share a SHA-256; visually similar images have a
    -- close dHash (64-bit difference hash stored as a signed integer).
    sha256         TEXT    NOT NULL UNIQUE,
    dhash          INTEGER NOT NULL,
    added_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (provider, provider_id)
) STRICT;

CREATE INDEX images_added_at ON images (added_at);

CREATE TABLE collections (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(trim(name)) > 0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE collection_images (
    collection_id INTEGER NOT NULL REFERENCES collections (id) ON DELETE CASCADE,
    image_id      INTEGER NOT NULL REFERENCES images (id) ON DELETE CASCADE,
    added_at      TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (collection_id, image_id)
) STRICT;

CREATE INDEX collection_images_image ON collection_images (image_id);
