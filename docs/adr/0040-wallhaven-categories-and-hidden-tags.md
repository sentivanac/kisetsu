# ADR 0040: Wallhaven categories and hidden tags

## Status

Accepted (2026-11-04).

## Context

Kisetsu asks Wallhaven only for images rated safe for work (ADR 0006).
The user still got suggestive pictures ("barefoot" and similar): Wallhaven's
SFW rating allows them, and most come from its Anime and People
categories. Leaving out general words such as "woman" would hide far too
much. Search results do not carry tags; Wallhaven sends them only with
the details of one image (`GET /api/v1/w/{id}`).

Checked against the live API (2026-11-04): `q=girl` gave 273,771 results,
`girl -barefoot` 267,630; `girl -long hair` excluded only "long"
(95,772), while `girl -"long hair"` excluded the tag (190,008).

## Decision

1. **Categories.** `providers.Query.Categories` (general, anime, people;
   empty means all) becomes Wallhaven's `categories` bits. Providers that
   apply it report it (`providers.Categorizer`), and the search form shows
   the three checkboxes only when such a source is selected. At least one
   stays on. The choice is part of the remembered filters and of saved
   searches, so Fetched images and rotation follow it.
2. **Hidden tags.** A list in the settings (`search.hiddenTags`, at most
   100 tags of up to 64 characters, compared case-insensitively). The
   search service puts it into every query (`Query.HiddenTags`), always
   from the settings, never from the caller; if it cannot be read, the
   search runs without it. Wallhaven sends each as `-tag`, or
   `-"several words"`. Sources without tags ignore it.
3. **Tags in the preview.** For sources with tags (`providers.Tagger`),
   the preview loads the image's tags 0.4 s after it is shown (paging
   quickly costs no API calls) and keeps them while the preview is open.
   Clicking a tag hides it; clicking a hidden one shows it again. The
   search form lists hidden tags with a button to show each again.

## Consequences

- Only tagged images are caught; untagged suggestive pictures still
  appear. Local image classification (Phase 5) could do better.
- One extra Wallhaven call per previewed image, through the existing rate
  limiter (40 per minute).
- Hidden tags apply to Wallhaven only; Wikimedia Commons has no tags.
