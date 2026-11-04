# ADR 0031: Desktop from the login session

- **Status:** accepted; `KISETSU_DESKTOP_FALLBACK` removed in 0.2.0 with
  the timer (ADR 0041): the agent starts inside the session, so its
  `XDG_CURRENT_DESKTOP` is right. Detection through loginctl stays.
- **Date:** 2026-11-02
- Changes desktop detection from ADR 0022 and the timer environment from
  ADR 0025.

## Context

The wallpaper method (MATE or GNOME) was chosen from `XDG_CURRENT_DESKTOP`.
For the rotation timer that value is unreliable:

- the timer unit set `XDG_CURRENT_DESKTOP` to the desktop found when
  rotation was enabled. After enabling rotation in MATE and logging into
  GNOME, the timer would still have changed the MATE setting, and the
  GNOME wallpaper would not have changed;
- the systemd user manager keeps the variables exported by the first
  session. Sessions such as i3 do not update them, so the timer may see
  the desktop of an earlier session.

Found while testing GNOME, Wayland and i3 sessions: the results were right
only because the timer did not run during the tests.

## Decision

The desktop is taken, in this order, from:

1. **the user's graphical session** according to `loginctl`
   (`show-user UID --property=Display`, then `show-session … Desktop,
   Type`) and the `DesktopNames` of its session file in
   `$XDG_DATA_DIRS/{xsessions,wayland-sessions}/<Desktop>.desktop` —
   the source the login manager itself uses for `XDG_CURRENT_DESKTOP`.
   Session files use `;` or `:` as separators. A file without
   `DesktopNames` (e.g. `i3-with-shmlog`), or no file, means the session
   name itself;
2. `XDG_CURRENT_DESKTOP`;
3. `KISETSU_DESKTOP_FALLBACK`, which the timer unit now sets instead of
   `XDG_CURRENT_DESKTOP`.

`loginctl` properties are read by name: it prints them in its own order.
A session name containing `/` or `\`, or `.`/`..`, is ignored because it
becomes part of a path.

On this system: `mate` → `MATE`, `ubuntu` → `ubuntu:GNOME` (X11 and
Wayland), `i3` → `i3` (unsupported, reported as such).

## Consequences

- Rotation follows the desktop the user is logged into now, whatever
  the timer's environment says.
- An installed timer still sets `XDG_CURRENT_DESKTOP`; it is harmless now
  (step 1 wins) and is replaced the next time rotation settings are saved.
- Setting `XDG_CURRENT_DESKTOP=NONE` no longer prevents a real wallpaper
  change in tests. Safe end-to-end checks need a `PATH` without
  `gsettings` (or a fake one).
- Without a graphical session, e.g. over SSH, the environment decides as
  before.

## Verification

Unit tests with session files like this system's (MATE, ubuntu X11 and
Wayland, i3, i3-with-shmlog): a stale `MATE` or `ubuntu:GNOME` in the
environment loses to the active session; i3 is unsupported whatever the
environment says; without a session the environment and then the
fallback are used. `loginctl` output on this system checked by hand
(`Desktop=mate`, `Type=x11`).
