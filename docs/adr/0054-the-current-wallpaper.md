# ADR 0054: Actions for the current wallpaper

- **Status:** accepted, for 0.8.0
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

### Don't show again

- Table `hidden_images` (migration 0010, a new table, no rebuild): the
  image stays in the library or its folder, and the row goes with the
  image (`ON DELETE CASCADE`).
- `Rotator.Next` leaves hidden images out of every source; `Previous`
  skips them. Setting one as the wallpaper by hand still works: it is
  the user's choice.
- "Don't show again" in the tray menu and "Don't show this again" in the
  Wallpaper tab hide the wallpaper on screen and show the next one (if
  nothing else is left, the hidden one stays until there is). In an
  image's preview, the same button hides that image, and becomes "Show
  in rotation again".
- The Wallpaper tab lists the hidden images, with "Show again" for each.
- This is how a folder image leaves rotation: Kisetsu deletes nothing
  in a folder.

### Add to favorites

- Favorites is an ordinary collection, not a new kind of thing: it can
  be renamed, browsed, put in rotation ("Images from") and deleted like
  any other. It is made the first time an image is added; its ID is
  remembered (setting `favorites_collection`), so renaming keeps it the
  favorites. A collection the user already called "Favorites" or
  "Favourites" is used instead of making a second one; if the favorites
  collection is deleted, the next favorite makes a new one.
- A fetched image is kept in the library first (as "Keep in my
  library"), so a favorite never disappears when the cache is cleaned.
- "Add to favorites" in the tray menu adds the wallpaper on screen. In
  the Wallpaper tab the same button shows "In favorites" when the
  wallpaper is one, and takes it out when clicked. In an image's
  preview, a heart adds or removes that image.

### Next does not wait for the internet

Found while using the tray: with a saved search as the source, "Next"
fetched new images (searches and five downloads, about 30 seconds)
before changing whenever fewer than three were left unshown, and the
disabled menu item said nothing about why.

- The change now happens at once while at least one image of the saved
  search is unshown; the top-up (ADR 0030) follows the change. The
  window and the agent fetch in the background, so "Next" returns;
  `kisetsu rotate`, which ends at once, fetches right after the change.
  One top-up runs at a time in a process.
- Only with nothing new left does the change wait for new images, so it
  does not repeat one; the tray item then reads "Fetching new images…".

## Verification

Unit tests for Previous: going back three steps and stopping at the
start of the history; another change ending the run; skipping a missing
file, a hidden image and a repeated picture; not counting as shown in
the cycle; no history at all. In the window: Next, Next, Previous, Previous showed
the images in exactly the reverse order.

Unit tests for hiding: rotation without hidden images, none left
(`ErrNoImages`), shown again; Previous skipping a hidden image; the App
marking hidden items, listing them, hiding the current wallpaper and
changing to the next. In the window: "Don't show this again" in the
Wallpaper tab (hidden, next image shown, listed), "Show again", and the
button in a folder image's preview.

Unit tests for favorites: the collection made by the first favorite,
renamed and still used, an existing "Favourites" reused, a fetched image
kept in the library, the current wallpaper, removing. In the window: the
Wallpaper tab button in both directions, and the heart in a folder
image's preview (the collection's checkbox followed).

Unit tests for the order of change and fetch: changing first with
images left, waiting (and telling the tray) only with none left, and
fetching in the background while Next returns.

On Windows 10 (the release candidate in the VM, upgrading 0.7.x with
migration 0010): the three tray items, the buttons of the Wallpaper tab
and the list of hidden images.
