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
  changelog.gz, THIRD_PARTY_LICENSES.gz}`. The icon cache and menu are updated by dpkg triggers
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

### Third-party licenses (added in the pre-release review)

The MIT, BSD and Apache 2.0 licenses of the code built into the binary
require shipping their notices. `scripts/third-party-licenses.sh` lists
what is actually built in — the Go standard library, every module
recorded in the binary (`go version -m`) and the npm packages bundled
into the frontend (`npm ls --omit=dev`) — and copies every license and
notice file of each (some have several: `modernc.org/sqlite` also
carries SQLite's). A component without a license file fails the build.
For 0.1.0: 23 Go modules, Go itself, and react, react-dom, scheduler.

## Verification

- `lintian`: only `no-manual-page` (a warning) remains.
- `dpkg-deb --info/--contents`: metadata, ownership and modes as listed
  above.
- `apt-get --simulate install ./build/deb/kisetsu_0.1.0_amd64.deb` on
  the development machine: all dependencies satisfied, nothing else
  installed. The package has not been installed for real.

## Open

- Uninstalling the package leaves the user's rotation timer (a package
  cannot reach into users' systemd configuration). Since the pre-release
  review the timer then skips its runs quietly (ADR 0025, "Missing
  program"); the README explains how to remove it.
- ~~A manual page.~~ Added in Phase 7 step 3: `docs/man/kisetsu.1`,
  installed as `/usr/share/man/man1/kisetsu.1.gz`; a test checks that it
  lists every command of `kisetsu help`.
