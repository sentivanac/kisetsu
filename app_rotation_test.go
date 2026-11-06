package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/onscreen"
	"github.com/sentivanac/kisetsu/internal/rotation"
	"github.com/sentivanac/kisetsu/internal/searches"
	"github.com/sentivanac/kisetsu/internal/wallpaper"
)

// fakeAgent stands in for the agent and its autostart entry.
type fakeAgent struct {
	autostart bool
	running   bool
	exe       string
	enableErr error
	starts    int
	stops     int
}

func (f *fakeAgent) Enable() error {
	if f.enableErr != nil {
		return f.enableErr
	}
	f.autostart, f.exe = true, "/opt/kisetsu/kisetsu"
	return f.Start()
}

func (f *fakeAgent) Disable() error {
	f.autostart = false
	return nil
}

func (f *fakeAgent) Start() error {
	if !f.running {
		f.starts++
	}
	f.running = true
	return nil
}

func (f *fakeAgent) Stop() error {
	if f.running {
		f.stops++
	}
	f.running = false
	return nil
}

func (f *fakeAgent) Status() (AgentStatus, error) {
	return AgentStatus{Running: f.running, Autostart: autostart.Status{Installed: f.autostart, Executable: f.exe}}, nil
}

// fakeTimer stands in for the systemd timer of Kisetsu 0.1.0.
type fakeTimer struct{ installed bool }

func (f *fakeTimer) Installed() (bool, error) { return f.installed, nil }

func (f *fakeTimer) Disable(context.Context) error {
	f.installed = false
	return nil
}

func TestSaveRotationStartsTheAgent(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)

	state, err := app.GetRotation()
	if err != nil {
		t.Fatal(err)
	}
	if state.Settings.Enabled || state.Settings.Interval != "1h" || len(state.Intervals) != 5 ||
		state.Agent.Autostart || state.NextChange != "" {
		t.Errorf("initial state = %+v", state)
	}

	state, err = app.SaveRotation(RotationSettings{Enabled: true, Interval: "15m", Order: "sequential"})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.autostart || !fake.running {
		t.Errorf("agent = %+v", fake)
	}
	if !state.Settings.Enabled || !state.Agent.Running || !state.Agent.Autostart || state.Agent.Outdated ||
		state.NextChange == "" {
		t.Errorf("state after enabling = %+v", state)
	}

	state, err = app.SaveRotation(RotationSettings{Enabled: false, Interval: "15m", Order: "sequential"})
	if err != nil {
		t.Fatal(err)
	}
	if fake.autostart || state.Settings.Enabled || state.Agent.Autostart || state.NextChange != "" {
		t.Errorf("after disabling: agent %+v, state %+v", fake, state)
	}
}

// The next change counts from the last one, manual changes included.
func TestNextChangeCountsFromTheLastChange(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	downloadN(t, app, 2)
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})

	state, err := app.NextWallpaper()
	if err != nil {
		t.Fatal(err)
	}
	last, _ := time.Parse(time.RFC3339, state.LastChange)
	next, _ := time.Parse(time.RFC3339, state.NextChange)
	if got := next.Sub(last); got < 59*time.Minute || got > 61*time.Minute {
		t.Errorf("next change %v after the last, want an hour", got)
	}
}

func TestFailedAgentLeavesRotationOff(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	fake.enableErr = autostart.ErrForeign

	_, err := app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	if pub := apperr.ToPublic(err); pub.Code != apperr.InvalidInput || !strings.Contains(pub.Message, "not created by Kisetsu") {
		t.Errorf("error = %+v", pub)
	}
	cfg, _ := rotation.LoadSettings(app.ctx, app.settings)
	if cfg.Enabled {
		t.Error("rotation is saved as enabled although the agent failed")
	}

	fake.enableErr = autostart.ErrTemporaryExecutable
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
	if app.agent.(*fakeAgent).autostart {
		t.Error("agent set up for invalid settings")
	}
}

func TestOutdatedAutostartIsReportedAndRepaired(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	fake.exe = "/old/place/kisetsu" // Kisetsu was moved since

	state, _ := app.GetRotation()
	if !state.Agent.Outdated {
		t.Fatal("outdated autostart entry not reported")
	}
	state, err := app.RepairAgent()
	if err != nil || state.Agent.Outdated {
		t.Errorf("after repair: %+v, %v", state.Agent, err)
	}
}

