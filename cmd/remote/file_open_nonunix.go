//go:build !unix

package main

import "os"

const regularFileReadFlags = os.O_RDONLY
