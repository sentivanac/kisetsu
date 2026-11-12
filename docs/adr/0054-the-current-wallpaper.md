# ADR 0054: Actions for the current wallpaper

- **Status:** accepted; being built for 0.8.0
- **Date:** 2026-11-12

## Context

The wallpaper changes, and the user reacts to the picture on screen: they
want the one before back, never to see this one again, or to keep it as
a favorite. The tray menu is where that reaction is quickest, without
opening the window. With local folders (ADR 0053) there is no "Delete"
for many images, so leaving rotation needs its own way.

Planned for 0.8.0 (decided 2026-11-12): Previous, Don't show again, Add
to favorites, in the tray menu and in the window.

## Decision

### Previous wallpaper

- `Rotator.Previous` shows the image before the current one in the
  wallpaper history; called again, it goes further back. "Next" then
  shows a new image, as before (as Variety does): Previous does not walk
  forward again.
- The history stays as it is. Showing an earlier image writes a new row,
  so without more, a second Previous would only swap the last two
  images. Kisetsu therefore remembers where a run of Previous stands
  (setting `rotation_back`: the row Previous wrote and the older row it
  went back to). The run continues only while Previous's own row is the
  newest; any other change (Next, rotation, "Set as wallpaper") ends it.
- The same picture twice in a row is no step back; images that are gone,
  whose files are missing, or that are hidden from rotation are skipped.
  It looks through the last 200 changes at most.
- The change counts as one made by hand: rotation does not count it as
  shown in its cycle, and waits a full interval after it.
- In the tray menu above "Next wallpaper", and as "Previous" next to
  "Next wallpaper" in the Wallpaper tab. Both hold the rotation lock.

### Still to come in 0.8.0

1. Don't show again: out of rotation for good, with a list that undoes
   it.
2. Add to favorites: an ordinary collection "Favorites".

## Verification

Unit tests: going back three steps and stopping at the start of the
history; another change ending the run; skipping a missing file, a
hidden image and a repeated picture; not counting as shown in the cycle;
no history at all. In the window: Next, Next, Previous, Previous showed
the images in exactly the reverse order.
