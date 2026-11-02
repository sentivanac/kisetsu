# ADR 0032: Application identity — ID, icon, version

- **Status:** accepted
- **Date:** 2026-11-02
- First step of Phase 7 (public release).

## Decision

### Application ID

`io.github.sentivanac.Kisetsu` (reverse DNS of the project's home). It
names the `.desktop` file and the icon, and is the ID a Flatpak or
AppStream metadata would need later. The window's program name stays
`kisetsu` (`StartupWMClass=kisetsu`), so the desktop links the running
window to the menu entry.

### Icon

A white mountain in front of an amber sun on the app's dark background
(colors from `style.css`). Chosen by the user from three drafts because
it reads best at 16–32 px, on light and dark panels.

- Source: `build/linux/io.github.sentivanac.Kisetsu.svg`.
- `build/linux/icons/{16…512}.png` and `build/appicon.png` (1024 px) are
  rendered from it with `make icons` (Inkscape) and committed, so
  building and packaging need no Inkscape.
- The 256 px PNG is embedded as the window icon, for a binary started
  without an installed `.desktop` file.
- `build/windows/icon.ico` still shows the Wails default; it is
  regenerated when Windows support starts.

### `.desktop` file

`build/linux/io.github.sentivanac.Kisetsu.desktop`: categories
`Graphics;Viewer;GTK;`, keywords for menu search. Checked with
`desktop-file-validate`.

### Version

Written once, as `VERSION` in the Makefile (0.1.0 for the first
release), and set at build time with `-ldflags -X …appinfo.Version`.
`wails dev` and `go test` keep `dev`. `kisetsu version` prints it; it is
also in the User-Agent sent to image providers.
