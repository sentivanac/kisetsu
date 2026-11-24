package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/sentivanac/kisetsu/internal/filelock"
)

// closeArg closes the open window, if there is one; the tray's Quit uses
// it. It reaches the window as a second instance (like aboutArg).
const closeArg = "--close-window"

// closeWindowWait bounds how long Quit waits for the window to be told.
const closeWindowWait = 3 * time.Second

// agentRecheck is when the window looks again whether the agent runs: one
// that was just told to quit still holds its lock for a moment.
const agentRecheck = 5 * time.Second

// windowLockPath is held by the open window for its whole life.
func (c *core) windowLockPath() string {
	return filepath.Join(c.paths.State, "window.lock")
}

// windowOpen reports whether a window holds its lock.
func windowOpen(path string) bool {
	release, err := filelock.Lock(path)
	if errors.Is(err, filelock.ErrLocked) {
		return true
	}
	if err == nil {
		release()
	}
	return false
}

// closeWindow tells the open window to close and waits until it was told,
// so it cannot start the agent again. Without a window it does nothing.
func closeWindow(ctx context.Context, lockPath string) error {
	if !windowOpen(lockPath) {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, closeWindowWait)
	defer cancel()
	err = exec.CommandContext(ctx, exe, closeArg).Run()
	// Wails ends a second instance with status 1 on Linux.
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	return err
}

// secondInstanceAction is what the window does when Kisetsu is started
// again with args.
func secondInstanceAction(args []string) (closing, about bool) {
	return slices.Contains(args, closeArg), slices.Contains(args, aboutArg)
}

// ensureAgent starts the agent when it has something to do but does not
// run: after Quit in the tray, opening Kisetsu brings all of it back. The
// autostart entry is left as it is. Problems are only logged; the
// Wallpaper tab shows whether the agent runs.
func (a *App) ensureAgent() {
	if a.closing.Load() {
		return
	}
	wanted, err := agentWanted(a.ctx, a.settings)
	if err == nil && wanted {
		err = a.agent.Start()
	}
	if err != nil {
		a.log.Warn("agent not started with the window", "err", err)
	}
}
