# ADR 0045: The agent on Windows

## Status

Accepted (2026-11-05).

## Decision

The agent (ADR 0041) and the tray icon (ADR 0042) run on Windows as on
Linux; only these parts differ:

- **Start at sign-in:** a value named `io.github.sentivanac.Kisetsu.Agent`
  in `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`,
  holding `"…\kisetsu.exe" agent`. No administrator rights; it shows in
  Task Manager's Startup tab. A value of this name is Kisetsu's own; the
  "outdated" check reads the program back from it. Tests use a key of
  their own, never the real Run key.
- **Starting and stopping:** the agent is started without a console
  (`DETACHED_PROCESS`, its own process group). Windows has no SIGTERM
  for such programs, so the window ends it with `TerminateProcess` (PID
  from `agent.lock`); it keeps no state that needs a clean exit, and the
  system releases its lock. Windows ends it at sign-out; no session watch
  is needed.
- **Tray:** the same library, with the `.ico` icon, set once the tray
  runs (on Windows the library accepts an icon only then; on Linux it is
  set before, for Ayatana).
- **Command line:** `kisetsu.exe` is a GUI program, which gets no console.
  For commands it attaches to the console of the terminal it was started
  from (`AttachConsole`), unless output is redirected. The terminal does
  not wait for GUI programs, so its prompt comes before the output; one
  Enter shows it again. A console twin (`kisetsu.com`, which a terminal
  prefers when `kisetsu` is typed) would avoid this; not done for now.
- **One window:** Wails' single-instance lock works on Windows as well.

## Verification

Unit tests on `windows-latest` for the Run value. In the Windows 10 22H2
VM: turning rotation on starts the agent; the tray icon and its menu
(Next wallpaper, Change automatically, Open Kisetsu, Quit); the agent
keeps rotating with the window closed; after signing out and in, it
starts by itself; `kisetsu.exe version` prints the version.
