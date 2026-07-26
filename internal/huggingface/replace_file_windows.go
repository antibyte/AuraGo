//go:build windows

package huggingface

import "golang.org/x/sys/windows"

func replaceDownloadedFile(source, destination string) error {
	return windows.Rename(source, destination)
}
