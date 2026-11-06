package main

import (
	"context"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/settings"
)

func TestRefreshClock(t *testing.T) {
	c, r, fake := testCore(t, 2)
	ctx := context.Background()
	screen := r.Screen.(*onscreen.Service)
	now := time.Date(2026, 11, 5, 14, 7, 0, 0, time.Local)
	screen.Now = func() time.Time { return now }
	history := rotation.NewHistory(c.db)
	lock := func() (func(), error) { return rotation.Lock(c.rotationLockPath()) }

	cfg := onscreen.DefaultSettings
	cfg.Clock = true
	if err := onscreen.SaveSettings(ctx, settings.NewStore(c.db), cfg); err != nil {
		t.Fatal(err)
	}

	// No wallpaper set by Kisetsu yet: nothing to draw on.
	refreshClock(ctx, c, screen, history, lock)
	if fake.path != "" {
		t.Fatalf("set %q before Kisetsu set any wallpaper", fake.path)
	}
	if screen.ClockDue(ctx) {
		t.Error("tried again within the same minute")
	}

	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	fake.path = ""
	refreshClock(ctx, c, screen, history, lock)
	if fake.path != "" {
		t.Errorf("redrawn within the minute of the change: %q", fake.path)
	}

	now = now.Add(time.Minute)
	refreshClock(ctx, c, screen, history, lock)
	if fake.path == "" {
		t.Error("not redrawn in a new minute")
	}

	// A change in progress holds the lock; the clock waits for it.
	now = now.Add(time.Minute)
	fake.path = ""
	unlock, err := lock()
	if err != nil {
		t.Fatal(err)
	}
	refreshClock(ctx, c, screen, history, lock)
	unlock()
	if fake.path != "" {
		t.Errorf("redrawn while the rotation lock was held: %q", fake.path)
	}
}

func TestAgentWanted(t *testing.T) {
	c, _, _ := testCore(t, 0)
	ctx := context.Background()
	store := settings.NewStore(c.db)
	if wanted, err := agentWanted(ctx, store); err != nil || wanted {
		t.Fatalf("defaults: %v, %v", wanted, err)
	}
	cfg := onscreen.DefaultSettings
	cfg.Credit = true
	if err := onscreen.SaveSettings(ctx, store, cfg); err != nil {
		t.Fatal(err)
	}
	if wanted, _ := agentWanted(ctx, store); wanted {
		t.Error("wanted for the credit, which needs no redrawing")
	}
	cfg.Clock = true
	if err := onscreen.SaveSettings(ctx, store, cfg); err != nil {
		t.Fatal(err)
	}
	if wanted, _ := agentWanted(ctx, store); !wanted {
		t.Error("not wanted with the clock on")
	}
}
