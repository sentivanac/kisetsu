package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sentivanac/kisetsu/internal/agent"
	"github.com/sentivanac/kisetsu/internal/filelock"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/settings"
)

// runAgent runs the background agent until it is stopped (Ctrl+C,
// SIGTERM or the end of the session). Only one agent runs per user.
func runAgent(args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "kisetsu agent: unexpected argument %q\n", args[0])
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c, err := openCore(ctx, "agent", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu agent:", err)
		return 1
	}
	defer c.Close()

	release, err := lockAgent(c.agentLockPath())
	if errors.Is(err, filelock.ErrLocked) {
		fmt.Println("The Kisetsu agent is already running.")
		return 0
	}
	if err != nil {
		c.log.Error("agent lock", "err", err)
		fmt.Fprintln(os.Stderr, "kisetsu agent:", err)
		return 1
	}
	defer release()
	// The window reads the PID to stop the agent (for example to show or
	// hide the tray icon); the lock tells it the PID is current.
	if err := os.WriteFile(c.agentLockPath(), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		c.log.Warn("agent PID not written", "err", err)
	}

	// A saved search as rotation source may need new images first.
	client := httpClient()
	searcher, err := c.searchService(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu agent:", err)
		return 1
	}
	dl, err := c.downloader(client, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kisetsu agent:", err)
		return 1
	}
	defer dl.Close()

	screen := c.rotationScreen()
	screen.KeepBase = true      // it redraws the clock every minute
	screen.ReleaseMemory = true // it runs all session
	a := newAgent(c, c.rotator(c.fetcher(searcher, dl), screen), screen)
	if wanted, err := a.Wanted(ctx); err == nil && !wanted {
		c.log.Info("rotation, the clock and the tray icon are off; agent not started")
		fmt.Println("Rotation, the clock and the tray icon are off, so the agent has nothing to do. " +
			"Turn one of them on in the Wallpaper tab.")
		return 0
	}
	store := settings.NewStore(c.db)
	prefs, err := agent.LoadPrefs(ctx, store)
	if err != nil {
		c.log.Warn("agent preferences not read; no tray icon", "err", err)
	}
	c.log.Info("agent started", "tray", prefs.ShowInTray)
	fmt.Println("Kisetsu agent is running. Press Ctrl+C to stop it.")

	// The agent belongs to the login session and ends with it.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer watchSession(ctx, cancel, c.log)()

	if !prefs.ShowInTray {
		a.Run(ctx)
		c.log.Info("agent stopped")
		return 0
	}

	// The tray runs on the main goroutine; the schedule beside it. Either
	// ending ends the other.
	done := make(chan struct{})
	// quit: the user chose Quit in the tray menu, rather than the session
	// ending.
	var quit atomic.Bool
	go func() {
		defer close(done)
		a.Run(ctx)
		cancel()
	}()
	runTray(ctx, c.log, filepath.Join(c.paths.Cache, "tray-icons"), trayActions{
		next: func(ctx context.Context) error {
			ctx, stop := context.WithTimeout(ctx, rotateTimeout)
			defer stop()
			return a.ChangeNow(ctx)
		},
		rotating: func(ctx context.Context) (bool, error) {
			cfg, err := rotation.LoadSettings(ctx, store)
			return cfg.Enabled, err
		},
		setRotating: func(ctx context.Context, on bool) error {
			cfg, err := rotation.LoadSettings(ctx, store)
			if err != nil {
				return err
			}
			cfg.Enabled = on
			c.log.Info("rotation turned on or off from the tray", "enabled", on)
			return rotation.SaveSettings(ctx, store, cfg)
		},
		open:  func() error { return openWindow() },
		about: func() error { return openWindow(aboutArg) },
	}, func() {
		quit.Store(true)
		cancel()
	})
	cancel()
	<-done
	if quit.Load() {
		// The clock stops with the agent; its last time must not stay on
		// the desktop until the next login.
		removeClock(c, screen, rotation.NewHistory(c.db), func() (func(), error) {
			return rotation.Lock(c.rotationLockPath())
		})
	}
	c.log.Info("agent stopped")
	return 0
}

