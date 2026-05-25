//go:build !windows

package desktop

import (
	"os"
	"time"
)

func getCreationTime(info os.FileInfo) time.Time {
	return info.ModTime()
}
