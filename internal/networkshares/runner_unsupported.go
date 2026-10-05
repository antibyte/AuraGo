//go:build !linux && !windows

package networkshares

func platformCommand(_ Options, _ bool, _ string, _ []string, _ []byte) (string, []string, []byte, bool, error) {
	return "", nil, nil, false, codedError(ErrorUnavailable, "Local network share management is supported only on Linux and Windows.", nil)
}

func platformElevated() bool {
	return false
}

func elevationReason() string {
	return "Local network share management is supported only on Linux and Windows."
}
