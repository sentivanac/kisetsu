# ADR 0006: Wallhaven provider

- **Status:** accepted
- **Date:** 2026-10-30

## Context

Wallhaven is the first provider. Its API (https://wallhaven.cc/help/api)
needs no key for safe-for-work searches, returns 24 results per page and
allows 45 API calls per minute per client. The limit is reported in the
`x-ratelimit-limit` / `x-ratelimit-remaining` response headers; exceeding it
returns `429`.

Verified against the live API on 2026-10-30:

- `ratios=landscape` and `ratios=portrait` work, although the documentation
  only lists explicit ratios such as `16x9`. There is no keyword for square.
- `atleast=WxH` filters by minimum resolution.
- Search results contain no author. Only the uploader is available, through
  one extra request per image, and the uploader is usually not the author.
- `source` is free text supplied by the uploader, not always a URL.
- Wallhaven gives no license: "All images remain property of their
  original owners."

## Decision

- **Safe-for-work only:** `purity=100` is always sent, and any result whose
  `purity` is not `sfw` is dropped.
- `categories=111` (general, anime, people).
- `sorting=relevance` for text searches, `toplist` when browsing without text.
- Orientation maps to `ratios=landscape|portrait`; square and any other
  filter are applied locally with `Query.Matches`.
- **Client-side rate limit of 40 requests per minute** (burst 4) with
  `golang.org/x/time/rate`, below Wallhaven's 45 because that limit is
  shared by everything on the user's IP address. Waiting respects
  cancellation. The limit matters for fast scrolling and, later, for query
  expansion, where one search becomes several API calls. Wallpaper rotation
  uses the local library and does not depend on it.
- `License.Status = unknown` with Wallhaven's statement as the notice;
  `Author` stays empty; `SourceURL` is kept only if it is an http(s) URL.
- Responses larger than 2 MB are rejected (a page is about 30 KB).
- Status mapping: `429` → `ErrRateLimited` (with `Retry-After`, default 60 s);
  `400`/`422` → `ErrInvalidQuery`; everything else, network failures and
  malformed JSON → `ErrUnavailable`.

## Testing

- Unit tests use `httptest` and `testdata/search.json`, a trimmed real
  response plus crafted entries (sketchy, missing download URL, non-URL
  source). They never use the network.
- `make test-live` runs one search against the real API (build tag `live`)
  to detect API changes. It is not part of `make check`.

## Not done yet

- API key support (would go in settings, never in the repository).
- Using `x-ratelimit-remaining` to adapt the limiter.
