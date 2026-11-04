// Package filelock takes exclusive, non-waiting locks on files, so that
// several Kisetsu processes (the window, the agent, commands) do not do
// the same work at once.
//
// It uses flock: the kernel releases a lock when the process ends, so a
// crash cannot leave a stale lock behind.
package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLocked means another process (or another lock in this one) holds the
// lock.
var ErrLocked = errors.New("file is locked")

// Lock takes an exclusive lock on path, creating the file if needed. It
// does not wait: a lock held elsewhere returns ErrLocked.
func Lock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("take lock: %w", err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
