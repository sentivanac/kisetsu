# ADR 0008: Gallery, preview and WebView GPU policy

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Search results must be browsable as images. Measured Wallhaven thumbnails
(2026-10-30) for a 5070 × 3380 image:

| Thumbnail | Size | Shape |
|---|---|---|
| `small` | 300 × 200, 24 KB | cropped to 3:2 |
| `large` | 432 × 243, 26 KB | cropped to 16:9 |
| `original` | 300 × 200, 24 KB | original aspect ratio |
| full file | 5070 × 3380, 1.2 MB | |

No thumbnail is large enough for a full-window preview.

## Decision

### Gallery

- Responsive CSS grid (`auto-fill`, min 240 px columns) of `small`
  thumbnails in 3:2 cells, matching their shape.
- `loading="lazy"` and fixed `width`/`height`, so only visible thumbnails
  load and the layout does not jump.
- **"Load more" button** for the next page instead of infinite scrolling:
  simpler, keyboard friendly and every API call is a deliberate action.
  Infinite scrolling may come later as an option. Pages are merged with
  duplicates removed.

### Preview

- Native `<dialog>` (focus trap, Escape to close); ←/→ move between results.
- The uncropped `original` thumbnail (`Image.PreviewURL`) is shown at once,
  blurred, and the full file (`DownloadURL`) fades in when loaded. **The full
  file is only fetched when a preview is opened**, never for the whole grid.
- Shows size, file type and size, source, author ("Unknown" when unknown),
  license and the provider's license notice.
- External links (provider page, original source, author, license) open in
  the system browser with Wails' `BrowserOpenURL`. A normal link would
  navigate the application window away from Kisetsu.

### WebView GPU policy

Wails disables hardware acceleration on Linux when `options.Linux` is not
set, because of blank windows on some systems
(https://github.com/wailsapp/wails/issues/2977). Image grids scroll much
better with acceleration, so Kisetsu uses **`ondemand`** (WebKit's default)
and allows `KISETSU_WEBVIEW_GPU=never|always|ondemand` as a workaround.

## Limitations

- Components (`Gallery`, `PreviewDialog`, `SearchPanel`) have no automated
  tests yet; their helpers in `src/lib/images.ts` do.
- Thumbnails and previews are loaded directly from the provider's servers,
  which therefore see the user's IP address, as with any web search.
- No download button yet (Phase 4).
