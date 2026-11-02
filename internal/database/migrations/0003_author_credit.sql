-- The provider's full author text, when it says more than the name
-- (for example a photographer's conditions of use). See ADR 0020.
ALTER TABLE images ADD COLUMN author_credit TEXT NOT NULL DEFAULT '';

-- Before this column, Commons credit texts were stored as the author name.
-- Names over 100 characters are such texts: keep them in full as the
-- credit. The name cannot be recovered without the original HTML.
UPDATE images
SET author_credit = author_name, author_name = ''
WHERE length(author_name) > 100;
