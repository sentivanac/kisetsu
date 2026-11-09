# ADR 0049: Search suggestions with synonyms

- **Status:** accepted
- **Date:** 2026-11-09

## Context

0.5.0 was to add synonyms to searches: "autumn" should also find "fall"
and "foliage". The plan was to expand the query itself. Checked against
the live APIs on 2026-11-09, none of the sources can search several
words as alternatives in one request:

| Source | Search | Results |
| --- | --- | --- |
| Wallhaven | `autumn` / `autumn foliage` | 4638 / 71: every word narrows |
| Wikimedia Commons | `autumn forest` / `(autumn OR fall) forest` | 908 / 82: no grouping, `OR` binds oddly |
| Openverse | `autumn` / `autumn\|fall` | 240 (the most shown) / 22: `\|` is no alternative |

Searching every synonym separately and merging the results would cost
two or three times as many requests: Openverse allows 200 a day without
a key, Wallhaven 45 a minute.

## Decision

Kisetsu **suggests** searches instead of changing them. Under the
results, "Also try:" lists the search with one word or phrase replaced
by a synonym; a click runs it with the same sources and filters, and the
search field shows what is searched.

- Package `internal/synonyms`, with a small list written by hand:
  groups of words and two-word phrases that mean about the same for a
  wallpaper (seasons, weather, sky, landscape, places, a few animals and
  things), and British and American spellings ("grey", "gray"). Every
  member of a group suggests the others; a word belongs to one group.
- The longest match wins: "night sky" is replaced as a phrase, not as
  "night".
- Excluded words (`-snow`) are kept and never replaced. The user's other
  words keep their spelling.
- At most 6 suggestions, in the order the words appear.
- No request is sent for a suggestion until it is clicked.

`App.SuggestSearches` returns the suggestions; the search panel asks for
them when a search starts.

## Limitations

- English only, like the sources' tags and most of their descriptions.
- The list is short and general; a missing word simply gives no
  suggestion.
- A word with several meanings ("fall") may suggest a search the user
  did not mean; it is only a suggestion.

## Alternatives considered

- **Expanding the query:** not possible with these sources (see Context).
- **Searching the synonyms automatically and merging:** more results,
  but several times the requests, and results the user did not ask for.
- **Only when a search finds little:** fewer extra requests, but harder
  to predict.
