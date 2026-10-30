# ADR 0010: Excluded search terms

- **Status:** accepted
- **Date:** 2026-10-30
- Extends `providers.Query` from ADR 0005.

## Context

Users exclude unwanted results by writing `-term` (for example
`mountains -snow`). Until now this only worked because Wallhaven received
the raw text; other providers may use a different syntax or none at all.

## Decision

- `Query.Exclude []string` holds excluded terms without the leading `-`.
- `Query.Normalize()` moves `-term` words from `Text` into `Exclude`:
  - words with an inner hyphen (`sci-fi`) stay in `Text`;
  - a lone `-` is dropped, repeated hyphens (`--snow`) are stripped;
  - duplicates are removed case-insensitively; the first spelling wins;
  - whitespace is collapsed; the operation is idempotent.
- `Validate` allows at most 16 excluded terms of at most 64 characters,
  each a single word.
- `search.Service` normalizes every query before validating it and
  calling a provider; providers also normalize, so direct callers work.
- Each provider translates `Exclude` into its own syntax. Wallhaven and
  Wikimedia Commons (CirrusSearch) both accept `-term`, verified against
  their live APIs (`mountains` → `mountains -snow`: 21 847 → 16 199 results
  on Wallhaven, 32 379 → 30 549 on Commons quality images).
- A provider without exclusion support must say so in its documentation;
  `Query.Matches` cannot apply exclusions locally because results carry no
  tags.

## Limitations

- Quoted phrases (`-"snow storm"`) are not supported.
- Wallhaven's other operators (`+tag`, `@user`, `id:`) pass through in
  `Text` unchanged and only work there.
