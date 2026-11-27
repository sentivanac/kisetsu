# ADR 0058: Styles on the wallpaper

- **Status:** accepted, for 0.10.0
- **Date:** 2026-11-27

## Context

The effects of ADR 0050 keep text legible. Variety also offers filters
that change how the picture looks (oil painting, pixellate, emboss…), and
Kisetsu already composes a copy of every wallpaper, so such filters cost
one more pass over that copy. Sample images of candidate styles on two
wallpapers were compared before any of this was written.

## Decision

One new choice, **Style**, first under Effects in the Wallpaper tab, off
by default:

| Style | What it does | Size (on a 1080 px screen) |
| --- | --- | --- |
| Oil painting | Kuwahara filter, then a light relief like canvas | brush radius 4, 7 or 11 px |
| Mosaic | tiles of their mean colour, with a dark joint of 1 px | tiles of 12, 24 or 48 px |
| Neon edges | Sobel edges in the picture's colours, on black | lines: sampled 1, 2 or 4 px apart |
| Pencil | the same edges, dark on paper | as neon |
| ASCII | ` .:-=+*#%@` in Go Mono, in the cell's colour on black | letters of 10, 16 or 24 px |
| Emboss | the picture's colours shaded as a relief lit from the top left | depth: 1, 2 or 4 px |

- **Only one style at a time**: they redraw the whole picture, and two
  together make no sense.
- **Size** has three levels, named after what it changes in that style
  (Brush, Tiles, Letters, Lines, Depth), stored as shares of the screen
  height so a style looks alike on any screen. **Strength** (100, 75 or
  50 %) mixes the original back in. No sliders, as in ADR 0050.
- **First in the chain**, before blur, colours, darkening and the
  vignette (ADR 0050), and before text. So Colors turns any style black
  and white or sepia: a separate one-colour ASCII or grey emboss is not
  needed (both were in the samples).
- **ASCII** ranks the cells by brightness and spreads the ranks over the
  characters: a picture of middle tones otherwise became one character
  (seen in the samples). Each character takes its cell's colour brought
  to full brightness (at most 2.2 times), since its density already shows
  how bright the cell is. Go Mono is already in Kisetsu for the text
  file (ADR 0051).
- **Oil painting** keeps statistics of the filter's squares for only r+1
  rows at a time, from running sums, so memory stays small and the cost
  does not depend on the brush size.
- Everything is split between the processor's cores, like the blur.
- Settings are stored with the other wallpaper settings (`style`,
  `styleSize`, `styleMix`); settings from earlier versions load with no
  style, and earlier versions ignore the new fields.

## Measured

On an Intel i7-10510U (4 cores, 8 threads), a 3840×2160 image, medium
size (`go test -bench Styles ./internal/overlay/`):

| Style | Time |
| --- | --- |
| Mosaic | 0.03 s |
| ASCII | 0.04 s |
| Emboss | 0.09 s |
| Oil painting | 0.37 s (1.24 s on one core) |
| Neon edges, pencil | 0.5 s, mostly the blur before finding edges |

A 1920×1080 screen has a quarter of the pixels. A style is computed once
per wallpaper; the clock redraws only the text (ADR 0047).

## Verification

Unit tests for every style on a plain image; the oil filter keeps a hard
edge and evens out a checkerboard; a mosaic tile's mean and its joints;
neon and pencil at an edge and away from it; the side of the emboss
facing the light; ASCII draws only on the bright half of an image;
strength 50 %; validation of the new settings and loading those of
0.9.0. Run with the race detector, since cells and rows are drawn in
parallel. The styles were rendered from two wallpapers with the real code
and compared with the samples.

## Alternatives considered

- **Half-block characters** (`▀`, two colours per cell) for ASCII: on an
  image rather than a terminal, they look exactly like a mosaic.
- **One-colour ASCII and grey emboss as styles of their own:** Colors
  gives both.
- **Sliders for size and strength:** more choice than a wallpaper needs.
- **More styles at once** (pointillism, posterize, watercolour): the
  first round is these six; others can follow when they prove worth it.
