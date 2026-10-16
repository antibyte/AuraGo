package bookshelfconvert

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Version is an explicit conversion-job probe; capability and read endpoints
// must never call it. The policy version invalidates derived-file reuse as well.
func Version(ctx context.Context, executable string) (string, error) {
	if ok, _ := Capability(executable); !ok {
		return "", ErrUnavailable
	}
	executable, err := trustedExecutable(executable)
	if err != nil {
		return "", err
	}
	work, err := os.MkdirTemp("", "bookshelf-version-")
	if err != nil {
		return "", fmt.Errorf("stage converter version: %w", err)
	}
	defer os.RemoveAll(work)
	input := filepath.Join(work, "empty")
	if err = os.WriteFile(input, nil, 0600); err != nil {
		return "", err
	}
	args := sandboxArgs(executable, input, work, ".txt")
	args = append(args[:len(args)-2], "--version")
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return "", ErrUnavailable
	}
	args[7] = bwrap
	prlimit, err := exec.LookPath("prlimit")
	if err != nil {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, prlimit, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	cmd.Dir = work
	var output versionOutput
	cmd.Stdout = &output
	cmd.WaitDelay = time.Second
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("probe isolated converter version: %w", err)
	}
	version := strings.Join(strings.Fields(output.String()), " ")
	if version == "" || len(version) > 256 || strings.IndexFunc(version, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("converter returned an invalid version")
	}
	return "bookshelf-sandbox-v1;" + version, nil
}

type versionOutput struct{ bytes.Buffer }

func (w *versionOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 4096 {
		return 0, fmt.Errorf("converter version output exceeds limit")
	}
	return w.Buffer.Write(p)
}
