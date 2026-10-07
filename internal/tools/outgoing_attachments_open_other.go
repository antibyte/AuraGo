//go:build !unix

package tools

// outgoingOpenNonblock has no counterpart here: opening a FIFO is a Unix concern.
const outgoingOpenNonblock = 0
