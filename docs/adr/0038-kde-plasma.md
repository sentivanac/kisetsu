# ADR 0038: Wallpaper on KDE Plasma

## Status

Accepted (2026-11-03).

## Context

Checked in a Kubuntu 26.04.1 live session (VirtualBox, Plasma 6,
Wayland):

- `XDG_CURRENT_DESKTOP=KDE`, also in the systemd user manager's
  environment, so the rotation timer sees it too.
- loginctl reports the graphical session but an empty `Desktop`, so the
  desktop is recognized from `XDG_CURRENT_DESKTOP` (ADR 0031, step 2).
  Plasma's session files (`plasma.desktop`, `plasmax11.desktop`) say
  `DesktopNames=KDE`, which covers systems where loginctl does give the
  name.
- Plasma keeps the wallpaper per screen in
  `plasma-org.kde.plasma.desktop-appletsrc`, not in gsettings.
- `plasma-apply-wallpaperimage` (part of Plasma since 5.24) is installed;
  `qdbus6` is too, under a name that differs between distributions
  (`qdbus`, `qdbus6`, `qdbus-qt6`).

Options:

1. `plasma-apply-wallpaperimage PATH`: Plasma's own command. It asks the
   running plasmashell to show the image on every screen, on Wayland and
   X11.
2. A Plasma script through D-Bus (`org.kde.PlasmaShell.evaluateScript`):
   more control (per screen, fill mode), but needs a `qdbus` whose name
   varies, and the scripting API has changed between Plasma versions.
3. Editing the config file: Plasma does not notice the change while it
   runs, and overwrites the file.

## Decision

Option 1. On KDE, Kisetsu runs `plasma-apply-wallpaperimage` with the
absolute path of the image (a plain path, not a URI). A missing command
means Plasma older than 5.24, which is reported as such.

Unlike gsettings, there is no reliable read-back: Plasma writes its
config on its own schedule. A failure shows in the command's exit status
(for example when plasmashell is not running), and is reported like any
other.

## Consequences

- Plasma 5.24 and newer (Kubuntu 22.04 and later); every screen gets the
  same image.
- The image is shown with Plasma's current fill mode.
- The lock screen keeps its own picture.

## Verification

Unit tests: the command and its argument, a missing command, a failing
command, and recognition through `XDG_CURRENT_DESKTOP=KDE` and through
Plasma's session file. In the Kubuntu 26.04.1 live session (2026-11-03),
with the new `.deb` installed: "Set as wallpaper", "Next wallpaper" and
the rotation timer all changed the wallpaper.
