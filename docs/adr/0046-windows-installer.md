# ADR 0046: Windows installer

## Status

Accepted (2026-11-05).

## Decision

An NSIS installer, built on Linux by `make windows` (cross-compiled
`kisetsu.exe`, the third-party licenses, `makensis`), in CI on every push
and in the Release workflow on the same Ubuntu job as the `.deb`. No
Windows machine is needed to build it.

- **Per user, without administrator rights:** into
  `%LocalAppData%\Programs\Kisetsu`, with a Start menu and a desktop
  shortcut, and listed in "Apps & features" under the current user. The
  Wails template's `project.nsi` is replaced; every value it would take
  from `wails.json` is defined in it, so the generated `wails_tools.nsh`
  placeholders are never used.
- **Contents:** `kisetsu.exe`, `LICENSE.txt` and
  `THIRD_PARTY_LICENSES.txt`. Microsoft's WebView2 bootstrapper (as
  shipped in the Wails module; Microsoft allows redistributing it) runs
  only when WebView2 is missing.
- **Update:** the installer ends running Kisetsu processes (window and
  agent) first, so files in use do not block it, and starts the agent
  again afterwards if its Run value exists. The last page offers to start
  Kisetsu.
- **Uninstall:** ends Kisetsu, removes the agent's Run value, the program
  folder, the shortcuts and the WebView2 cache. **The library, the
  database and the settings in `%LocalAppData%\Kisetsu` stay**; the log
  of the uninstaller says where.
- **Versions:** the file version is `VERSION` from the Makefile (Windows
  accepts only numbers) and must equal `productVersion` in `wails.json`
  (a test checks it); the installer's file name carries the full build
  version, e.g. `kisetsu-0.3.0-windows-amd64-setup.exe`.
- **WebView2 cache:** in `%LocalAppData%\Kisetsu\cache\webview`
  (`windows.Options.WebviewUserDataPath`), not in the roaming profile,
  which Wails uses by default.
- **Not signed.** Windows SmartScreen warns on first start ("More info,
  Run anyway"); Windows 11 with Smart App Control turned on blocks
  unsigned programs. Signing is a separate decision.

## Verification

In the Windows 10 22H2 VM, from the "clean" snapshot: install without
administrator rights, start from the Start menu, rotation and tray,
"Apps & features" entry, installing again over a running Kisetsu,
uninstall (no tray icon, no startup entry, program folder gone, library
kept), and installing again with the old library in place.
