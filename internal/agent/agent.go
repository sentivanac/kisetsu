// Package agent runs Kisetsu's background work while the user is logged
// in: it changes the wallpaper when rotation is due (ADR 0041).
//
// The agent keeps no schedule of its own. Every tick it reads the rotation
// settings and the time of the last wallpaper change from the database,
// so changes made in the window apply within a tick, a manual change
// restarts the countdown, and time spent asleep or a changed clock is
// simply caught up on the next tick.
package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/sentivanac/kisetsu/internal/rotation"
)

const (
	// DefaultTick is how often the agent checks whether a change is due.
	DefaultTick = 30 * time.Second
	// retryDelay is the pause after a failed change (no images, an
	// unsupported desktop), so a lasting problem is not retried every tick.
	// It is never longer than the rotation interval.
	retryDelay = 5 * time.Minute
)

// Agent changes the wallpaper on schedule. The functions are its view of
// the rest of Kisetsu, so tests can replace them.
type Agent struct {
	// Settings returns the rotation settings.
	Settings func(ctx context.Context) (rotation.Settings, error)
	// LastChange returns when the wallpaper last changed (by rotation or
	// by the user), and false if it never did.
	LastChange func(ctx context.Context) (time.Time, bool, error)
	// Lock takes the rotation lock, shared with the window and commands;
	// rotation.ErrLocked means someone else is changing the wallpaper.
	Lock func() (unlock func(), err error)
	// Rotate changes the wallpaper to the next image.
	Rotate func(ctx context.Context) error
	// Wanted reports whether the agent has anything to do (rotation is
	// on). When it has not, Run returns: turning rotation off in the
	// window stops the agent without any signal. Nil means always.
	Wanted func(ctx context.Context) (bool, error)

	Log  *slog.Logger
	Now  func() time.Time // nil means time.Now
	Tick time.Duration    // 0 means DefaultTick

	// retryAt is when to try again after a failure.
	retryAt time.Time
}

// Due returns when the next change is due, and false when rotation is off.
// Without any change yet, it is due at once.
func Due(cfg rotation.Settings, last time.Time, changed bool) (time.Time, bool) {
	if !cfg.Enabled {
		return time.Time{}, false
	}
	interval, err := cfg.Interval.Duration()
	if err != nil {
		return time.Time{}, false
	}
	if !changed {
		return time.Time{}, true
	}
	return last.Add(interval), true
}

// Run checks at once and then every tick, until ctx is canceled or the
// agent is no longer wanted. If Wanted cannot be read, the agent keeps
// running.
func (a *Agent) Run(ctx context.Context) error {
	tick := a.Tick
	if tick <= 0 {
		tick = DefaultTick
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		if a.Wanted != nil {
			if wanted, err := a.Wanted(ctx); err == nil && !wanted {
				a.Log.Info("nothing to do; agent exits")
				return nil
			}
		}
		a.Check(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// Check changes the wallpaper if a change is due. Problems are logged; the
// agent keeps running.
func (a *Agent) Check(ctx context.Context) {
	now := a.now()
	if now.Before(a.retryAt) {
		return
	}
	if due, err := a.due(ctx, now); err != nil || !due {
		if err != nil {
			a.Log.Warn("cannot tell whether a wallpaper change is due", "err", err)
		}
		return
	}

	unlock, err := a.Lock()
	if errors.Is(err, rotation.ErrLocked) {
		a.Log.Debug("another wallpaper change is in progress; skipping")
		return
	}
	if err != nil {
		a.Log.Error("rotation lock", "err", err)
		return
	}
	defer unlock()

	// The user may have changed the wallpaper just before the lock was
	// taken; that restarts the countdown.
	if due, err := a.due(ctx, now); err != nil || !due {
		return
	}
	if err := a.Rotate(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		cfg, _ := a.Settings(ctx)
		delay := retryDelay
		if interval, err := cfg.Interval.Duration(); err == nil && interval < delay {
			delay = interval
		}
		a.retryAt = now.Add(delay)
		if errors.Is(err, rotation.ErrNoImages) {
			a.Log.Warn("no images to rotate", "retry_in", delay)
		} else {
			a.Log.Error("wallpaper change failed", "err", err, "retry_in", delay)
		}
	}
}

func (a *Agent) due(ctx context.Context, now time.Time) (bool, error) {
	cfg, err := a.Settings(ctx)
	if err != nil {
		return false, err
	}
	last, changed, err := a.LastChange(ctx)
	if err != nil {
		return false, err
	}
	at, on := Due(cfg, last, changed)
	return on && !now.Before(at), nil
}

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}
