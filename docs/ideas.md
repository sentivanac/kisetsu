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

## 0.7.0: local folders (planned 2026-11-12)

Images the user already has, used where they are, subfolders included
(ADR 0053). Steps: the data and syncing; the window showing folder files
and thumbnails; a "Folders" view in the Library tab; a folder as a
rotation source; checks, docs, release. Favorites and rotation by time of
day (theme A) follow after.

## 0.6.0: effects and text from a file (planned 2026-11-09)

Both build on the composer of 0.4.0 (ADR 0047), off by default.

1. **Effects** on the wallpaper: darken, blur, black and white,
   vignette. They make text and desktop icons legible on bright, busy
   images. Applied once per image, to the base the clock is drawn on.
2. **Text from a file**: the content of one text file the user picks,
   in a corner, redrawn when it changes (checked with the clock). Kisetsu
   never runs a command: output of `fortune` or `df -h` comes through a
   file the user's own cron or timer writes. Guards: only a regular file
   owned by the user and writable only by them, a few KB, valid UTF-8,
   control and direction characters removed, at most about 10 lines; a
   warning that whatever is in the file will be on the desktop. A folder
   of files, shown in turn, may follow later.
3. Checks on real desktops where behaviour may differ, docs, release.

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
2. **Find similar** (postponed 2026-11-09): Wallhaven's `like:<id>`
   search, computed by Wallhaven, as a button in the preview. Its API
   documents `like:`, but answers it with 403 and a Cloudflare challenge
   page, while other searches work. Check again later; if it stays
   blocked, a search by the image's tags is the fallback.
3. **Synonyms**: a small, readable word list in the code, offered as
   suggested searches under the results ("autumn" → "fall", "foliage";
   ADR 0049). None of the sources can search alternatives in one
   request.

## Phase 5: local AI search (postponed indefinitely, 2026-11-05)

The plan with Python and OpenCLIP is too heavy for what most users of a
wallpaper app would turn on; 0.5.0 above covers most of its value. If AI
is ever added, the lighter way: a small CLIP model (100 to 150 MB) run
from Go through ONNX, without Python, optional and local only. It could
also rate images as suggestive locally (ADR 0040).
