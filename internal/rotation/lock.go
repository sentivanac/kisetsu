package rotation

import (
	"errors"

	"github.com/sentivanac/kisetsu/internal/filelock"
)

// ErrLocked means another rotation holds the lock.
var ErrLocked = errors.New("another wallpaper rotation is running")

// Lock takes the rotation lock at path, so the agent, the "Next
// wallpaper" button and a manual `kisetsu rotate` never change the
// wallpaper at the same time. It does not wait.
func Lock(path string) (unlock func(), err error) {
	unlock, err = filelock.Lock(path)
	if errors.Is(err, filelock.ErrLocked) {
		return nil, ErrLocked
	}
	return unlock, err
}
