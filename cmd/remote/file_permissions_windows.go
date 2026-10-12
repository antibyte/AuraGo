package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

var reopenFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

func preserveFilePermissions(source, target *os.File) error {
	sourceHandle, err := reopenPermissionHandle(source, windows.READ_CONTROL)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(sourceHandle)
	sd, err := windows.GetSecurityInfo(sourceHandle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read existing ACL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	handle, err := reopenPermissionHandle(target, windows.WRITE_DAC|windows.READ_CONTROL)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func reopenPermissionHandle(file *os.File, access uint32) (windows.Handle, error) {
	// Reopen the pinned file object, never its potentially replaced path.
	handle, _, callErr := reopenFile.Call(file.Fd(), uintptr(access), windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, 0)
	if windows.Handle(handle) == windows.InvalidHandle {
		return 0, fmt.Errorf("reopen file for ACL preservation: %w", callErr)
	}
	return windows.Handle(handle), nil
}
