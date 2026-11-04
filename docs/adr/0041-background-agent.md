# ADR 0041: Background agent instead of the systemd timer

## Status

Accepted (2026-11-04). Replaces ADR 0025 once the timer is removed (step
3 below); until then both may run, which is harmless (see Decision).

## Context

Since 0.1.0 a systemd user timer runs `kisetsu rotate --scheduled`
(ADR 0025). It works without any resident process, but:

- it exists only on Linux with systemd; Windows (planned for 0.3.0)
  would need a second mechanism (Task Scheduler);
- the systemd user manager keeps the environment of the first session,
  which led to ADR 0031 and `KISETSU_DESKTOP_FALLBACK`;
- a tray icon (planned) and live overlays (`docs/ideas.md`) need a
  process that keeps running anyway.

## Decision

`kisetsu agent` is a small process without a window or WebView, started
at login (XDG autostart; the HKCU Run key on Windows later). The window
stays a separate process, started on demand: keeping a WebView resident
would cost far more memory.

**Scheduling.** The agent keeps no schedule. Every 30 seconds it reads
the rotation settings and the time of the last wallpaper change from the
database (`wallpaper_history`), and changes the wallpaper when
`last change + interval` has passed (at once if there was never a
change). Therefore:

- changes made in the window apply within 30 seconds, without any
  connection between the processes (no socket, no local server);
- a manual change ("Set as wallpaper", "Next wallpaper") restarts the
  countdown, because it is the last change;
- after sleep, or when the clock changes, the next tick catches up with
  one change, never a burst;
- the old timer and the agent can run side by side during the move: a
  change by either is the last change for both.

Reading two small rows every 30 seconds costs nothing measurable, so the
`PRAGMA data_version` shortcut considered in the plan is not needed.

**Stopping.** The agent exits by itself at the next tick once rotation
is off (later: and the tray icon is off), so turning rotation off needs
no signal between the processes.

**Coordination.** The change runs under the existing rotation lock
(`rotate.lock`), shared with the window and `kisetsu rotate`. Under the
lock the agent checks again whether the change is still due, so a manual
change made a moment earlier is not followed by another one. Only one
agent runs per user: it holds `agent.lock` (package `filelock`, which the
rotation lock now also uses) for its whole life; a second one exits at
once.

**Failures.** After a failed change (no images, an unsupported desktop)
the agent waits 5 minutes, or the interval if shorter, before trying
again, and logs each attempt. It never exits because of a failed change.

## Steps

1. The agent and `kisetsu agent` (this ADR).
2. Autostart (`internal/autostart`: an XDG entry with `TryExec`, so it is
   skipped once Kisetsu is uninstalled; only an entry Kisetsu wrote is
   changed), the agent state in the Wallpaper tab with a Start button,
   and the move from 0.1.0: when the window opens, or rotation settings
   are saved, it sets up the agent and then removes the Kisetsu systemd
   timer; if the agent cannot be set up, the timer is kept. The window
   saves the settings before starting the agent, because an agent with
   rotation off exits at once. It finds out whether the agent runs by
   taking `agent.lock` for an instant, so a starting agent tries its
   lock a few times before it concludes that another one runs.
3. Remove what only the timer needed: `internal/schedule` becomes
   `internal/legacytimer` (find Kisetsu's unit files, remove them), and
   `KISETSU_DESKTOP_FALLBACK` goes. `kisetsu rotate --scheduled` stays,
   undocumented in the help: after an upgrade the old timer keeps
   calling it until the window first opens and replaces the timer.
   Starting the agent from that call was considered, but systemd ends
   every process of a service when the service finishes, the agent
   included.
4. Tray icon (ADR 0042).

## Consequences

- One resident process of about 15 MB while rotation or the tray is on.
- If the agent crashes, rotation stops until the next login or until it
  is started again from the Wallpaper tab. Restarting it automatically is
  noted in `docs/ideas.md`.
- Desktops that ignore XDG autostart (i3, sway) need `kisetsu agent` in
  their own startup file.
