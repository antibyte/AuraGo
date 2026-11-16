//go:build windows

package desktop

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameDesktopPathNoReplace(root *os.Root, src, dst string, expected *os.File) error {
	srcParent, err := root.OpenRoot(filepath.Dir(src))
	if err != nil {
		return err
	}
	defer srcParent.Close()
	dstParent, err := root.OpenRoot(filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer dstParent.Close()
	srcDir, err := srcParent.Open(".")
	if err != nil {
		return err
	}
	defer srcDir.Close()
	dstDir, err := dstParent.Open(".")
	if err != nil {
		return err
	}
	defer dstDir.Close()

	info, err := root.Lstat(src)
	if err != nil {
		return err
	}
	name, err := windows.NewNTUnicodeString(filepath.Base(src))
	if err != nil {
		return err
	}
	attributes := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT)
	if info.IsDir() {
		attributes |= windows.FILE_DIRECTORY_FILE
	} else {
		attributes |= windows.FILE_NON_DIRECTORY_FILE
	}
	oa := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(srcDir.Fd()),
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var iosb windows.IO_STATUS_BLOCK
	var sourceHandle windows.Handle
	err = windows.NtCreateFile(&sourceHandle,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		oa, &iosb, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, attributes, 0, 0)
	if err != nil {
		return err
	}
	source := os.NewFile(uintptr(sourceHandle), filepath.Base(src))
	defer source.Close()
	actualInfo, err := source.Stat()
	if err != nil {
		return err
	}
	var expectedInfo os.FileInfo
	if expected != nil {
		expectedInfo, err = expected.Stat()
		if err != nil || !os.SameFile(actualInfo, expectedInfo) {
			return ErrFileUndoUnavailable
		}
	} else if actualInfo.Mode()&os.ModeSymlink == 0 {
		return ErrFileUndoUnavailable
	}

	newName, err := windows.UTF16FromString(filepath.Base(dst))
	if err != nil {
		return err
	}
	nameBytes := len(newName)*2 - 2 // FILE_RENAME_INFORMATION excludes the trailing NUL.
	var layout fileRenameInformation
	buffer := make([]byte, int(unsafe.Offsetof(layout.FileName))+nameBytes)
	if len(buffer) < int(unsafe.Sizeof(layout)) {
		buffer = make([]byte, unsafe.Sizeof(layout))
	}
	renameInfo := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	renameInfo.ReplaceIfExists = 0
	renameInfo.RootDirectory = windows.Handle(dstDir.Fd())
	renameInfo.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&renameInfo.FileName[0], nameBytes/2), newName[:len(newName)-1])
	if err := windows.NtSetInformationFile(sourceHandle, &iosb, &buffer[0], uint32(len(buffer)), windows.FileRenameInformation); err != nil {
		if status, ok := err.(windows.NTStatus); ok && status == windows.STATUS_OBJECT_NAME_COLLISION {
			return fs.ErrExist
		}
		return fmt.Errorf("rename without replacement: %w", err)
	}
	if expected == nil {
		movedInfo, err := root.Lstat(dst)
		if err != nil || movedInfo.Mode()&os.ModeSymlink == 0 {
			return ErrFileUndoUncertain
		}
	} else {
		moved, err := root.Open(dst)
		if err != nil {
			return ErrFileUndoUncertain
		}
		movedInfo, statErr := moved.Stat()
		_ = moved.Close()
		if statErr != nil || !os.SameFile(movedInfo, expectedInfo) {
			return ErrFileUndoUncertain
		}
	}
	return nil
}
