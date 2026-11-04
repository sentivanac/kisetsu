package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
		c.log.Info("rotation is off; agent not started")
		fmt.Println("Rotation is off, so the agent has nothing to do. Turn it on in the Wallpaper tab.")
		return 0
	}
	c.log.Info("agent started")
	fmt.Println("Kisetsu agent is running. Press Ctrl+C to stop it.")
	a.Run(ctx)
	c.log.Info("agent stopped")
	return 0
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
			return cfg.Enabled, err
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
