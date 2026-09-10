//go:build !linux

package sandbox

import (
	"fmt"
	"os/exec"
)

func protectPlatformCommand(cmd *exec.Cmd, roots []string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("existing Desktop Notes are protected: unrestricted local execution is unavailable on this platform; use desktop_notes or an isolated virtual workspace")
}
