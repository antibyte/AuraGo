//go:build !unix

package server

import "os"

// openRegularFileFlags is a plain read-only open: these platforms have no
// FIFOs inside a directory tree (Windows named pipes live under \\.\pipe\), so
// the open in openRegularFileInRoot cannot block on one.
const openRegularFileFlags = os.O_RDONLY
