//go:build unix

package server

import (
	"os"
	"syscall"
)

// openRegularFileFlags opens the last component non-blocking, so a FIFO
// swapped in between openRegularFileInRoot's Lstat check and its open cannot
// block the open; the post-open Stat then refuses it. Go never registers
// regular files with the poller, so their reads and sendfile are unaffected.
const openRegularFileFlags = os.O_RDONLY | syscall.O_NONBLOCK