// openWindow starts the Kisetsu window in its own session, with args; if
// one is open already, it comes to the front instead (single instance) and
// gets the args.
func openWindow(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// newAgent connects the agent to the database, the rotator and the
// screen that draws the clock.
func newAgent(c *core, r *rotation.Rotator, screen *onscreen.Service) *agent.Agent {
	store := settings.NewStore(c.db)
	history := rotation.NewHistory(c.db)
	lock := func() (func(), error) { return rotation.Lock(c.rotationLockPath()) }
	return &agent.Agent{
		Settings: func(ctx context.Context) (rotation.Settings, error) {
			return rotation.LoadSettings(ctx, store)
		},
		LastChange: func(ctx context.Context) (time.Time, bool, error) {
			last, ok, err := history.Last(ctx)
			return last.At, ok, err
		},
		Lock: lock,
		Rotate: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, rotateTimeout)
			defer cancel()
			_, err := r.Next(ctx)
			return err
		},
		Wanted: func(ctx context.Context) (bool, error) {
			return agentWanted(ctx, store)
		},
		Refresh: func(ctx context.Context) {
			refreshClock(ctx, c, screen, history, lock)
		},
		Log: c.log,
	}
}

// agentWanted reports whether the agent has anything to do: rotation, the
// clock, the text file or the tray icon is on.
func agentWanted(ctx context.Context, store *settings.Store) (bool, error) {
	cfg, err := rotation.LoadSettings(ctx, store)
	if err != nil {
		return true, err
	}
	text, err := onscreen.LoadSettings(ctx, store)
	if err != nil {
		return true, err
	}
	prefs, err := agent.LoadPrefs(ctx, store)
	return cfg.Enabled || text.Live() || prefs.ShowInTray, err
}

// refreshClock shows the current wallpaper again when the clock's text or
// the text file changed. A wallpaper change in progress is left alone: it
// draws them as well.
func refreshClock(ctx context.Context, c *core, screen *onscreen.Service, history *rotation.History, lock func() (func(), error)) {
	if !screen.RedrawDue(ctx) {
		return
	}
	unlock, err := lock()
	if err != nil {
		if !errors.Is(err, rotation.ErrLocked) {
			c.log.Warn("clock not redrawn", "err", err)
		}
		return
	}
	defer unlock()
	e, path, ok, err := currentWallpaper(ctx, history, c.lib, c.files)
	if err != nil || !ok {
		// Tried again at the next minute, not every few seconds.
		screen.SkipRedraw(ctx)
		if err != nil {
			c.log.Warn("clock not redrawn", "err", err)
		}
		return // without an error: Kisetsu never set a wallpaper
	}
	if _, err := screen.Show(ctx, e, path); err != nil {
		c.log.Warn("clock not redrawn", "entry", e.ID, "err", err)
	}
}

// removeClockTimeout bounds removeClock: a wallpaper change in progress
// (which holds the rotation lock) is waited for, briefly.
const removeClockTimeout = 15 * time.Second

// removeClock shows the current wallpaper again without the clock and the
// text file, which nothing updates once the agent ends, with the author,
// license and effects if they are on. It does nothing when both are off,
// when Variety sets the wallpaper (Kisetsu drew neither), or when Kisetsu
// never set a wallpaper.
func removeClock(c *core, screen *onscreen.Service, history *rotation.History, lock func() (func(), error)) {
	ctx, cancel := context.WithTimeout(context.Background(), removeClockTimeout)
	defer cancel()
	cfg, err := onscreen.LoadSettings(ctx, screen.Settings)
	if err != nil || !cfg.Live() || screen.Setter.ViaVariety() {
		return
	}
	var unlock func()
	for {
		unlock, err = lock()
		if !errors.Is(err, rotation.ErrLocked) {
			break
		}
		select {
		case <-ctx.Done():
			c.log.Warn("clock not removed: a wallpaper change is still running")
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	if err != nil {
		c.log.Warn("clock not removed", "err", err)
		return
	}
	defer unlock()
	e, path, ok, err := currentWallpaper(ctx, history, c.lib, c.files)
	if err != nil || !ok {
		if err != nil {
			c.log.Warn("clock not removed", "err", err)
		}
		return
	}
	if _, err := screen.ShowWithoutLive(ctx, e, path); err != nil {
		c.log.Warn("clock not removed", "entry", e.ID, "err", err)
		return
	}
	c.log.Info("clock removed from the wallpaper", "entry", e.ID)
}

// lockAttempts: the window checks whether the agent runs by taking its
// lock for an instant, so a starting agent tries a few times before it
// concludes that another agent runs.
const lockAttempts = 10

func lockAgent(path string) (release func(), err error) {
	for range lockAttempts {
		release, err = filelock.Lock(path)
		if !errors.Is(err, filelock.ErrLocked) {
			return release, err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// agentLockPath is held by the running agent for its whole life.
func (c *core) agentLockPath() string {
	return filepath.Join(c.paths.State, "agent.lock")
}
