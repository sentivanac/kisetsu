# ADR 0056: Telling about new versions

- **Status:** accepted, for 0.9.0
- **Date:** 2026-11-24

## Context

A `.deb` installed from a file is never updated by `apt`, and the Windows
installer has no updater, so users learned of a release only by looking.

## Decision

- Kisetsu asks GitHub's `releases/latest` once a day and tells about a newer
  version: status line, tray menu, About. It downloads and installs nothing.
- **On by default** with a switch in About Kisetsu (user decision): otherwise
  hardly anyone would learn of releases. The README says what is sent.
- `internal/updates` is used by the window and the agent. The answer is in
  the database, so both together ask once a day. A failed check is tried
  again after an hour.
- The address of the release page is made from the version; the answer's own
  addresses are not used. A tag that is not a plain version is refused.
- Development builds without a version ("dev") never ask. An unreadable
  setting counts as off.

## Later

An APT repository would update Linux installations through the system. Such
an installation should not be told by Kisetsu as well.
