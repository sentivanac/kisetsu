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
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/sentivanac/kisetsu/internal/agent"
	"github.com/sentivanac/kisetsu/internal/filelock"
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

	a := newAgent(c, c.rotator(c.fetcher(searcher, dl)))
	if wanted, err := a.Wanted(ctx); err == nil && !wanted {
		c.log.Info("rotation and the tray icon are off; agent not started")
		fmt.Println("Rotation and the tray icon are off, so the agent has nothing to do. " +
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

	// The agent belongs to the login session: when the session's D-Bus
	// goes away (logout), it ends too, instead of lingering.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if conn, err := dbus.ConnectSessionBus(); err == nil {
		defer conn.Close()
		go func() {
			select {
			case <-conn.Context().Done():
				c.log.Info("session bus closed; agent exits")
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	if !prefs.ShowInTray {
		a.Run(ctx)
		c.log.Info("agent stopped")
		return 0
	}

	// The tray runs on the main goroutine; the schedule beside it. Either
	// ending ends the other.
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.Run(ctx)
		cancel()
	}()
	runTray(ctx, c.log, trayActions{
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
		open: openWindow,
	}, cancel)
	cancel()
	<-done
	c.log.Info("agent stopped")
	return 0
}

// openWindow starts the Kisetsu window in its own session; if one is open
// already, it comes to the front instead (single instance).
func openWindow() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// newAgent connects the agent to the database and the rotator.
func newAgent(c *core, r *rotation.Rotator) *agent.Agent {
	store := settings.NewStore(c.db)
	history := rotation.NewHistory(c.db)
	return &agent.Agent{
		Settings: func(ctx context.Context) (rotation.Settings, error) {
			return rotation.LoadSettings(ctx, store)
		},
		LastChange: func(ctx context.Context) (time.Time, bool, error) {
			last, ok, err := history.Last(ctx)
			return last.At, ok, err
		},
		Lock: func() (func(), error) { return rotation.Lock(c.rotationLockPath()) },
		Rotate: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, rotateTimeout)
			defer cancel()
			_, err := r.Next(ctx)
			return err
		},
		Wanted: func(ctx context.Context) (bool, error) {
			cfg, err := rotation.LoadSettings(ctx, store)
			if err != nil {
				return true, err
			}
			prefs, err := agent.LoadPrefs(ctx, store)
			return cfg.Enabled || prefs.ShowInTray, err
		},
		Log: c.log,
	}
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
