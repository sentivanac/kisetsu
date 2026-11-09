# ADR 0050: Effects on the wallpaper

- **Status:** accepted
- **Date:** 2026-11-09

## Context

Text on the wallpaper (ADR 0047) and desktop icons are hard to read on
bright, busy images; white text on a white sky shows only its shadow.
Kisetsu already composes a copy of the image for the screen, so effects
on the whole image cost little more.

## Decision

Four effects, each off by default, in the Wallpaper tab under "On the
wallpaper":

| Effect | Levels | What it does |
| --- | --- | --- |
| Darken | off, light, medium, strong | takes 20, 35 or 50 % of the brightness |
| Blur | off, light, medium, strong | a radius of 0.2, 0.5 or 1.2 % of the screen height (2, 5 or 13 px at 1080 px) |
| Colors | original, black and white, sepia | Rec. 709 luma; sepia colors it brown, from black to cream (255, 224, 176) |
| Vignette | on, off | darkens towards the corners, by half at the corners themselves |

They are applied in a fixed order (blur, colors, darken, vignette), to
the base: the image already cropped and scaled to the screen (ADR 0047),
before any text. Effects alone, without text, also
make Kisetsu set a composed image.

- **Once per image.** The agent keeps the base for the clock; the
  effects are part of it, so a new minute costs what it did. The kept
  base is computed again when the effects change.
- **The blur** is three box blurs along rows and then columns, which
  approximate a Gaussian blur at a cost independent of the radius. Both
  passes read memory in order, and the work is split between the
  processor cores.
- **Package `internal/overlay`** has the effects (`Effects`, with shares
  of the screen height and of brightness); `internal/onscreen` turns the
  stored levels into them. Settings stored before 0.6.0 load with every
  effect off.

## Measured

On an Intel i7-10510U (4 cores, 8 threads), for a 4K image:

| Effect | Time |
| --- | --- |
| Blur, medium | 0.27 s (1.07 s on one core) |
| Vignette | 0.16 s |
| Black and white, sepia, darken | 0.06 s each |
| All four | 0.53 s |

A 1920×1080 screen has a quarter of the pixels.

## Verification

Unit tests for each effect (a plain image stays plain under blur, a hard
edge becomes a symmetric gradient; darkening by half; red in black and
white; black and white in sepia; the vignette leaves the middle and
darkens the corners), for effects alone composing an image, for settings
from before 0.6.0, and for a new base when the effects change. In the window, each effect saved
and shown the current wallpaper again at once.

## Alternatives considered

- **A band behind the text** instead of effects: covers part of the
  picture (rejected in ADR 0047 as well).
- **Blurring a scaled-down copy and scaling it back up:** faster for
  strong blurs, but blocky at the light levels; the exact blur is fast
  enough.
- **Black and white and sepia as two switches:** they exclude each
  other, so they are one choice, "Colors".
- **A strength slider per effect:** more choice than a wallpaper needs;
  four levels are enough to read text.
