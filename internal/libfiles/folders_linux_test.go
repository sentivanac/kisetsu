package libfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openUnder lists this process's open files under dir (Linux: /proc).
func openUnder(t *testing.T, dir string) []string {
	t.Helper()
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skip("no /proc/self/fd:", err)
	}
	var out []string
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name()))
		if err == nil && (target == dir || strings.HasPrefix(target, dir+string(filepath.Separator))) {
			out = append(out, target)
		}
	}
	return out
}

// TestFolderIsNotKeptOpen: on Windows an open folder cannot be renamed or
// deleted, so after every operation nothing in it may stay open. Linux
// shows the open files, so the rule is checked here.
func TestFolderIsNotKeptOpen(t *testing.T) {
	_, folder, s, _ := withFolder(t)
	fget(t, s, "folders/5/Trips/sea.jpg")
	if _, err := s.OriginalPath("folders/5/Trips/sea.jpg"); err != nil {
		t.Fatal(err)
	}
	s.Remove("folders/5/Trips/sea.jpg") // refused
	if open := openUnder(t, folder); len(open) != 0 {
		t.Errorf("still open in the folder: %q", open)
	}
}
