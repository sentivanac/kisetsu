package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/rotation"
)

var start = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

// fake is the rest of Kisetsu as the agent sees it: settings, a history of
// changes, a lock and a clock.
type fake struct {
	cfg     rotation.Settings
	last    time.Time
	changed bool
	now     time.Time
	locked  bool
	rotated int
	rotate  error
	// beforeLock runs when the lock is taken, e.g. a manual change that
	// happened in the meantime.
	beforeLock func()
}

func newAgent(f *fake) *Agent {
	return &Agent{
		Settings: func(context.Context) (rotation.Settings, error) { return f.cfg, nil },
		LastChange: func(context.Context) (time.Time, bool, error) {
			return f.last, f.changed, nil
		},
		Lock: func() (func(), error) {
			if f.locked {
				return nil, rotation.ErrLocked
			}
			if f.beforeLock != nil {
				f.beforeLock()
			}
			return func() {}, nil
		},
		Rotate: func(context.Context) error {
			if f.rotate != nil {
				return f.rotate
			}
			f.rotated++
			f.last, f.changed = f.now, true
			return nil
		},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return f.now },
	}
}

func hourly() rotation.Settings {
	return rotation.Settings{Enabled: true, Interval: rotation.Hourly, Order: rotation.Random}
}

func TestDue(t *testing.T) {
	if _, on := Due(rotation.Settings{Interval: rotation.Hourly}, start, true); on {
		t.Error("due while rotation is off")
	}
	if at, on := Due(hourly(), time.Time{}, false); !on || !at.IsZero() {
		t.Errorf("never changed: %v, %v; want due at once", at, on)
	}
	if at, _ := Due(hourly(), start, true); !at.Equal(start.Add(time.Hour)) {
		t.Errorf("due at %v, want an hour after the last change", at)
	}
}

func TestChangesWhenDue(t *testing.T) {
	f := &fake{cfg: hourly(), last: start, changed: true, now: start.Add(59 * time.Minute)}
	a := newAgent(f)

	a.Check(context.Background())
	if f.rotated != 0 {
		t.Fatal("changed before the interval passed")
	}
	f.now = start.Add(time.Hour)
	a.Check(context.Background())
	if f.rotated != 1 {
		t.Fatalf("rotated %d times at the interval, want 1", f.rotated)
	}
	// The next change counts from this one.
	f.now = start.Add(time.Hour + 30*time.Minute)
	a.Check(context.Background())
	if f.rotated != 1 {
		t.Error("changed again before a full interval")
	}
}

// After sleep or a clock jump, one change catches up; there is no burst.
func TestCatchesUpOnceAfterSleep(t *testing.T) {
	f := &fake{cfg: hourly(), last: start, changed: true, now: start.Add(10 * time.Hour)}
	a := newAgent(f)
	a.Check(context.Background())
	a.Check(context.Background())
	if f.rotated != 1 {
		t.Errorf("rotated %d times after waking up, want 1", f.rotated)
	}
}

func TestDoesNothingWhileOff(t *testing.T) {
	f := &fake{cfg: rotation.Settings{Interval: rotation.Hourly}, now: start}
	newAgent(f).Check(context.Background())
	if f.rotated != 0 {
		t.Error("changed while rotation is off")
	}
}

func TestSkipsWhileAnotherChangeRuns(t *testing.T) {
	f := &fake{cfg: hourly(), now: start, locked: true}
	newAgent(f).Check(context.Background())
	if f.rotated != 0 {
		t.Error("changed without the lock")
	}
}

// A manual change just before the lock was taken restarts the countdown.
func TestRechecksUnderTheLock(t *testing.T) {
	f := &fake{cfg: hourly(), last: start, changed: true, now: start.Add(2 * time.Hour)}
	f.beforeLock = func() { f.last = f.now }
	newAgent(f).Check(context.Background())
	if f.rotated != 0 {
		t.Error("changed right after a manual change")
	}
}

func TestWaitsAfterAFailure(t *testing.T) {
	f := &fake{cfg: hourly(), now: start, rotate: rotation.ErrNoImages}
	a := newAgent(f)
	a.Check(context.Background())

	f.rotate = nil
	f.now = start.Add(4 * time.Minute)
	a.Check(context.Background())
	if f.rotated != 0 {
		t.Fatal("retried before the retry delay")
	}
	f.now = start.Add(retryDelay)
	a.Check(context.Background())
	if f.rotated != 1 {
		t.Errorf("rotated %d times after the retry delay, want 1", f.rotated)
	}
}

func TestRetryDelayIsAtMostTheInterval(t *testing.T) {
	cfg := hourly()
	cfg.Interval = rotation.Every15Minutes
	f := &fake{cfg: cfg, now: start, rotate: errors.New("unsupported desktop")}
	a := newAgent(f)
	a.Check(context.Background())
	if want := start.Add(retryDelay); !a.retryAt.Equal(want) {
		t.Errorf("retry at %v, want %v (15 minutes is longer than the delay)", a.retryAt, want)
	}
}

func TestRunStopsWhenCanceled(t *testing.T) {
	f := &fake{cfg: hourly(), now: start}
	a := newAgent(f)
	a.Tick = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- a.Run(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
	if f.rotated != 1 {
		t.Errorf("rotated %d times, want 1 (checked at once, then not due)", f.rotated)
	}
}

func TestRunExitsWhenNotWanted(t *testing.T) {
	f := &fake{cfg: hourly(), now: start}
	a := newAgent(f)
	a.Tick = time.Millisecond
	wanted := true
	var mu sync.Mutex
	a.Wanted = func(context.Context) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		return wanted, nil
	}
	done := make(chan error)
	go func() { done <- a.Run(context.Background()) }()
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	wanted = false
	mu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after rotation was turned off")
	}
}