func TestStartAgent(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	fake.running = false // it stopped

	state, _ := app.GetRotation()
	if state.Agent.Running {
		t.Fatal("stopped agent reported as running")
	}
	if state, err := app.StartAgent(); err != nil || !state.Agent.Running {
		t.Errorf("after start: %+v, %v", state.Agent, err)
	}
}

// Kisetsu 0.1.0 rotated with a systemd timer; the agent replaces it.
func TestOldTimerIsReplaced(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	timer := &fakeTimer{installed: true}
	app.legacyTimer = timer
	rotation.SaveSettings(app.ctx, app.settings, rotation.Settings{Enabled: true, Interval: rotation.Hourly, Order: rotation.Random})

	app.retireTimer()
	if timer.installed || !fake.autostart || !fake.running {
		t.Errorf("timer installed %v, agent %+v", timer.installed, fake)
	}
}

func TestOldTimerIsKeptWhenTheAgentCannotRun(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.agent.(*fakeAgent).enableErr = autostart.ErrTemporaryExecutable
	timer := &fakeTimer{installed: true}
	app.legacyTimer = timer
	rotation.SaveSettings(app.ctx, app.settings, rotation.Settings{Enabled: true, Interval: rotation.Hourly, Order: rotation.Random})

	app.retireTimer()
	if !timer.installed {
		t.Error("timer removed although the agent could not be set up")
	}

	// With rotation off there is nothing to keep it for.
	rotation.SaveSettings(app.ctx, app.settings, rotation.Settings{Interval: rotation.Hourly, Order: rotation.Random})
	app.retireTimer()
	if timer.installed {
		t.Error("unused timer kept")
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

func TestNextWallpaperOnUnsupportedDesktop(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	downloadN(t, app, 1)
	app.rotation.Screen.(*onscreen.Service).Setter = &fakeWallpaper{err: fmt.Errorf("%w: %q", wallpaper.ErrUnsupportedDesktop, "cinnamon")}

	_, err := app.NextWallpaper()
	if pub := apperr.ToPublic(err); pub.Code != Unsupported || pub.Message != unsupportedDesktopMessage {
		t.Errorf("got %+v", pub)
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

func TestRotationFromSavedSearch(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	saved, err := app.SaveSearch("Lakes", searches.Query{ProviderIDs: []string{"stub"}})
	if err != nil {
		t.Fatal(err)
	}

	state, err := app.SaveRotation(RotationSettings{Interval: "1h", Order: "random", SavedSearchID: saved.ID})
	if err != nil {
		t.Fatal(err)
	}
	if state.Settings.SavedSearchID != saved.ID || state.SourceImages != 0 {
		t.Errorf("state = %+v", state)
	}

	_, err = app.SaveRotation(RotationSettings{Interval: "1h", Order: "random", SavedSearchID: saved.ID, CollectionID: 3})
	if apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("both sources: %v", err)
	}
}

func TestShowInTray(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	app.trayHost = func() bool { return true }

	// Tray on, rotation off: the agent runs for the icon alone.
	state, err := app.SetShowInTray(true)
	if err != nil || !state.ShowInTray || !state.TrayHost || !fake.autostart || !fake.running {
		t.Fatalf("tray on: state %+v, agent %+v, %v", state, fake, err)
	}

	// Rotation on and off again: the tray keeps the agent.
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	app.SaveRotation(RotationSettings{Enabled: false, Interval: "1h", Order: "random"})
	if !fake.autostart {
		t.Error("autostart entry removed although the tray icon is on")
	}

	// Showing or hiding the icon restarts the agent.
	starts, stops := fake.starts, fake.stops
	state, err = app.SetShowInTray(false)
	if err != nil || state.ShowInTray || fake.autostart || fake.running || fake.stops != stops+1 {
		t.Errorf("tray off: state %+v, agent %+v, %v", state, fake, err)
	}
	app.SaveRotation(RotationSettings{Enabled: true, Interval: "1h", Order: "random"})
	starts = fake.starts
	app.SetShowInTray(true)
	if !fake.running || fake.starts != starts+1 {
		t.Errorf("agent not restarted for the icon: %+v", fake)
	}
}

func TestShowInTrayLeavesTheSettingOffWhenTheAgentFails(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	app.agent.(*fakeAgent).enableErr = autostart.ErrForeign

	if _, err := app.SetShowInTray(true); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("err = %v", err)
	}
	if state, _ := app.GetRotation(); state.ShowInTray {
		t.Error("tray setting saved although the agent could not be set up")
	}
}
