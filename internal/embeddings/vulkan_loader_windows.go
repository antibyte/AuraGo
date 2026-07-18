//go:build windows

package embeddings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func vulkanLoaderEvidence() string {
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		systemRoot = strings.TrimSpace(os.Getenv("WINDIR"))
	}
	if systemRoot == "" {
		return ""
	}
	loader := filepath.Join(systemRoot, "System32", "vulkan-1.dll")
	info, err := os.Stat(loader)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return fmt.Sprintf("%s|size=%d|mtime=%d", filepath.Clean(loader), info.Size(), info.ModTime().Unix())
}
