# ADR 0033: .deb package

- **Status:** accepted
- **Date:** 2026-11-02
- Second step of Phase 7.

## Context

Kisetsu links WebKitGTK 4.1 and GTK 3 dynamically. Options compared:

| Format | For | Against |
|---|---|---|
| **.deb** | native on Ubuntu, Mint, Kubuntu, Xubuntu, Debian 12+; `apt` installs WebKitGTK and GTK; small; built with `dpkg-deb`, no new tool | not for Fedora, Arch |
| AppImage | one file, many distributions | WebKitGTK starts helper processes from paths compiled into the library, which makes bundling it fragile; 100 MB+ |
| Flatpak | sandbox, WebKitGTK in the GNOME runtime | the sandbox blocks what Kisetsu needs on the host: `gsettings` of the host desktop, a systemd user timer, `loginctl`, `/proc` (Variety) |

## Decision

`.deb` first (user decision). `.rpm` and Flatpak later, if there is
demand.

`make deb` builds the release binary and runs `scripts/build-deb.sh`,
which writes `build/deb/kisetsu_<version>_<arch>.deb`. It installs
nothing.

- **Contents:** `/usr/bin/kisetsu`, the `.desktop` file, the icon (SVG
  and PNG sizes in `hicolor`), `/usr/share/doc/kisetsu/{copyright,
  changelog.gz}`. The icon cache and menu are updated by dpkg triggers
  of `hicolor-icon-theme` and `desktop-file-utils`, so the package has
  no maintainer scripts.
- **Depends** are computed by `dpkg-shlibdeps` from the libraries the
  binary links (libc6 ≥ 2.34, GTK 3, WebKitGTK 4.1, libsoup 3, …), so
  they stay right when the libraries change. That means Ubuntu 22.04+,
  Linux Mint 21+, Debian 12+.
- **Recommends:** `libglib2.0-bin` (`gsettings`, to set the wallpaper)
  and `systemd` (rotation). Kisetsu works without them as an image
  finder and library.
- The script refuses a binary whose `kisetsu version` is not the
  package version (for example one built by `wails dev`).
- The release binary is built as **PIE** (`GOFLAGS=-buildmode=pie`), so
  ASLR covers it fully. The first PIE build takes about 1.5 minutes
  because the standard library is rebuilt; later ones are fast.
- `--root-owner-group`: files are owned by root without `fakeroot`.

## Verification

- `lintian`: only `no-manual-page` (a warning) remains.
- `dpkg-deb --info/--contents`: metadata, ownership and modes as listed
  above.
- `apt-get --simulate install ./build/deb/kisetsu_0.1.0_amd64.deb` on
  the development machine: all dependencies satisfied, nothing else
  installed. The package has not been installed for real.

## Open

- Uninstalling the package leaves the user's rotation timer, which then
  starts a missing `/usr/bin/kisetsu`. To be handled in the pre-release
  review (ADR to follow).
- A manual page.
