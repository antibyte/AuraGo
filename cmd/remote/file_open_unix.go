//go:build unix

package main

import (
	"os"
	"syscall"
)

const regularFileReadFlags = os.O_RDONLY | syscall.O_NONBLOCK
