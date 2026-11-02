-- How far each source of a saved search has been fetched (ADR 0029).
CREATE TABLE fetch_cursors (
    saved_search_id INTEGER NOT NULL REFERENCES saved_searches (id) ON DELETE CASCADE,
    provider        TEXT    NOT NULL,
    next_page       INTEGER NOT NULL CHECK (next_page >= 1),
    PRIMARY KEY (saved_search_id, provider)
) STRICT;
