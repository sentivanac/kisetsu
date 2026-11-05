# Changelog

All notable changes to Kisetsu are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/).

## [0.3.0] - 2026-11-05

Kisetsu now runs on **Windows 10 and 11** as well.

### Windows
- Installer for the current user, without administrator rights; it
  replaces an older version and keeps your library when Kisetsu is
  removed. Not signed yet: Windows may warn before starting it.
- Search, library, "Set as wallpaper", rotation in the background
  (started at sign-in), the tray icon and the command line work as on
  Linux.
- Files live in `%LocalAppData%\Kisetsu`.

### Fixes
- A database path containing `%`, `#` or `?` now opens correctly.
- A full log no longer stops Kisetsu from starting when another Kisetsu
  process has it open.

## [0.2.0] - 2026-11-04

### Search
- Wallhaven categories (General, Anime, People) can be turned off in the
  search form; saved searches remember them, so Fetched and rotation
  follow them too.
- The preview of a Wallhaven image lists its tags; clicking one hides
  images with that tag from all later searches, including Fetched.

### Wallpaper
- Rotation runs in a small background process, `kisetsu agent`, started
  at login, instead of a systemd timer. The next change counts from the
  last one, so a wallpaper you set by hand stays for a full interval,
  and a computer waking from sleep catches up with one change. The
  window replaces the timer of 0.1.0 the first time it opens.
- The Wallpaper tab shows whether the background process runs, and
  starts it if it does not.
- Optional tray icon with Next wallpaper, Change automatically, Open
  Kisetsu and Quit (on plain GNOME it needs the AppIndicator extension).
- Only one Kisetsu window: starting it again brings the open window to
  the front.

## [0.1.0] - 2026-11-03

The first release. Linux only (Ubuntu 22.04+, Linux Mint 21+, Debian 12+).

### Search
- Search Wallhaven (safe-for-work images only) and, when turned on,
  Wikimedia Commons at the same time; results are interleaved, and one
  source failing does not stop the others.
- Filters for minimum size, "my screen size" and orientation; `-word`
  leaves a word out.
- Saved searches.
- Previews with size, file type, source, author and license.

### Library
- Download images into a local library that works offline.
- Author, source page and license are kept for every image.
- Collections.
- Duplicate detection, including re-encoded copies of the same picture.
- Thumbnails, deletion with confirmation, disk usage.

### Wallpaper
- Set an image as the wallpaper on GNOME and KDE Plasma (X11 and
  Wayland), Xfce, MATE and Cinnamon (Linux Mint), or through Variety when
  it runs.
- Automatic rotation every 15 minutes to once a day, in random or added
  order, from the library, a collection or a saved search; it runs as a
  systemd user timer, also while the window is closed.
- From a saved search, new images are downloaded at screen size when few
  are left; they live in a 200 MB cache, and **Keep** moves one into the
  library.

### Command line
- `kisetsu rotate`, `kisetsu fetch NAME [--count N]`, `kisetsu version`.

### Packaging
- `.deb` package with menu entry, icon, manual page and the licenses of
  all included libraries.
