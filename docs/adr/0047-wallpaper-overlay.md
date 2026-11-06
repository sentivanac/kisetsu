# ADR 0047: Text on the wallpaper

## Status

Accepted (2026-11-05), for 0.4.0: the image's author and license, and a
clock, each off by default.

## Context

Users of Variety know its clock on the wallpaper. A separate window on
the desktop is not an option: GNOME on Wayland does not allow one, and
conky already does that job. Kisetsu has what a composed wallpaper needs:
the background agent (ADR 0041) runs all session and knows the current
image, and every image in the library carries its author and license.

## Decision

Kisetsu composes a new image, the wallpaper with the text drawn on it,
and sets that instead of the original, as Variety does. Package
`internal/overlay` does the drawing; it knows nothing about the library,
the settings or the desktop.

- **Exactly the screen size.** `Base` scales the image to cover the
  stored screen size and crops it around the centre, like the "Zoom" or
  "Fill" picture option. The text then lands in the chosen corner
  whatever option the desktop uses. While text is shown, "Fit" (with
  bars) has no effect.
- **Once per image, then once per minute.** `Base` is computed when the
  wallpaper changes; `Draw` copies it and draws the text. Fetched images
  are already reduced to cover the screen, so usually `Base` only crops.
- **Corners and margins.** Text goes in one of four corners; blocks in
  the same corner are stacked. The vertical margin (7 % of the screen
  height) keeps the text clear of panels and the Windows taskbar, which
  are drawn over the wallpaper. A line wider than 60 % of the screen is
  shortened with "…".
- **Legibility.** White text with a soft dark shadow, legible on light
  and dark images without a band across the picture.
- **Font.** Go Regular, from `golang.org/x/image` (BSD licensed, already
  a dependency): no new module and no system fonts, so the result is the
  same on every desktop. It covers Latin, Greek and Cyrillic;
  `Drawable` tells the caller whether a text would show empty boxes (for
  example Japanese names), so it can leave that text out.
- **JPEG at quality 90**: small to write every minute, and every desktop
  can show it.

The original file and the wallpaper history are never changed.

### Where it is used

Package `internal/onscreen` is the one way a library image reaches the
desktop: "Set as wallpaper" in the window, rotation, the agent and
`kisetsu rotate` all call `Show`. It reads the settings (stored under
`overlay`, everything off by default) and:

- sets the original when no text is turned on, or when the image would
  go to Variety (Variety applies its own effects and clock);
- otherwise composes the image for the stored screen size and sets that;
- sets the original when composing fails (an unreadable file, a full
  disk): text is never a reason for the wallpaper not to change. The
  problem is logged.

Composed images go to `<cache>/kisetsu/overlay/`, alternating between
`wallpaper-a.jpg` and `wallpaper-b.jpg`, because GNOME and MATE do not
reload a picture whose path did not change. The window and the agent may
both compose, so the file to write is chosen by age, not remembered; it
is written to a temporary file and renamed, so a desktop never reads half
of it.

Changing the text settings in the window shows the wallpaper Kisetsu set
last again at once, with or without the text. This is skipped while
Variety sets the wallpaper, and when Kisetsu never set one.

### Author and license

One line, "Ana Ćirić · CC BY-SA 4.0 · Wikimedia Commons": the author
and the license when known, and the source, which always is. A name the
font cannot draw is left out rather than shown as empty boxes. The font
size is 1.9 % of the screen height (20 px on a 1080 px screen).

### Clock

The time ("14:07", or "2:07 PM") and, optionally, the date under it
("Friday, 6 November"; English, like the rest of Kisetsu), in one of
three sizes (5, 7.5 or 11 % of the screen height; the date a third of
that). Off by default.

The agent redraws it: every 2 seconds it asks the screen service whether
the clock's text differs from the one last drawn, and if so shows the
current wallpaper again under the rotation lock. Asking instead of
timing keeps the clock right after a suspend, a change of the system
clock or of the time zone, and catches a changed format. The check
reads one setting and formats the time; drawing happens once a minute.

- The agent runs while the clock is on (as for rotation and the tray
  icon); turning the clock on in the window starts it, and the clock
  stays off when the agent cannot run.
- The agent keeps the base of the current image in memory, so a new
  minute only draws the text and encodes the JPEG. The window does not
  keep it.
- A wallpaper change in progress holds the rotation lock; the clock waits
  for it, and the change draws the clock too.
- Without a wallpaper set by Kisetsu, there is nothing to draw on; the
  agent tries again at the next minute.
- While the clock is on, Kisetsu keeps showing the wallpaper it set
  last: a wallpaper chosen in the desktop's settings is replaced at the
  next minute. The Wallpaper tab says so.

## Measured

On an Intel i7-10510U (a 2019 laptop CPU), for a 4K screen:

| Work                                   | When                    | Time    |
| -------------------------------------- | ----------------------- | ------- |
| `Base` from a fetched image (crop)     | once per wallpaper      | 16 ms   |
| `Base` from a 24 MP photo (scale)      | once per wallpaper      | 2.5 s   |
| `Draw` (two blocks of text)            | every minute (clock)    | 30 ms   |
| JPEG encoding                          | every minute (clock)    | 300 ms  |

A 1920×1080 screen has a quarter of the pixels.

## Alternatives considered

- **A desktop window or widget.** Not possible on GNOME with Wayland, and
  differs on every desktop.
- **A band behind the text.** More legible on busy images, but covers
  part of the picture; the shadow is enough in tests on light and dark
  images.
- **System fonts.** Would cover every script, but which font exists
  differs by system, and finding one needs fontconfig or a new
  dependency.
