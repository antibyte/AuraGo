//go:build !linux

package sandbox

// canonicalizeUnixSocketGrantPath keeps the caller path unchanged.
// Windows filepath.Abs would rewrite a POSIX socket such as
// /var/run/docker.sock; grant tests on Windows expect that raw segment.
func canonicalizeUnixSocketGrantPath(path string) (string, bool) {
	return path, true
}
