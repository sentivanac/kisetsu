# ADR 0034: Pre-release review for 0.1.0

- **Status:** accepted
- **Date:** 2026-11-02
- Fourth step of Phase 7.

## Findings and what was done

| # | Finding | Done |
|---|---|---|
| 1 | The package shipped only Kisetsu's license; the MIT/BSD/Apache licenses of the 23 Go modules and 3 npm packages built in require their notices too | `THIRD_PARTY_LICENSES.gz`, generated from what the binary actually contains (ADR 0033) |
| 2 | After uninstalling, the rotation timer would start a missing program at every interval | `ConditionFileIsExecutable` in the service (ADR 0025) |
| 3 | The log grew without limit (window and every rotation run append) | new log above 5 MB, previous kept as `.1` (ADR 0002 addendum) |
| 4 | First start: empty Search tab with no hint; "Next wallpaper" enabled with no images; footer showed developer details | see below |
| 5 | The WebView had no Content-Security-Policy | see below |

Left for later: a README screenshot (with images whose license allows it,
e.g. from Wikimedia Commons with credit), a manual page, more tests in
`main` (71 %), `schedule` (77 %) and `wallpaper` (79 %).

Checked and fine: SQLite uses WAL with a 5 s busy timeout, so the window
and the timer can use the database together; history and library queries
are indexed or paged; no HTML from providers is inserted (`innerHTML` is
not used); HTTP requests have timeouts.

## First start

- The Search tab explains how to search before the first search, and
  that searching without words shows popular images. "Save this search"
  stays visible: an empty search ("Popular images") is a valid saved
  search.
- "Next wallpaper" is disabled when the chosen source has no images, with
  a note on how to get some (an empty collection says so). A saved search
  is never disabled: it downloads images when it has none.
- The footer shows only name and version; platform, schema version and
  database path are in its tooltip, for bug reports.

## Content-Security-Policy

Added to `index.html` of the production build by a small Vite plugin
(`frontend/vite.config.ts`), not under `wails dev`, where Vite needs
inline scripts and a websocket:

```
default-src 'self'; script-src 'self'; style-src 'self';
img-src 'self' https: data: blob:; connect-src 'self';
object-src 'none'; base-uri 'none'; form-action 'none'
```

Scripts and styles come only from the app; images also from providers
over HTTPS (thumbnails and previews are loaded by the WebView). Calls to
Go do not go through `fetch`, so `connect-src 'self'` does not affect
them. The bundle has no inline scripts or `style=` attributes.

## Verification

The release binary was started on a hidden X display (Xvfb) with empty
temporary XDG directories and no `gsettings` in `PATH`: the first-start
texts appear, a live Wallhaven search shows thumbnails, the preview opens,
a download lands in the temporary library and its thumbnail (served from
`/library/`) is shown — all under the CSP. Screenshots were taken of the
hidden display only.
