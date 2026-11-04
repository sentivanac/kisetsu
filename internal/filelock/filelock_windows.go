//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Windows locks are mandatory: a locked byte cannot be read or written
// through another handle. One byte far past the end of the file is locked
// instead, so the file's content stays usable.
const lockOffsetHigh = 0x7fffffff

func lockFile(f *os.File) error {
	ol := windows.Overlapped{OffsetHigh: lockOffsetHigh}
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrLocked
	}
	return err
}

func unlockFile(f *os.File) {
	ol := windows.Overlapped{OffsetHigh: lockOffsetHigh}
	windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
