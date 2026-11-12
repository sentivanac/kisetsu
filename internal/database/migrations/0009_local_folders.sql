-- kisetsu: foreign keys off
-- Local folders (ADR 0053): folders the user added, whose images are used
-- where they are. Their images join the images table with origin 'folder',
-- so collections, history and the wallpaper work as for any image.
--
-- The images table is rebuilt, because SQLite cannot change a CHECK or a
-- UNIQUE NOT NULL column in place: origin gains 'folder', and sha256
-- becomes optional (a folder's files are not hashed; a picture may be in
-- the library and in a folder). Every row is copied unchanged; foreign
-- keys are off during the rebuild and checked before the commit.

CREATE TABLE folders (
    id       INTEGER PRIMARY KEY,
    -- Absolute path, as the user chose it.
    path     TEXT    NOT NULL UNIQUE CHECK (length(path) > 0),
    added_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE images_new (
    id              INTEGER PRIMARY KEY,
    provider        TEXT    NOT NULL,
    provider_id     TEXT    NOT NULL,
    page_url        TEXT    NOT NULL,
    download_url    TEXT    NOT NULL,
    source_url      TEXT    NOT NULL DEFAULT '',
    author_name     TEXT    NOT NULL DEFAULT '',
    author_url      TEXT    NOT NULL DEFAULT '',
    author_credit   TEXT    NOT NULL DEFAULT '',
    license_status  TEXT    NOT NULL CHECK (license_status IN ('known', 'unknown')),
    license_name    TEXT    NOT NULL DEFAULT '',
    license_url     TEXT    NOT NULL DEFAULT '',
    license_notice  TEXT    NOT NULL DEFAULT '',
    width           INTEGER NOT NULL CHECK (width > 0),
    height          INTEGER NOT NULL CHECK (height > 0),
    mime_type       TEXT    NOT NULL,
    file_size       INTEGER NOT NULL CHECK (file_size > 0),
    -- Paths are relative: to the library directory, the cache for fetched
    -- images, or "folders/<folder id>/…" for a folder's files.
    file_path       TEXT    NOT NULL UNIQUE,
    thumbnail_path  TEXT    NOT NULL DEFAULT '',
    -- Exact duplicates share a SHA-256; visually similar images have a
    -- close dHash (64-bit difference hash stored as a signed integer).
    -- Folder images have no SHA-256.
    sha256          TEXT    UNIQUE,
    dhash           INTEGER NOT NULL,
    added_at        TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    origin          TEXT    NOT NULL DEFAULT 'library'
                    CHECK (origin IN ('library', 'fetched', 'folder')),
    saved_search_id INTEGER REFERENCES saved_searches (id) ON DELETE SET NULL,
    folder_id       INTEGER REFERENCES folders (id) ON DELETE CASCADE,
    -- A folder file's modification time (Unix nanoseconds) when it was
    -- read, so syncing notices a changed file.
    file_mod_time   INTEGER,
    UNIQUE (provider, provider_id),
    CHECK ((origin = 'folder') = (folder_id IS NOT NULL)),
    CHECK (origin = 'folder' OR sha256 IS NOT NULL)
) STRICT;

INSERT INTO images_new (
    id, provider, provider_id, page_url, download_url, source_url,
    author_name, author_url, author_credit, license_status, license_name, license_url, license_notice,
    width, height, mime_type, file_size, file_path, thumbnail_path, sha256, dhash, added_at,
    origin, saved_search_id)
SELECT
    id, provider, provider_id, page_url, download_url, source_url,
    author_name, author_url, author_credit, license_status, license_name, license_url, license_notice,
    width, height, mime_type, file_size, file_path, thumbnail_path, sha256, dhash, added_at,
    origin, saved_search_id
FROM images;

DROP TABLE images;
ALTER TABLE images_new RENAME TO images;

CREATE INDEX images_added_at ON images (added_at);
CREATE INDEX images_origin ON images (origin, saved_search_id);
CREATE INDEX images_origin_added ON images (origin, added_at, id);
CREATE INDEX images_folder ON images (folder_id, file_path);
