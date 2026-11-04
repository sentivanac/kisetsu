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

## Live overlays in the agent

The tray icon and rotation now live in `kisetsu agent` (ADR 0041, 0042),
which runs while the user is logged in. Per-minute overlays (clock,
command output) belong there too: the agent recomposes the current image
(kept as a cached base) with the overlay and sets it again, under the
rotation lock, without starting the program every minute. Static
overlays (effects, author and license) are applied whenever an image is
set.

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

## Later phases

- Phase 5: local AI search (OpenCLIP), optional.
- Windows, after the Linux release.
