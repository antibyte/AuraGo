//go:build !windows

package huggingface

import "os"

func replaceDownloadedFile(source, destination string) error {
	return os.Rename(source, destination)
}
