package rotation

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLockReportsErrLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rotate.lock")
	unlock, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := Lock(path); !errors.Is(err, ErrLocked) {
		t.Errorf("second lock: err = %v, want ErrLocked", err)
	}
}
