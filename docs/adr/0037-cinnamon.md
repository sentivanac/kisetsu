# ADR 0037: Cinnamon

- **Status:** accepted
- **Date:** 2026-11-03
- First desktop added after Phase 7; extends ADR 0022 and ADR 0031.

## Context

Checked in a Linux Mint 22.3 Cinnamon live session (VirtualBox):

- the session is X11; `XDG_CURRENT_DESKTOP=X-Cinnamon`, `loginctl` reports
  `Desktop=cinnamon`, and the session file has no `DesktopNames`;
- the wallpaper is `org.cinnamon.desktop.background picture-uri`, a
  `file://` URI like GNOME's, with no dark variant;
- Mint also keeps `org.gnome.desktop.background` set, but Cinnamon does
  not read it.

The `.deb` installed there unchanged (WebKitGTK 4.1 2.50.3 from Ubuntu
24.04), and Kisetsu reported the desktop as unsupported — which exposed
the error-message bug fixed in the ADR 0004 addendum.

## Decision

`wallpaper.Cinnamon` sets only `org.cinnamon.desktop.background
picture-uri` (with the usual read-back check). The desktop is recognized
by the names `cinnamon`, `X-Cinnamon` and `cinnamon2d` (Mint's session
without 3D); an `X-` prefix is ignored when names are compared. GNOME's
keys are not touched on Cinnamon.

## Verification

Unit tests: the Cinnamon key is set with a percent-encoded URI, no GNOME
key changes, and `cinnamon` / `cinnamon2d` sessions without session-file
entries are recognized. In the Mint 22.3 live session (2026-11-03), with
the new `.deb` installed: "Set as wallpaper", "Next wallpaper" and the
rotation timer all changed the wallpaper.
