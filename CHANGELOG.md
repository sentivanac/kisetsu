# Changelog

All notable changes to Kisetsu are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/).

## [0.5.0] - 2026-11-09

### Search
- New source, **Openverse** (off by default): paintings, prints and
  photographs from the Rijksmuseum, Museums Victoria and the National
  Gallery of Denmark, and photos from the WordPress Photo Directory, all
  openly licensed or in the public domain. Only collections with files
  large enough for a wallpaper are searched.
- Under the results, "Also try:" suggests the same search with synonyms
  ("fall forest", "autumn woods" for "autumn forest") or the other
  spelling ("gray" for "grey"); a click runs it.

### Window
- **About Kisetsu**: version, author, license, links to the website and
  to bug reports, and credits. Open it by clicking the version at the
  bottom of the window, or from the tray icon's menu.

### Fixes
- The preview no longer flashes blurred for a moment after the full
  image appears.
- Quit in the tray menu no longer leaves a stopped clock on the
  wallpaper: the wallpaper is shown again without it (with the author
  and license if they are on).

## [0.4.0] - 2026-11-06

### On the wallpaper
- The image's author, license and source can be shown in a corner of the
  wallpaper (Wallpaper tab → On the wallpaper; off by default). Kisetsu
  draws them on a copy cropped to the screen; the image in the library
  is not changed.
- A clock on the wallpaper, with the date if you like, in three sizes and
  12- or 24-hour time, redrawn every minute by the background process
  (off by default). While it is on, Kisetsu keeps showing the wallpaper
  it set last.
- Tested on Windows 10, GNOME, KDE Plasma, Xfce and Cinnamon.

### Fixes
- Turning off "Show in tray" right after the background process started
  could fail with "invalid agent PID".

### Documentation
- Install: run `sudo apt update` first if `apt` says a package cannot be
  installed.

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
