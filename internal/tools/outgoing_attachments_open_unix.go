//go:build unix

package tools

import "syscall"

// outgoingOpenNonblock is added to the flags that open an outgoing attachment, so that a
// FIFO swapped in for the file cannot hang the open until a writer appears.
const outgoingOpenNonblock = syscall.O_NONBLOCK
