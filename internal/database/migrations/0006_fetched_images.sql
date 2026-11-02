-- Images fetched automatically from saved searches (ADR 0028). They live in
-- the XDG cache and are not part of the user's library until kept.
ALTER TABLE images ADD COLUMN origin TEXT NOT NULL DEFAULT 'library'
    CHECK (origin IN ('library', 'fetched'));
ALTER TABLE images ADD COLUMN saved_search_id INTEGER
    REFERENCES saved_searches (id) ON DELETE SET NULL;

CREATE INDEX images_origin ON images (origin, saved_search_id);
