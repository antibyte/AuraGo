package tools

import (
	"errors"
	"os"
	"path/filepath"
)

// Keep custom build contexts working while repository Dockerfiles live in deploy/docker.
func sidecarDockerfilePath(contextDir, name string) string {
	legacy := filepath.Join(contextDir, name)
	if _, err := os.Stat(legacy); errors.Is(err, os.ErrNotExist) {
		bundled := filepath.Join(contextDir, "deploy", "docker", name)
		if _, err := os.Stat(bundled); !errors.Is(err, os.ErrNotExist) {
			return bundled
		}
	}
	return legacy
}
