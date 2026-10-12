//go:build !linux && !darwin && !windows

package main

import (
	"fmt"
	"os"
)

func preserveFileACL(source, target *os.File) error {
	return fmt.Errorf("ACL preservation is unsupported on this platform")
}
