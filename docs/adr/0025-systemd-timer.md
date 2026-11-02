# ADR 0025: systemd user timer for rotation

- **Status:** accepted
- **Date:** 2026-11-02
- Third step of Phase 6; the "Wallpaper" tab (next step) turns it on and off.

## Decision

`internal/schedule.Manager` installs two user units in
`~/.config/systemd/user/`:

```ini
# kisetsu-rotate.service
# Created by Kisetsu. Changes are overwritten when rotation settings change.
[Service]
Type=oneshot
ExecStart="/path/to/kisetsu" rotate --scheduled
Environment="KISETSU_DESKTOP_FALLBACK=MATE"

# kisetsu-rotate.timer
[Timer]
OnActiveSec=3600s
OnUnitActiveSec=3600s
AccuracySec=30s
[Install]
WantedBy=timers.target
```

- **Enable**: write both files atomically (temporary file + rename, mode
  0644), then `systemctl --user daemon-reload`, `enable` and `restart` the
  timer (restart, so a changed interval applies at once).
- **Disable**: `systemctl --user disable --now`, remove both files,
  `daemon-reload`. Missing files are fine.
- **Status**: reads the installed `ExecStart` and asks `systemctl --user
  show` for `ActiveState`, `NextElapseUSecRealtime` and `LastTriggerUSec`
  (printed as `Mon 2026-11-02 09:21:29 CET`, parsed in the local zone).
  Kisetsu's timer is *monotonic*, and for monotonic timers
  `NextElapseUSecRealtime` is empty; `NextElapseUSecMonotonic` counts from
  boot without suspended time and cannot be turned into a clock time
  locally. The next run is then read from the NEXT column of
  `systemctl --user list-timers --no-legend`, which systemd computes.
  (Found by the user on 2026-11-02: "Next change" never appeared; the test
  had only faked the realtime property.) systemctl runs with `LC_ALL=C`.
  `Status.Outdated` tells when the installed timer runs another binary
  than the current one (Kisetsu moved or rebuilt elsewhere).

### Safety

- Only the two files named above are ever written or removed. Each starts
  with the line `# Created by Kisetsu.`; a file of the same name without it
  is the user's and makes Enable and Disable fail without touching
  anything (`ErrForeignUnit`).
- Enabling from a temporary build (under `os.TempDir()`, e.g. `go run`) is
  refused, because the timer would point at a file that disappears.
- `ExecStart` and `Environment` values are double-quoted with systemd
  escaping: `\` and `"` escaped, `%` and `$` doubled (tested round trip
  with a path containing all of them and spaces).
- The units are created only when the user enables rotation.

### Why both `OnActiveSec` and `OnUnitActiveSec`

`OnUnitActiveSec` counts from the service's last activation and never fires
if the service has not run yet. `OnActiveSec` counts from the timer's start
(enabling, and each login), so the first change happens one interval after
that, and the following ones one interval after each change.

### Missing program (added 2026-11-02)

The service has `ConditionFileIsExecutable=<path of kisetsu>`. When
Kisetsu was uninstalled or moved while rotation was on, systemd skips
the run (`ConditionResult=no`, result `success`) instead of failing at
every interval. Condition values are taken literally, so only `%` is
escaped. Checked with `systemd-analyze --user verify` and a transient
unit with a missing path. (`ConditionPathIsExecutable` does not exist;
systemd ignores it with a warning.)

### Environment

The service sets `KISETSU_DESKTOP_FALLBACK` to the desktop found when
rotation was enabled. It is used only when the desktop cannot be found
otherwise (ADR 0031). Until 2026-11-02 the service set
`XDG_CURRENT_DESKTOP` itself, which overrode the session the user was
logged into.
The user's MATE session does export it, together with the D-Bus address
`gsettings` needs. User timers run while the user is logged in, which is
what a wallpaper needs.

## Verification

- Tests replace `systemctl` with a fake and use a temporary unit directory.
- The real unit texts, with the real executable path, were checked with
  `systemd-analyze --user verify` in a scratch directory (exit 0, no
  warnings) without installing anything. The first attempt used a wrong
  relative path, and `verify` reported the missing executable — the check
  works.
