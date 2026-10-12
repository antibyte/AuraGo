//go:build !windows

package decisions

import "os/exec"

func configureHiddenProcess(_ *exec.Cmd) {}
