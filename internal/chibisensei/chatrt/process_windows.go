//go:build windows

package chatrt

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// startProcess starts the command inside a job object with KILL_ON_JOB_CLOSE, so Windows ends the server when
// AuraGo exits or crashes. It keeps the SysProcAttr that onnxrt.ConfigureWorkerCommand set (hidden window).
func startProcess(cmd *exec.Cmd) (*procHandle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, handle)
		_ = windows.CloseHandle(handle)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = windows.CloseHandle(job)
		return nil, err
	}
	var once sync.Once
	kill := func() error {
		var closeErr error
		once.Do(func() { closeErr = windows.CloseHandle(job) })
		return closeErr
	}
	h := &procHandle{done: make(chan struct{}), kill: kill, pid: cmd.Process.Pid}
	go func() { h.err = cmd.Wait(); _ = kill(); close(h.done) }()
	return h, nil
}
