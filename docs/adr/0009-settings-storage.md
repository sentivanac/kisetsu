# ADR 0009: Storing user settings

- **Status:** accepted
- **Date:** 2026-10-30
- Resolves the open question in ADR 0002 ("a settings file format").

## Context

The first real setting is the set of search filters (source, minimum
resolution, orientation), remembered between sessions.

## Decision

1. **Settings live in SQLite**, table `settings (key, value, updated_at)`
   created by migration `0001_settings.sql`, not in a separate file.
   - One storage location, atomic writes, schema migrations already exist,
     and CLAUDE.md lists user preferences as database content.
   - Cost: settings are not hand-editable in a text editor.
2. Values are JSON (`CHECK (json_valid(value))`, `STRICT` table), accessed
   through `internal/settings.Store` (`Load`/`Save`) and typed helpers
   such as `SearchFilters`.
3. **A bad stored setting never breaks the app.** Values that cannot be
   decoded or fail validation (for example written by another Kisetsu
   version) fall back to defaults. Only database failures are errors.
4. **The search text is not stored** with the filters. It is search history,
   which is private and will be a separate, explicit feature.
5. The frontend saves filters when they change, but only after the
   remembered filters were loaded, so startup defaults cannot overwrite them.
   A remembered provider that no longer exists falls back to the first one.

### "Use my screen size"

Sets the minimum resolution to the **physical** pixel size of the screen the
window is on (Wails `ScreenGetAll`, `physicalSize` = logical size × GTK
scale factor), because a wallpaper must fill device pixels: a 200% HiDPI
screen of 1920 × 1080 logical pixels needs 3840 × 2160 images. The Wails
v2.17 TypeScript type lacks `size`/`physicalSize`; `src/lib/screen.ts`
declares them.

It also sets the **orientation** to the screen's (landscape, or portrait
for a monitor turned on its side; a square screen leaves it open), added
2026-11-02 at the user's suggestion: a minimum size alone lets a
2000 × 3000 portrait image through for a 1920 × 1080 screen, and it cannot
fill that screen.

## Limitations

- GTK reports an integer scale factor, so with fractional scaling (e.g. 125%)
  the physical size may be inaccurate.
- `App` now receives its dependencies through a `Deps` struct.
