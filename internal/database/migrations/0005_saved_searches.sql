-- Searches the user saved by name, later used to fetch new images
-- automatically (ADR 0027). The query is JSON (searches.Query).
CREATE TABLE saved_searches (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(trim(name)) > 0),
    query      TEXT NOT NULL CHECK (json_valid(query)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;
