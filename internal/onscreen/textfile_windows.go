//go:build windows

package onscreen

import "io/fs"

// Windows has no FIFOs to block on in the file system.
const openNonBlock = 0

// checkOwner accepts any file on Windows: who may write a file is set by
// its access list, which Kisetsu does not inspect. Files in the user's
// own folders are writable by the user only by default (ADR 0051).
func checkOwner(fs.FileInfo) error { return nil }
