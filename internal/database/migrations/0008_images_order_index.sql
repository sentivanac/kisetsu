-- Library pages are ordered by when images were added. Without this
-- index SQLite sorted every image of an origin for each page: 140 ms per
-- page with 10 000 images (ADR 0035).
CREATE INDEX images_origin_added ON images (origin, added_at, id);
