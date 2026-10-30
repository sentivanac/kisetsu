# Database migrations

SQL files in this directory are embedded into the binary and applied in
order when Kisetsu starts.

Rules:

- Name files `NNNN_description.sql`, numbered from `0001` without gaps,
  using lowercase letters, digits and underscores.
- Migrations are forward-only. Never edit or delete a migration that has
  been released; add a new one instead.
- Each file runs in a single transaction together with the schema version
  update (`PRAGMA user_version`). Do not put `BEGIN`/`COMMIT` in the file.
- Never drop or rewrite user data without a migration path that keeps it.
