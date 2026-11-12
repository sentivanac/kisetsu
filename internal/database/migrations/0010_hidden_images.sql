-- Images the user does not want to see again as the wallpaper (ADR 0054).
-- They stay in the library or the folder; rotation and Previous skip
-- them. Deleting or forgetting the image removes the row with it.
CREATE TABLE hidden_images (
    image_id  INTEGER PRIMARY KEY REFERENCES images (id) ON DELETE CASCADE,
    hidden_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;
