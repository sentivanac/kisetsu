package main

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/filelock"
)

// A stand-in "kisetsu" that holds the agent lock like the real agent and
// writes its PID, so the test can stop it.
func TestAgentControlStartsAndDetectsTheAgent(t *testing.T) {
	flock, err := exec.LookPath("flock")
	if err != nil {
		t.Skip("flock(1) not installed")
	}
	dir := t.TempDir()
	lock := filepath.Join(dir, "agent.lock")
	pidFile := filepath.Join(dir, "pid")
	script := filepath.Join(dir, "kisetsu")
	// Like the agent, the holder of the lock writes its PID into it.
	os.WriteFile(script, []byte("#!/bin/sh\n[ \"$1\" = agent ] || exit 2\necho $$ > "+pidFile+
		"\nexec "+flock+" -n "+lock+` sh -c 'echo $$ > "$0"; exec sleep 30' `+lock+"\n"), 0o755)

	c := &agentControl{
		entry:    &autostart.Entry{Dir: filepath.Join(dir, "autostart"), Executable: script, TempDir: "/nonexistent"},
		lockPath: lock,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if st, _ := c.Status(); st.Running {
		t.Fatal("running before start")
	}
	if err := c.Enable(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				syscall.Kill(-pid, syscall.SIGTERM) // its own session and process group
			}
		}
	})

	st, err := c.Status()
	if err != nil || !st.Running || !st.Autostart.Installed || st.Autostart.Executable != script {
		t.Errorf("status = %+v, %v", st, err)
	}
	// The check must not take the lock away from the running agent.
	if _, err := filelock.Lock(lock); err == nil {
		t.Error("lock is free while the agent runs")
	}
	if err := c.Start(); err != nil {
		t.Errorf("second start: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.Status(); st.Running {
		t.Error("still running after Stop")
	}
	if err := c.Stop(); err != nil {
		t.Errorf("stop when not running: %v", err)
	}
	if err := c.Disable(); err != nil {
		t.Fatal(err)
	}
	if st, _ := c.Status(); st.Autostart.Installed {
		t.Error("autostart entry still installed")
	}
}
