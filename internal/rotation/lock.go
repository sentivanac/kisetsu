package rotation

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLocked means another rotation holds the lock.
var ErrLocked = errors.New("another wallpaper rotation is running")

// Lock takes an exclusive lock on path, so the timer, the "Next wallpaper"
// button and a manual `kisetsu rotate` never change the wallpaper at the
// same time. It uses flock: the kernel releases the lock when the process
// ends, so a crash cannot leave a stale lock behind. It does not wait.
func Lock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open rotation lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("take rotation lock: %w", err)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
