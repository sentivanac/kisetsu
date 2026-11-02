-- Every wallpaper Kisetsu set, for rotation without repeats (ADR 0023).
-- A rotation "cycle" ends when every image of the source has been shown.
CREATE TABLE wallpaper_history (
    id       INTEGER PRIMARY KEY,
    image_id INTEGER NOT NULL REFERENCES images (id) ON DELETE CASCADE,
    cycle    INTEGER NOT NULL CHECK (cycle >= 0),
    reason   TEXT    NOT NULL CHECK (reason IN ('rotation', 'manual')),
    shown_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX wallpaper_history_cycle ON wallpaper_history (cycle);
