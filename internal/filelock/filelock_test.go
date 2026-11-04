package filelock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	unlock, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(path); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock: err = %v, want ErrLocked", err)
	}

	unlock()
	again, err := Lock(path)
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	again()
}

// A process that holds the lock and dies must not leave it behind.
func TestLockIsReleasedWhenTheProcessEnds(t *testing.T) {
	if os.Getenv("KISETSU_LOCK_HELPER") != "" {
		if _, err := Lock(os.Getenv("KISETSU_LOCK_HELPER")); err != nil {
			os.Exit(3)
		}
		os.Exit(0) // exits without unlocking
	}
	path := filepath.Join(t.TempDir(), "test.lock")

	cmd := exec.Command(os.Args[0], "-test.run=TestLockIsReleasedWhenTheProcessEnds")
	cmd.Env = append(os.Environ(), "KISETSU_LOCK_HELPER="+path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("helper process: %v", err)
	}

	unlock, err := Lock(path)
	if err != nil {
		t.Fatalf("lock left behind by a finished process: %v", err)
	}
	unlock()
}
