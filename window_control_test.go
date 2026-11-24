package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/agent"
	"github.com/sentivanac/kisetsu/internal/filelock"
)

func TestWindowOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window.lock")
	if windowOpen(path) {
		t.Error("open without a window")
	}
	// Nothing is started when no window is open.
	if err := closeWindow(context.Background(), path); err != nil {
		t.Error(err)
	}
	release, err := filelock.Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	if !windowOpen(path) {
		t.Error("not open although the lock is held")
	}
	release()
	if windowOpen(path) {
		t.Error("still open after the lock was released")
	}
}

func TestSecondInstanceAction(t *testing.T) {
	for _, c := range []struct {
		args           []string
		closing, about bool
	}{{nil, false, false}, {[]string{aboutArg}, false, true}, {[]string{closeArg}, true, false}} {
		if closing, about := secondInstanceAction(c.args); closing != c.closing || about != c.about {
			t.Errorf("%q: %v, %v", c.args, closing, about)
		}
	}
}

func TestEnsureAgent(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	fake := app.agent.(*fakeAgent)
	app.ensureAgent()
	if fake.starts != 0 {
		t.Error("started with nothing to do")
	}
	if err := agent.SavePrefs(app.ctx, app.settings, agent.Prefs{ShowInTray: true}); err != nil {
		t.Fatal(err)
	}
	app.ensureAgent()
	app.ensureAgent()
	if fake.starts != 1 || fake.autostart {
		t.Errorf("agent = %+v, want one start and no autostart entry", fake)
	}
	// A closing window never brings the agent back.
	fake.running = false
	app.closing.Store(true)
	app.ensureAgent()
	if fake.running {
		t.Error("started by a closing window")
	}
}
