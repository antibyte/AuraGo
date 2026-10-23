// Package mediabins resolves and runs external media converters such as ffmpeg,
// ffprobe and ImageMagick.
//
// The package exists so that every AuraGo component that shells out to a media
// converter shares one discovery policy and one bounded command runner:
// an explicitly configured path must be an existing regular file, the Windows
// system "convert.exe" is never mistaken for ImageMagick, command output is
// capped, and a cancelled or expired context is reported as such.
package mediabins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LookPath is the executable lookup used by the package-level helpers. It is a
// variable so that callers and tests can substitute a deterministic lookup.
var LookPath = exec.LookPath

// DefaultOutputLimit is the number of bytes retained per stream when a caller
// does not pass an explicit limit.
const DefaultOutputLimit = 1 << 20

// ResolveWith returns an executable for the given candidates using lookPath.
//
// An explicitly configured path wins over the candidates. A configured path that
// looks like a filesystem path (absolute, or containing a separator) must exist
// and must be a regular file; a directory or a dangling symlink is rejected.
// A configured bare name is looked up on PATH.
func ResolveWith(lookPath func(string) (string, error), configuredPath string, candidates []string) (string, error) {
	if lookPath == nil {
		lookPath = LookPath
	}
	configured := strings.TrimSpace(configuredPath)
	if configured != "" {
		if filepath.IsAbs(configured) || strings.ContainsAny(configured, `/\`) {
			resolved, err := filepath.Abs(configured)
			if err != nil {
				return "", fmt.Errorf("resolve binary path %q: %w", configured, err)
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return "", fmt.Errorf("binary %q was not found", configured)
			}
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("binary %q is not a regular file", configured)
			}
			return resolved, nil
		}
		if found, err := lookPath(configured); err == nil {
			return found, nil
		}
		return "", fmt.Errorf("binary %q was not found", configured)
	}
	for _, candidate := range candidates {
		if found, err := lookPath(candidate); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("required binary was not found: %s", strings.Join(candidates, ", "))
}

// Resolve returns an executable for the given candidates.
func Resolve(configuredPath string, candidates []string) (string, error) {
	return ResolveWith(LookPath, configuredPath, candidates)
}

// ResolveImageMagickWith resolves ImageMagick using the supplied lookup.
//
// On Windows the bare name "convert" is never a candidate: C:\Windows\System32\convert.exe
// is the built-in FAT converter and would be reported as a working ImageMagick.
// An explicitly configured path is still honoured, so an operator can point at a
// real ImageMagick "convert.exe".
func ResolveImageMagickWith(lookPath func(string) (string, error), configuredPath string) (string, error) {
	if strings.TrimSpace(configuredPath) != "" {
		return ResolveWith(lookPath, configuredPath, nil)
	}
	candidates := []string{"magick"}
	if runtime.GOOS == "windows" {
		candidates = []string{"magick", "magick.exe"}
	} else {
		candidates = []string{"magick", "convert"}
	}
	return ResolveWith(lookPath, "", candidates)
}

// ResolveImageMagick resolves ImageMagick.
func ResolveImageMagick(configuredPath string) (string, error) {
	return ResolveImageMagickWith(LookPath, configuredPath)
}

// Run executes path with args and returns the captured stdout.
//
// At most limit bytes per stream are retained. The returned error carries the
// captured stderr so callers can surface the converter's own diagnostics. When
// the context was cancelled or its deadline expired, the error wraps
// context.Canceled or context.DeadlineExceeded and can be matched with errors.Is.
func Run(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = DefaultOutputLimit
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr cappedBuffer
	stdout.limit = limit
	stderr.limit = limit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				return stdout.Bytes(), ctxErr
			}
			return stdout.Bytes(), fmt.Errorf("%s: %w", message, ctxErr)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return stdout.Bytes(), err
		}
		return stdout.Bytes(), fmt.Errorf("%w: %s", err, message)
	}
	return stdout.Bytes(), nil
}

// RunDefault runs a converter with the default output limit.
func RunDefault(ctx context.Context, path string, args []string) ([]byte, error) {
	return Run(ctx, path, args, DefaultOutputLimit)
}

// IsContextError reports whether err was caused by a cancelled or expired context.
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// cappedBuffer keeps at most limit bytes and silently discards the rest.
type cappedBuffer struct {
	data  []byte
	limit int
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	if remaining := b.limit - len(b.data); remaining > 0 {
		if len(data) > remaining {
			b.data = append(b.data, data[:remaining]...)
		} else {
			b.data = append(b.data, data...)
		}
	}
	return len(data), nil
}

func (b *cappedBuffer) Bytes() []byte  { return b.data }
func (b *cappedBuffer) String() string { return string(b.data) }
