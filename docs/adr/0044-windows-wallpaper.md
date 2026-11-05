# ADR 0044: Wallpaper on Windows

## Status

Accepted (2026-11-05).

## Decision

On Windows, `wallpaper.Setter` calls `SystemParametersInfoW` from
`user32.dll` with `SPI_SETDESKWALLPAPER` and the absolute path of the
image, with `SPIF_UPDATEINIFILE` (kept after sign-out) and
`SPIF_SENDCHANGE` (Explorer is told). It then reads the wallpaper back
with `SPI_GETDESKWALLPAPER` and compares it without regard to case, as
Windows compares paths; a different value is `ErrNotApplied`, like the
gsettings read-back on Linux.

- No external command and no new dependency (`golang.org/x/sys/windows`
  is already used for file locks).
- The picture position (Fill, Fit, Span, ...) stays as the user chose it
  in Windows Settings.
- There is no Variety on Windows; `VarietyRunning` is false.
- The call is a field of the Setter, so tests use a fake that keeps the
  wallpaper in memory and never change the real one.

## Verification

Unit tests on `windows-latest` (fake call): set and read back, saved and
announced, a relative path, a value that does not change, a failing
call, the same path in another case. In the Windows 10 22H2 VM, with the
cross-compiled `kisetsu.exe`: search, download, "Set as wallpaper" and
"Next wallpaper" changed the wallpaper.

## Also in this step

`build/windows/icon.ico` was still the template's icon; it is now made
from the application's PNG icons (16 to 256 px) by `make icons`.

## Limits

WebP images: Windows 10 shows them as wallpaper only with the WebP
image extension installed. Wallhaven and Commons rarely serve WebP; if it
turns out to matter, Kisetsu can convert them when setting.
