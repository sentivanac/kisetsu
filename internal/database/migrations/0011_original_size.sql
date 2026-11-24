-- The size of the original of an image. A file reduced to screen size is
-- smaller than that; with the same size, the file is the original. NULL is
-- unknown: images stored before this version may be either.
ALTER TABLE images ADD COLUMN original_width INTEGER CHECK (original_width > 0);
ALTER TABLE images ADD COLUMN original_height INTEGER CHECK (original_height > 0);
