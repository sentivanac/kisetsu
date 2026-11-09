//go:build !windows

package onscreen

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadTextFileDoesNotBlockOnAFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no FIFO:", err)
	}
	done := make(chan error, 1)
	go func() { _, err := readTextFile(fifo); done <- err }()
	select {
	case err := <-done:
		if msg := problemOf(t, err); msg != "Not a text file" {
			t.Errorf("FIFO: %q", msg)
		}
	case <-t.Context().Done():
		t.Fatal("blocked on a FIFO")
	}
}
