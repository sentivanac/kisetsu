package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/schedule"
)

// fakeSchedule stands in for systemd.
type fakeSchedule struct {
	enabled   bool
	interval  time.Duration
	enableErr error
	exe       string
	restarts  int
}

func (f *fakeSchedule) Restart(ctx context.Context) error {
	if f.enabled {
		f.restarts++
	}
	return nil
}

func (f *fakeSchedule) Enable(ctx context.Context, interval time.Duration) error {
	if f.enableErr != nil {
		return f.enableErr
	}
	f.enabled, f.interval, f.exe = true, interval, "/opt/kisetsu/kisetsu"
	return nil
}

func (f *fakeSchedule) Disable(ctx context.Context) error {
	f.enabled = false
	return nil
}

func (f *fakeSchedule) Status(ctx context.Context) (schedule.Status, error) {
	if !f.enabled {
		return schedule.Status{}, nil
	}
	return schedule.Status{Installed: true, Active: true, NextRun: time.Now().Add(f.interval), Executable: f.exe}, nil
}

func TestSaveRotationInstallsAndRemovesTheTimer(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.schedule.(*fakeSchedule)

	state, err := app.GetRotation()
	if err != nil {
		t.Fatal(err)
	}
	if state.Settings.Enabled || state.Settings.Interval != "1h" || len(state.Intervals) != 5 || state.Timer.Installed {
		t.Errorf("initial state = %+v", state)
	}

	state, err = app.SaveRotation(RotationSettings{Enabled: true, Interval: "15m", Order: "sequential"})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.enabled || fake.interval != 15*time.Minute {
		t.Errorf("timer = %+v", fake)
	}
	if !state.Settings.Enabled || !state.Timer.Active || state.Timer.NextRun == "" || state.Timer.Outdated {
		t.Errorf("state after enabling = %+v", state)
	}

	state, err = app.SaveRotation(RotationSettings{Enabled: false, Interval: "15m", Order: "sequential"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.enabled || state.Settings.Enabled || state.Timer.Installed {
		t.Errorf("after disabling: timer %+v, state %+v", fake, state)
	}
}

func TestFailedTimerLeavesRotationOff(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.schedule.(*fakeSchedule)
	fake.enableErr = schedule.ErrForeignUnit

	_, err := app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	if pub := apperr.ToPublic(err); pub.Code != apperr.InvalidInput || !strings.Contains(pub.Message, "not created by Kisetsu") {
		t.Errorf("error = %+v", pub)
	}
	cfg, _ := rotation.LoadSettings(app.ctx, app.settings)
	if cfg.Enabled {
		t.Error("rotation is saved as enabled although the timer failed")
	}

	fake.enableErr = schedule.ErrTemporaryExecutable
	_, err = app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	if !strings.Contains(apperr.ToPublic(err).Message, "temporary build") {
		t.Errorf("temporary build: %v", err)
	}
}

func TestSaveRotationRejectsInvalidSettings(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	_, err := app.SaveRotation(RotationSettings{Enabled: true, Interval: "5s", Order: "random"})
	if apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("err = %v", err)
	}
	if app.schedule.(*fakeSchedule).enabled {
		t.Error("timer installed for invalid settings")
	}
}

func TestOutdatedTimerIsReportedAndRepaired(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.schedule.(*fakeSchedule)
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	fake.exe = "/old/place/kisetsu" // Kisetsu was moved since

	state, _ := app.GetRotation()
	if !state.Timer.Outdated {
		t.Fatal("outdated timer not reported")
	}
	state, err := app.RepairRotationTimer()
	if err != nil || state.Timer.Outdated {
		t.Errorf("after repair: %+v, %v", state.Timer, err)
	}
}

func TestNextWallpaper(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	_, err := app.NextWallpaper()
	if pub := apperr.ToPublic(err); pub.Code != apperr.InvalidInput || !strings.Contains(pub.Message, "no images") {
		t.Errorf("empty library: %+v", pub)
	}

	ids := downloadN(t, app, 2)
	state, err := app.NextWallpaper()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastImageID != ids[0] && state.LastImageID != ids[1] || state.LastChange == "" {
		t.Errorf("state = %+v", state)
	}

	unlock, err := rotation.Lock(app.rotationLock)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := app.NextWallpaper(); !strings.Contains(apperr.ToPublic(err).Message, "already being changed") {
		t.Errorf("while locked: %v", err)
	}
}

func TestVarietyWarning(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.wallpaper.(*fakeWallpaper).variety = true

	state, err := app.GetRotation()
	if err != nil || !state.VarietyRunning {
		t.Errorf("VarietyRunning = %v, %v", state.VarietyRunning, err)
	}
}

func TestManualChangesRestartTheTimer(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.schedule.(*fakeSchedule)
	ids := downloadN(t, app, 2)

	// Rotation off: nothing to restart.
	app.NextWallpaper()
	if fake.restarts != 0 {
		t.Errorf("restarted with rotation off: %d", fake.restarts)
	}

	app.SaveRotation(RotationSettings{Enabled: true, Interval: "15m", Order: "random"})
	if _, err := app.NextWallpaper(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetWallpaper(ids[0]); err != nil {
		t.Fatal(err)
	}
	if fake.restarts != 2 {
		t.Errorf("restarts = %d, want 2 (Next wallpaper and Set as wallpaper)", fake.restarts)
	}
}
