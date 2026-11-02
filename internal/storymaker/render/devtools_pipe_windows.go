//go:build windows

package render

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// attachDevToolsPipe passes the two pipe ends to the browser as inherited handles and names them in
// --remote-debugging-io-pipes=<read>,<write>, which is how Chromium finds its DevTools pipe on Windows.
func attachDevToolsPipe(cmd *exec.Cmd, in, out *os.File) error {
	inHandle, outHandle := syscall.Handle(in.Fd()), syscall.Handle(out.Fd())
	for _, h := range []syscall.Handle{inHandle, outHandle} {
		if err := syscall.SetHandleInformation(h, syscall.HANDLE_FLAG_INHERIT, syscall.HANDLE_FLAG_INHERIT); err != nil {
			return err
		}
	}
	cmd.Args = append(cmd.Args, fmt.Sprintf("--remote-debugging-io-pipes=%d,%d", inHandle, outHandle))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags:              syscall.CREATE_NEW_PROCESS_GROUP,
		AdditionalInheritedHandles: []syscall.Handle{inHandle, outHandle},
	}
	return nil
}

// killProcessTree kills the browser and the processes it started.
func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
	_ = p.Kill()
}
