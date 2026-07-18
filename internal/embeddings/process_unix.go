//go:build !windows

package embeddings

import "os/exec"

func configureHiddenProcess(_ *exec.Cmd) {}
