package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sentivanac/kisetsu/internal/autostart"
	"github.com/sentivanac/kisetsu/internal/filelock"
)

// AgentStatus is what the window knows about the background agent.
type AgentStatus struct {
	Running   bool
	Autostart autostart.Status
}

// AgentControl starts the agent and keeps its autostart entry;
// *agentControl in the app. Stopping needs no call: the agent exits by
// itself once rotation is off (ADR 0041).
type AgentControl interface {
	// Enable installs the autostart entry for this binary and starts the
	// agent if it is not running.
	Enable() error
	// Disable removes the autostart entry.
	Disable() error
	// Start starts the agent if it is not running.
	Start() error
	// Stop ends a running agent, for example to restart it with or
	// without the tray icon.
	Stop() error
	Status() (AgentStatus, error)
}

// agentControl runs `kisetsu agent` from the running binary.
type agentControl struct {
	entry    *autostart.Entry
	lockPath string // the agent holds it while it runs
	log      *slog.Logger
}

func (c *agentControl) Enable() error {
	if err := c.entry.Enable(); err != nil {
		return err
	}
	return c.Start()
}

func (c *agentControl) Disable() error {
	return c.entry.Disable()
}

// startWait is how long Start waits for a new agent to take its lock.
const startWait = 2 * time.Second

func (c *agentControl) Start() error {
	if c.running() {
		return nil
	}
	cmd := exec.Command(c.entry.Executable, "agent")
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}
	// Reap it if it exits while the window runs (for example when
	// rotation is turned off).
	go cmd.Wait()
	c.log.Info("agent started", "pid", cmd.Process.Pid)

	for deadline := time.Now().Add(startWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if c.running() {
			return nil
		}
	}
	return nil // it may still be starting; the status shows it either way
}

func (c *agentControl) Status() (AgentStatus, error) {
	st, err := c.entry.Status()
	return AgentStatus{Running: c.running(), Autostart: st}, err
}

// stopWait is how long Stop waits for the agent to end.
const stopWait = 3 * time.Second

func (c *agentControl) Stop() error {
	if !c.running() {
		return nil
	}
	// The agent writes its PID into the lock file; while the lock is held,
	// the PID belongs to the agent that holds it.
	data, err := os.ReadFile(c.lockPath)
	if err != nil {
		return fmt.Errorf("read agent PID: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 1 {
		return fmt.Errorf("invalid agent PID %q", strings.TrimSpace(string(data)))
	}
	if err := terminate(pid); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	for deadline := time.Now().Add(stopWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if !c.running() {
			c.log.Info("agent stopped", "pid", pid)
			return nil
		}
	}
	return fmt.Errorf("agent %d did not stop within %s", pid, stopWait)
}

// running reports whether an agent holds its lock.
func (c *agentControl) running() bool {
	release, err := filelock.Lock(c.lockPath)
	if errors.Is(err, filelock.ErrLocked) {
		return true
	}
	if err == nil {
		release()
	}
	return false
}
