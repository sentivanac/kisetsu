# ADR 0048: Openverse provider

- **Status:** accepted
- **Date:** 2026-11-09

## Context

0.5.0 adds a source without AI (see `docs/ideas.md`). The candidates,
checked against their documentation and the live APIs on 2026-11-06 and
2026-11-09:

- **Pexels** and **Unsplash** forbid exactly this kind of application in
  their API terms. Pexels: "You may not copy or replicate core
  functionality of Pexels (including making Pexels content available as a
  wallpaper app)." Unsplash: "You cannot replicate the core user
  experience of Unsplash (unofficial clients, wallpaper applications,
  etc.)". Neither is an option, with or without the user's own key.
- **Pixabay** returns at most 1280 px without approval for "full API
  access"; too small for a wallpaper.
- **Openverse** (run by the WordPress Foundation) indexes about 915
  million openly licensed images from 52 sources, always with creator and
  license, and needs no key.

Openverse is less useful for wallpapers than its size suggests. Its
`url` field is not always the original file:

| Source | Images | What `url` gives |
| --- | --- | --- |
| Flickr | 536 M | 1024 px (`_b` size), whatever the original |
| iNaturalist, NASA, Katmai | 270 M | 1024 px or smaller (also through Flickr) |
| Wikimedia Commons | 89 M | the original; Kisetsu already has Commons |
| StockSnap, Rawpixel | 1.3 M | 960 or 1024 px, while `width`/`height` give the original |
| SMK | 53 k | at most 2048 px, while `width`/`height` give the original |
| Met | 498 k | the original, but `width`/`height` are empty |
| Rijksmuseum, Museums Victoria, WP Photo Directory | 260 k | the original (2000 to 3000 px), sizes correct |
| Smithsonian, Wellcome, Europeana, others | | sizes empty, or small files |

Checked by downloading 5 to 20 results per source and comparing the
file with the metadata.

Limits for requests without a key: 20 per minute and 200 per day per IP
address, 1000 thumbnails per day, at most 20 results per page and 240
per search (12 pages). Registering an application gives "slightly higher
limits", but needs a client secret, which an open-source desktop program
cannot keep secret; openverse.org itself works without a key.

The terms of service forbid scraping and circumventing the rate limits,
require attribution for CC-licensed works, and ask applications to say
that they use Openverse without being endorsed by it.

## Decision

A provider `openverse` (shown as **Openverse**), off by default, that
searches only sources whose files are large enough for a wallpaper and
whose sizes are correct.

### Sources

`source=rijksmuseum,museumsvictoria,wordpress,smk`, kept in one list in
the code:

- **Rijksmuseum, Museums Victoria, WP Photo Directory**: original files,
  correct sizes.
- **SMK**: the size is capped at 2048 px on the longer side, scaled
  proportionally, since that is what the file has.
- **Not included:**
  - Flickr and the sources served through it, StockSnap and Rawpixel
    (previews only);
  - Wikimedia (already a source of its own, so not twice);
  - the Met, whose files are excellent but whose results have no size.
    Kisetsu needs the size before download (filters, orientation, the
    "fits your screen" mark). Reading it from the start of each file
    would add 20 requests to the Met per page; left for later.

A source Openverse adds or drops changes nothing until the list in the
code changes. The library always stores the size of the downloaded file,
so a wrong size in the metadata is corrected on download.

### Mapping

- ID `openverse`; `ProviderID` is Openverse's UUID.
- Page: the image's page on openverse.org ("Open on Openverse");
  `SourceURL`: `foreign_landing_url`, the work's page at the museum or
  the photo directory ("Original source").
- Thumbnail and preview: Openverse's `thumbnail` (600 px), shown while
  the file loads. Download: `url`. (With the file as the preview too,
  the preview stayed blurred: placeholder and full image had the same
  address.)
- Author: `creator` and `creator_url`; empty when missing, never guessed.
- License: `license` and `license_version` → "CC BY-SA 4.0", "CC0 1.0",
  "Public Domain Mark 1.0"; `license_url` as the link. Every Openverse
  result has a license, so the status is always `known`. Non-commercial
  and no-derivatives licenses are kept: a wallpaper is personal use, and
  the library shows the license.
- Only JPEG, PNG and WebP (`filetype`), as for Commons.
- Results without a size are dropped.

### Filters

- `q` is the search text; excluded words are sent as `-word`, which
  Openverse applies itself ("winter -snow": 16 of 20 results mentioned
  snow without it, 2 with it). Searches are limited to 200 characters.
- Orientation → `aspect_ratio=wide|tall|square`, and checked locally as
  well, since Openverse does not document where its classes end.
- Minimum size is checked locally. Openverse's `size=large` is not used:
  it does not say what "large" means.
- Categories and hidden tags do not apply.

### Content

Museums hold paintings and photographs of nudes. `mature=false` (the
default) drops what Openverse has flagged, and the local word check of
the Commons provider (title and tags) also applies. Like Commons, the
source is off until the user turns it on, with the same notice that the
filter is not perfect.

### Requests

- 20 results per page; `HasMore` while `page < page_count`. Pages after
  `page_count` (at most 12) are not requested.
- The User-Agent names Kisetsu and its address, as for the other
  sources.
- A client-side limit of one request every 4 seconds, with a burst of
  3, keeps Kisetsu under 20 a minute.
- HTTP 429 → `ErrRateLimited`, with `Retry-After` when sent; Kisetsu
  never retries on its own. Other errors → `ErrUnavailable`, with
  Openverse's explanation (it answers a page past the limit with HTTP
  401 and "pagination depth may not exceed 240").
- No key, no registration.

The risky-word check moves from the Commons provider to the `providers`
package (`providers.LooksUnsafe`), shared by both.

## Verification

Unit tests against a fixture made from a live response (five real
results and five crafted ones: no size, TIFF, a risky tag, SMK with the
original's size, a `javascript:` URL), for the request parameters, the
local filters, paging, errors and license names. A live test
(`make test-live`) searches "harbour -night" with a minimum width and
landscape orientation and loads the first thumbnail and file.

### Credit

The README says that Kisetsu searches Openverse and is not endorsed by
it. On the wallpaper and in the library, the source is shown as
"Openverse", the author and license as for every image.

## Limitations

- 200 searches a day per IP address, shared with everything else on the
  same network that uses Openverse anonymously. Rotation from a saved
  search uses a few of them when it fetches new images.
- Many results are museum objects (coins, prints, furniture) rather than
  views; the orientation and size filters remove some of them.
- Openverse's ranking favours the WP Photo Directory for many searches;
  museum images often come on later pages.
- The Met is not searched (sizes missing).

## Alternatives considered

- **Openverse with all sources:** mostly 1024 px results from Flickr.
- **Probing image sizes with range requests:** would let the Met in, but
  adds a request per result to the museum's servers. Possible later.
- **Registering Kisetsu with Openverse:** the client secret would be in
  the public source code.
- **Pexels, Unsplash, Pixabay:** see Context.
