# Changelog

All notable changes to Kisetsu are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions
follow [Semantic Versioning](https://semver.org/).

## [0.1.0] - Unreleased

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
- Set an image as the wallpaper on GNOME (X11 and Wayland) and MATE, or
  through Variety when it runs.
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
