//go:build linux

package sandbox

// canonicalizeUnixSocketGrantPath uses the Landlock path rule: absolute, then
// symlink-resolved. A missing path keeps the cleaned absolute form so a
// required open still fails closed. A path that cannot be stored in a
// colon-separated list is rejected.
func canonicalizeUnixSocketGrantPath(path string) (string, bool) {
	resolved, err := canonicalSandboxPath(path)
	if err != nil {
		return "", false
	}
	return resolved, true
}
