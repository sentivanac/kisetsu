# ADR 0011: Wikimedia Commons provider

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Wikimedia Commons is the second provider: no API key, real licenses and
authors for every file, and many historical photographs and artworks.
Verified against the live API on 2026-10-30:

- `generator=search` + `prop=imageinfo|categories` returns everything in one
  request. Results are unordered; `index` is the search rank.
- CirrusSearch filters `filetype:bitmap`, `filew:>N`, `fileh:>N`,
  `incategory:A|B` and `-term` all work server side.
- Searches are limited to **300 characters**, not counting `filetype:`.
- `Artist` and `ImageDescription` are HTML, sometimes lists with links to
  missing user pages (`redlink=1`).
- Thumbnail URLs can be rewritten to other widths (`/500px-` → `/1280px-`);
  both return HTTP 200.
- **Commons has no safe-for-work flag.** `-deepcategory:Nudity` only applies
  a subset of categories (the server says so), and `nude -deepcategory:Nudity`
  still returns 47 190 files.

## Decision

### Content safety (user decision)

Best effort, clearly communicated, off by default:

1. Searches are limited to `Quality_images|Featured_pictures_on_Wikimedia_Commons`
   (community-reviewed, high resolution). "mountains" still returns 32 379
   files.
2. `-nude -naked -nudity -erotic` is added to every search (kept short
   because of the 300-character limit). Within the curated collections this
   reduced an explicit "nude" search from 232 files to 0.
3. Results whose title, description or (non-hidden) categories contain a
   risky word are dropped locally (`sfw.go`, matched per word so "Essex"
   does not match "sex").
4. The source is **off by default**; the UI will say that the filter is not
   perfect. A local image classifier may follow in Phase 5.

### Mapping

- ID `wikimedia-commons`; `ProviderID` is the page ID.
- Thumbnail 500 px (grid), preview 1280 px, download = original file.
  Images narrower than the requested width use the original (Commons does
  not upscale). Only the width prefix of the URL is rewritten, so its
  encoding stays exactly as Commons sent it.
- License `known` with `LicenseShortName` and `LicenseUrl`; when
  `AttributionRequired` is true the notice says so. Files without license
  metadata are `unknown` and the notice asks the user to check the page.
- Author: visible text of `Artist` and its first link to an existing page,
  parsed with `golang.org/x/net/html` (never shown as markup).
- Only JPEG, PNG and WebP are returned; TIFF and other formats cannot be
  shown by the WebView.
- Orientation is filtered locally.

### Requests

- Pages map to `gsroffset = (page − 1) × 24`; CirrusSearch stops at offset
  10 000, so deeper pages are empty without a request. `HasMore` follows
  `continue.gsroffset`.
- `maxlag=5`, a descriptive User-Agent with a contact URL (required by the
  Wikimedia User-Agent policy) and a client-side limit of 2 requests per
  second.
- Errors: API `maxlag` → `ErrUnavailable` with `Retry-After`;
  `ratelimited` and HTTP 429 → `ErrRateLimited`; too-long query →
  `ErrInvalidQuery` (also checked before sending); anything else →
  `ErrUnavailable`.

## Limitations

- Long natural-language queries (over roughly 180 characters of user text)
  exceed Commons' limit and are rejected for this source only.
- The SFW filter is best effort and cannot catch files whose metadata does
  not describe their content.
- Categories of a page may be incomplete if a response needs category
  continuation; the local check then sees fewer categories.
- The provider is implemented and tested but not yet registered in
  `main.go`; it is enabled together with source selection in the UI.
  (Registered in ADR 0014, off by default through `providers.ContentWarner`.)
