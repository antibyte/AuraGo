package sandbox

import (
	"os"
	"strconv"
	"strings"
)

// filterEnv removes AURAGO_SBX_* and other sensitive env vars from the environment
// passed to the sandboxed process.
func filterEnv(env []string) []string {
	var filtered []string
	for _, e := range env {
		if strings.HasPrefix(e, "AURAGO_SBX_") {
			continue
		}
		if strings.HasPrefix(e, "AURAGO_MASTER_KEY") {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// splitPaths splits a colon-separated path list, trimming whitespace and skipping empty segments.
func splitPaths(s string) []string {
	if s == "" {
		return nil
	}
	var paths []string
	for _, p := range strings.Split(s, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// envInt reads an environment variable and parses it as an integer. Returns 0 on error or if empty.
func envInt(key string) int {
	s := os.Getenv(key)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}
