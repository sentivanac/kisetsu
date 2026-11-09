package main

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
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

func TestRemoveClock(t *testing.T) {
	c, r, fake := testCore(t, 2)
	ctx := context.Background()
	screen := r.Screen.(*onscreen.Service)
	history := rotation.NewHistory(c.db)
	lock := func() (func(), error) { return rotation.Lock(c.rotationLockPath()) }
	store := settings.NewStore(c.db)
	// testCore's files are not images; drawing needs real ones.
	for n := range 2 {
		writeJPEG(t, filepath.Join(c.paths.Data, "images", "test", fmt.Sprintf("%d.jpg", n)))
	}

	// The clock off: nothing to remove.
	removeClock(c, screen, history, lock)
	if fake.path != "" {
		t.Fatalf("set %q with the clock off", fake.path)
	}

	cfg := onscreen.DefaultSettings
	cfg.Clock = true
	if err := onscreen.SaveSettings(ctx, store, cfg); err != nil {
		t.Fatal(err)
	}
	// No wallpaper set by Kisetsu: nothing to show.
	removeClock(c, screen, history, lock)
	if fake.path != "" {
		t.Fatalf("set %q before Kisetsu set a wallpaper", fake.path)
	}

	if _, err := r.Next(ctx); err != nil {
		t.Fatal(err)
	}
	composed := fake.path
	if !strings.HasPrefix(composed, filepath.Join(c.paths.Cache, "overlay")) {
		t.Fatalf("with the clock on, set %q, want a composed image", composed)
	}
	_, original, _, err := currentWallpaper(ctx, history, c.lib, c.files)
	if err != nil {
		t.Fatal(err)
	}

	// Only the clock: the original image comes back.
	removeClock(c, screen, history, lock)
	if fake.path != original {
		t.Errorf("after removing the clock, set %q, want the original %q", fake.path, original)
	}

	// With the author on, a new composed image without the clock.
	cfg.Credit = true
	if err := onscreen.SaveSettings(ctx, store, cfg); err != nil {
		t.Fatal(err)
	}
	removeClock(c, screen, history, lock)
	if fake.path == original || fake.path == composed || !strings.HasPrefix(fake.path, filepath.Join(c.paths.Cache, "overlay")) {
		t.Errorf("with the author on, set %q, want a new composed image", fake.path)
	}
}

func writeJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}
