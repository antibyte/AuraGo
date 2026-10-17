//go:build windows

package decisions

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Match the managed-runtime job policy: closing the job also reaps descendants.
func runDiscoveryCommand(command *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	command.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Assign the job before any child code can spawn an unowned descendant.
	command.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if err = command.Start(); err != nil {
		return err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, handle)
		windows.CloseHandle(handle)
	}
	if err == nil {
		err = resumeDiscoveryProcess(uint32(command.Process.Pid))
	}
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return err
	}
	return command.Wait()
}

func resumeDiscoveryProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("snapshot discovery thread: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if openErr != nil {
			return fmt.Errorf("open discovery thread: %w", openErr)
		}
		count, resumeErr := windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if resumeErr != nil {
			return fmt.Errorf("resume discovery thread: %w", resumeErr)
		}
		if count == 1 {
			return nil
		}
		if count > 1 {
			return fmt.Errorf("discovery thread remains suspended")
		}
	}
	return fmt.Errorf("find suspended discovery thread: %w", err)
}
