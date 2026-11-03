package desktop

import (
	"io/fs"
	"syscall"
)

func archiveSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func regularFileIdentity(path string, _ fs.FileInfo) (fileID, uint64, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fileID{}, 0, err
	}
	handle, err := syscall.CreateFile(pointer, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fileID{}, 0, err
	}
	defer syscall.CloseHandle(handle)
	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &data); err != nil {
		return fileID{}, 0, err
	}
	return fileID{
		dev: uint64(data.VolumeSerialNumber),
		ino: uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow),
	}, uint64(data.NumberOfLinks), nil
}
