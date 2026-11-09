# ADR 0051: Text from a file on the wallpaper

- **Status:** accepted
- **Date:** 2026-11-09

## Context

Users of conky and Variety put more than a clock on the desktop: a note,
the output of `fortune` or `df -h`. Running a command from Kisetsu was
considered and dropped (2026-11-05): it would make Kisetsu run programs
on a schedule, with the risks that come with that. Showing a file is a
smaller step: Kisetsu only reads text, and whatever writes the file (the
user's own cron job or timer) stays outside Kisetsu.

## Decision

One text file the user chooses in the Wallpaper tab ("Show text from a
file", "Choose file…", a corner and a size; off by default) is drawn on
the wallpaper and redrawn when it changes.

### Guards

The file's content ends up on the desktop, so it is read carefully
(`internal/onscreen/textfile.go`):

| Risk | Guard |
| --- | --- |
| A FIFO or a device blocks or floods the agent | Opened without blocking; only a regular file, also behind a symlink |
| Another user writes text onto this user's desktop (for example a fake "enter your password" note in a shared folder) | Linux: only a file the user owns that neither group nor others can write. Windows: not checked (access lists); files in the user's folders are the user's by default |
| A huge or binary file | At most 4 KB, cut at the last whole line; valid UTF-8 only |
| Text that looks different from what it is | Terminal escape sequences (colors from commands), control characters, the byte order mark and characters that change the direction of text are removed |
| Too much text | At most 10 lines; lines wider than 60 % of the screen are shortened with "…"; characters the font lacks become "?" |
| Private text in screenshots and screen sharing | Off by default; the Wallpaper tab warns that whatever is in the file is on the desktop |

When a guard refuses the file, or it is missing, the wallpaper says why
("Text file not found", "Others can change the text file") instead of
showing nothing, so the user knows what to fix.

### Drawing and updates

- Go Mono (from `golang.org/x/image`, like Go Regular), in three sizes:
  1.7, 2.2 or 3 % of the screen height (18, 24 or 32 px at 1080 px;
  medium by default), the lines aligned with each other on the left in
  every corner: a command's columns stay columns.
- The clock and the file are the "live" text the agent keeps up to date.
  Every 2 seconds it compares what they show with what it drew last
  (`RedrawDue`, formerly `ClockDue`) and redraws when they differ; reading
  a file of at most 4 KB costs next to nothing. The base with the effects
  is kept, as for the clock.
- The agent runs while the file is on, as for the clock. Quit in the
  tray menu shows the wallpaper without the clock and without the file,
  which nothing would update any more.

## Verification

Unit tests for every guard (a FIFO does not block, files others can
write, a missing file, a directory, binary content, a symlink, the line
and size limits, escape and direction characters), for redrawing when
the file changes or disappears, for the agent being wanted, and for the
block alignment. In the window on Linux: choosing the file in the
dialog, `df -h` output with color codes (cleaned, columns aligned), a
change shown about 3 seconds later, the three sizes, and a file made
writable by everyone (the message instead of the content). On Windows 10
(the release candidate in the VM): the file dialog, the sizes, updates,
and Quit in the tray removing the text.

## Limitations

- One file. A folder of files shown in turn may follow.
- No ownership check on Windows.
- A change is shown within about 2 seconds, not at once.

## Alternatives considered

- **Running a command** with a timeout and a length limit: Kisetsu would
  run programs every minute; the file leaves that to the user's own
  tools.
- **Watching the file** (inotify, ReadDirectoryChanges): faster, but a
  second mechanism next to the clock's check, and editors often replace
  files instead of writing them; reading 4 KB every 2 seconds is simpler.
- **Drawing in Go Regular:** columns of command output fall apart in a
  proportional font.
