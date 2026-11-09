# Ideas for later

Ideas that were discussed and are worth doing some day, but are not
planned for a specific version. When one is picked up, it gets a plan
and an ADR; until then this is only a note.

## 0.4.0: overlays and a clock on the wallpaper (planned 2026-11-05)

Kisetsu composes a new image (the wallpaper plus an overlay) and sets
that, as Variety does for its clock — not a separate desktop window,
which GNOME on Wayland does not allow and which conky already covers.
The background agent (ADR 0041) makes the clock affordable: it already
runs all session, knows the current image and shares the rotation lock
with the window.

Steps, in this order:

1. **Composer.** Reduce the current image once to the screen size
   (already stored), keep it in memory as the base, draw text on it with
   an embedded free font (the Go fonts, BSD licensed: no new external
   dependency) and a shadow or band for legibility, and write the result
   to the cache. Two alternating file names, because GNOME and MATE do
   not reload a picture whose path did not change. History keeps the
   real image, never the composed one.
2. **Author and license** in a corner, drawn when the image changes.
   Fits Kisetsu's respect for image creators, and tests the composer
   without any per-minute work.
3. **Clock**, redrawn by the agent every minute from the cached base,
   under the rotation lock. A change from the window shows without the
   clock until the next minute. Off by default.
4. **Settings** in the Wallpaper tab: what to show, corner, 24 h or
   12 h, size.

To check on each desktop before claiming support (as for rotation):
whether the desktop fades on every change (a fade every minute may
disturb), the cost on laptops (tens of milliseconds and one settings
call per minute), where the text lands with several monitors and with
zoom or span modes, and never fighting Variety (no overlay while it
runs). Later, on the same composer: simple effects (darken, blur,
grayscale, vignette) and the output of a command (with a timeout and a
length limit).

## Restart the agent after a crash (decided 2026-11-04: later)

If the agent crashes, rotation stops until the next login or until it is
started from the Wallpaper tab. A small supervisor (or, where present, a
systemd user service with `Restart=on-failure`) could restart it. Not
needed while crashes are not seen.

## More desktops

LXQt (`pcmanfm-qt --set-wallpaper`), Budgie and others, and Xfce on
Wayland. Each verified in a VirtualBox live session before it is claimed
as supported, as Cinnamon (ADR 0037), KDE Plasma (ADR 0038) and Xfce
(ADR 0039) were.

## 0.5.0: better search without AI (planned 2026-11-05)

Most of what the local AI search (Phase 5) was meant to bring, without
its weight (Python, PyTorch and a model: 1 to 2 GB, minutes of work per
library):

1. **More sources**, each its own provider module:
   - **Openverse**, limited to the museums and the photo directory whose
     files are large enough (ADR 0048): most of its images come from
     Flickr at 1024 px.
   - **Not Unsplash or Pexels:** their API terms forbid wallpaper
     applications by name. **Not Pixabay** without approval for full API
     access (1280 px otherwise). Checked 2026-11-06.
   - Later, perhaps: the Met (its files are excellent, but results come
     without sizes), NASA's image library.
2. **Find similar**: Wallhaven's `like:<id>` search, computed by
   Wallhaven, as a button in the preview.
3. **Synonyms and query expansion**: a small, readable word list in the
   code (for example "autumn" also finds "fall" and "foliage").

## Phase 5: local AI search (postponed indefinitely, 2026-11-05)

The plan with Python and OpenCLIP is too heavy for what most users of a
wallpaper app would turn on; 0.5.0 above covers most of its value. If AI
is ever added, the lighter way: a small CLIP model (100 to 150 MB) run
from Go through ONNX, without Python, optional and local only. It could
also rate images as suggestive locally (ADR 0040).
