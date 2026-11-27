//go:build !windows

package chatrt

import "os/exec"

// startProcess starts the command and returns its handle. There is no parent-death hook here: systemd and Docker
// end the child with AuraGo, and a hand-started AuraGo that is killed hard leaves it behind (see the plan's risks).
func startProcess(cmd *exec.Cmd) (*procHandle, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	h := &procHandle{done: make(chan struct{}), kill: cmd.Process.Kill, pid: cmd.Process.Pid}
	go func() { h.err = cmd.Wait(); close(h.done) }()
	return h, nil
}
