// Package bookshelfconvert creates derived EPUB files in a network-isolated job.
package bookshelfconvert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxBytes int64 = 256 << 20

var jobs = make(chan struct{}, 1)
var ErrUnavailable = errors.New("bookshelf: safe conversion unavailable")

// Capability performs no process execution or installation. A configured runtime
// can still be refused by kernel namespace policy when a job actually starts.
func Capability(executable string) (bool, string) {
	if executable == "" {
		return false, "not_configured"
	}
	if runtime.GOOS != "linux" {
		return false, "linux_isolation_required"
	}
	if _, err := trustedExecutable(executable); err != nil {
		return false, "invalid_converter"
	}
	for _, name := range []string{"bwrap", "prlimit"} {
		if _, err := exec.LookPath(name); err != nil {
			return false, "isolation_tools_missing"
		}
	}
	return true, ""
}

func trustedExecutable(name string) (string, error) {
	if !filepath.IsAbs(name) {
		return "", ErrUnavailable
	}
	path, err := filepath.EvalSymlinks(name)
	if err != nil {
		return "", ErrUnavailable
	}
	// Only standard system runtimes are exposed, never a home or AuraGo directory.
	if !strings.HasPrefix(path, "/usr/") && !strings.HasPrefix(path, "/opt/calibre/") {
		return "", ErrUnavailable
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", ErrUnavailable
	}
	return path, nil
}

func sandboxArgs(executable, input, output, extension string) []string {
	args := []string{"--as=1073741824", "--cpu=120", "--nproc=64", "--nofile=128", "--fsize=268435456", "--core=0", "--", "bwrap",
		"--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--clearenv"}
	for _, path := range []string{"/usr", "/bin", "/lib", "/lib64", "/opt/calibre"} {
		args = append(args, "--ro-bind-try", path, path)
	}
	args = append(args, "--ro-bind-try", "/etc/fonts", "/etc/fonts", "--ro-bind-try", "/etc/ld.so.cache", "/etc/ld.so.cache",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", "/home", "--dir", "/input",
		"--ro-bind", input, "/input/book"+extension, "--bind", output, "/output",
		"--setenv", "HOME", "/tmp", "--setenv", "TMPDIR", "/tmp", "--setenv", "LANG", "C.UTF-8",
		"--setenv", "PATH", "/usr/bin:/bin", "--setenv", "QT_QPA_PLATFORM", "offscreen",
		"--setenv", "CALIBRE_CONFIG_DIRECTORY", "/tmp/calibre", "--chdir", "/tmp",
		"--", executable, "/input/book"+extension, "/output/book.epub")
	return args
}

// Convert accepts only a prepared original and a new managed destination. It
// never publishes partial output or falls back to unconfined process execution.
func Convert(ctx context.Context, executable, inputPath, outputPath string) error {
	if ok, _ := Capability(executable); !ok {
		return ErrUnavailable
	}
	executable, err := trustedExecutable(executable)
	if err != nil {
		return err
	}
	extension := strings.ToLower(filepath.Ext(inputPath))
	switch extension {
	case ".cbr", ".rtf", ".docx", ".odt", ".lit", ".pdb", ".lrf":
	default:
		return fmt.Errorf("convert: unsupported input format")
	}
	if strings.ToLower(filepath.Ext(outputPath)) != ".epub" {
		return fmt.Errorf("convert: output must be EPUB")
	}
	if _, err := os.Lstat(outputPath); !os.IsNotExist(err) {
		return fmt.Errorf("convert: destination already exists")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	select {
	case jobs <- struct{}{}:
		defer func() { <-jobs }()
	case <-ctx.Done():
		return ctx.Err()
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open conversion input: %w", err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBytes {
		return fmt.Errorf("convert: invalid input size")
	}
	work, err := os.MkdirTemp(filepath.Dir(outputPath), ".bookshelf-convert-")
	if err != nil {
		return fmt.Errorf("stage conversion: %w", err)
	}
	defer os.RemoveAll(work)
	staged := filepath.Join(work, "input"+extension)
	copy, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(copy, io.LimitReader(input, maxBytes+1))
	closeErr := copy.Close()
	if copyErr != nil || closeErr != nil || n != info.Size() {
		return fmt.Errorf("convert: incomplete input copy")
	}
	outDir := filepath.Join(work, "output")
	if err := os.Mkdir(outDir, 0700); err != nil {
		return err
	}
	prlimit, err := exec.LookPath("prlimit")
	if err != nil {
		return ErrUnavailable
	}
	args := sandboxArgs(executable, staged, outDir, extension)
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return ErrUnavailable
	}
	args[7] = bwrap
	cmd := exec.CommandContext(ctx, prlimit, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	cmd.Dir = work
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("convert: isolated converter failed")
	}
	root, err := os.OpenRoot(outDir)
	if err != nil {
		return err
	}
	defer root.Close()
	output, err := root.Open("book.epub")
	if err != nil {
		return fmt.Errorf("convert: missing output")
	}
	defer output.Close()
	result, err := output.Stat()
	if err != nil || !result.Mode().IsRegular() || result.Size() <= 0 || result.Size() > maxBytes {
		return fmt.Errorf("convert: invalid output size")
	}
	target, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		target.Close()
		if !complete {
			os.Remove(outputPath)
		}
	}()
	n, err = io.Copy(target, io.LimitReader(output, maxBytes+1))
	if err != nil || n != result.Size() {
		return fmt.Errorf("convert: incomplete output")
	}
	if err = target.Sync(); err != nil {
		return err
	}
	if err = target.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
