// Package filelock takes exclusive, non-waiting locks on files, so that
// several Kisetsu processes (the window, the agent, commands) do not do
// the same work at once.
//
// The operating system releases a lock when the process ends, so a crash
// cannot leave a stale lock behind: flock on Unix, LockFileEx on Windows.
package filelock

import (
	"errors"
	"fmt"
	"os"
)

// ErrLocked means another process (or another lock in this one) holds the
// lock.
var ErrLocked = errors.New("file is locked")

// Lock takes an exclusive lock on path, creating the file if needed. It
// does not wait: a lock held elsewhere returns ErrLocked. The file's
// content stays readable and writable (the agent keeps its PID in it).
func Lock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := lockFile(f); err != nil {
		f.Close()
		if errors.Is(err, ErrLocked) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("take lock: %w", err)
	}
	return func() {
		unlockFile(f)
		f.Close()
	}, nil
}
