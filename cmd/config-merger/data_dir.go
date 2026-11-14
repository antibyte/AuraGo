package main

import (
	"fmt"
	"strings"

	"aurago/internal/config"
)

// Use the backend's actual loader, including config-relative path resolution.
// This query must not initialize a Vault or create any runtime state.
func resolvedDataDirectory(path string) (string, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return "", err
	}
	dir := cfg.Directories.DataDir
	if dir == "" || strings.ContainsAny(dir, "\x00\r\n") {
		return "", fmt.Errorf("configuration has no usable data directory")
	}
	return dir, nil
}
