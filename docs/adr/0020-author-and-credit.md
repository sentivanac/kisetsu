# ADR 0020: Author name and credit note

- **Status:** accepted
- **Date:** 2026-11-02
- Amends the author mapping of ADR 0011.

## Context

Many Commons photographers fill the Artist field with a template:
icons, a link to the file itself, the author's user page, a link for
messages, and their conditions of use. Example (`File:Giorgio_Moroder.jpg`,
saved in `internal/providers/commons/testdata/artist_moroder.html`):

> This Photo was taken by Wolfgang Moroder. Feel free to use my photos, but
> please mention me as the author and send me a message. … This
> specifically excludes use in social media …

Two problems followed:

1. The whole text was shown as the author's name.
2. The author link was the first link in the HTML, which here is a camera
   icon (`File:Emblem-camera.svg`), not the photographer.

Shortening the text is not an option: it holds the author's conditions of
use, and attribution must never be discarded.

## Decision

- `images.Author` gains **`Credit`**: the provider's full author text when
  it says more than the name. It is never shortened.
- `parseArtist` (Commons):
  - the **name** is the text of the first link to a person: a Commons
    `User:` or `Creator:` page, a Wikidata item, or a Wikipedia article.
    Links to files, templates, talk pages and the image itself are skipped;
  - the **profile URL** is that link, unless the page does not exist
    (redlink), in which case there is no URL rather than someone else's;
  - without such a link, text of up to 100 characters is the name
    ("Unknown author"); longer text is only a credit;
  - whenever the name is not the whole text, the whole text is the credit.
    "Original: Florian Kriechbaumer Derivative work: Aristeas" thus becomes
    the name "Florian Kriechbaumer" with the full text as credit.
- Database: migration `0003_author_credit.sql` adds `images.author_credit`
  and moves author names longer than 100 characters — credit texts stored
  before this change — into it, in full. Their name stays empty, because
  it cannot be recovered without the original HTML.
- Preview: "Author" shows the name (or "See the author's note below");
  the credit is a collapsed **Author's note** (`<details>`), scrollable
  when long.

## Lessons

`golang.org/x/net/html.Tokenizer.Text` consumes the token's text: calling
it twice returns nothing the second time. A comment in `parseHTML` notes it.
