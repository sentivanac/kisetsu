# Ideas for later

Ideas that were discussed and are worth doing some day, but are not
planned for a specific version. When one is picked up, it gets a plan
and an ADR; until then this is only a note.

## Overlays on the wallpaper (discussed 2026-11-03)

Kisetsu would compose a new image (original + overlay) before setting it,
as Variety does for its clock — not a separate desktop window, which
GNOME on Wayland does not allow and which conky already covers.

In order of effort:

1. **Simple effects** applied when the wallpaper is set: darken, blur,
   grayscale, vignette, tint. Small: decoding, resizing and the fetched
   cache already exist.
2. **Author and license on the wallpaper**, in a corner, changed only
   with the image. Small to medium: an embedded free font (Noto or
   DejaVu), a shadow or band for legibility. Fits Kisetsu's respect for
   image creators.
3. **Clock or the output of a command**, refreshed every minute. Medium:
   a per-minute timer that recomposes from a cached base image and sets
   the wallpaper again (CPU and disk work on laptops, GNOME's transition
   on every change), a timeout and length limit for commands, and a
   decision on where the text goes with several monitors. Optional and
   off by default.

## Tray indicator (discussed 2026-11-02)

A small separate process (`kisetsu tray`, optional autostart) with Next
wallpaper, Pause/Resume rotation, Open Kisetsu and Quit, through
StatusNotifierItem over D-Bus (for example `fyne.io/systray`; Wails v2
has no tray). Needs a monochrome icon and a single-instance window. Not
shown on plain GNOME without an extension. Good companion to the next
item, which is tested on the same desktops.

## More desktops (planned after 0.1.0)

Cinnamon (`org.cinnamon.desktop.background`), KDE Plasma
(`plasma-apply-wallpaperimage`), XFCE (`xfconf-query`, per monitor and
workspace). Each verified in a VirtualBox live session before it is
claimed as supported.

## Later phases

- Phase 5: local AI search (OpenCLIP), optional.
- Windows, after the Linux release.
