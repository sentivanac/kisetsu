//go:build !windows

package onscreen

import (
	"io/fs"
	"os"
	"syscall"
)

// openNonBlock keeps opening a FIFO from blocking; reading a regular file
// is not affected.
const openNonBlock = syscall.O_NONBLOCK

// checkOwner accepts only a file the user owns that only the user can
// write: anyone else able to write it could put text on this user's
// desktop.
func checkOwner(fi fs.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return problem("The owner of the text file is unknown")
	}
	if int(st.Uid) != os.Getuid() {
		return problem("The text file belongs to another user")
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return problem("Others can change the text file")
	}
	return nil
}
