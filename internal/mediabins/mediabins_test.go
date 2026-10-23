package mediabins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveWithRejectsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	if _, err := ResolveWith(lookPath, dir, nil); err == nil {
		t.Fatal("expected a directory to be rejected as a binary")
	} else if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("error = %q, want a regular-file error", err)
	}
}

func TestResolveWithRejectsMissingConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	missing := filepath.Join(dir, "ffmpeg")
	if _, err := ResolveWith(lookPath, missing, nil); err == nil {
		t.Fatal("expected a missing configured path to be rejected")
	} else if !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("error = %q, want a not-found error", err)
	}
}

func TestResolveWithAcceptsRegularFile(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(binary, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	resolved, err := ResolveWith(lookPath, binary, nil)
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if resolved != binary {
		t.Fatalf("resolved = %q, want %q", resolved, binary)
	}
}

func TestResolveWithUsesCandidates(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "ffmpeg" {
			return "/usr/bin/ffmpeg", nil
		}
		return "", os.ErrNotExist
	}

	resolved, err := ResolveWith(lookPath, "", []string{"ffmpeg", "avconv"})
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if resolved != "/usr/bin/ffmpeg" {
		t.Fatalf("resolved = %q", resolved)
	}
}

func TestResolveWithReportsMissingCandidates(t *testing.T) {
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	_, err := ResolveWith(lookPath, "", []string{"ffmpeg", "avconv"})
	if err == nil {
		t.Fatal("expected a not-found error")
	}
	if !strings.Contains(err.Error(), "ffmpeg, avconv") {
		t.Fatalf("error = %q, want the candidate list", err)
	}
}

func TestResolveImageMagickSkipsWindowsSystemConvert(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the convert.exe trap only exists on Windows")
	}
	dir := t.TempDir()
	convert := filepath.Join(dir, "convert.exe")
	if err := os.WriteFile(convert, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	lookPath := func(name string) (string, error) {
		if name == "convert" || name == "convert.exe" {
			return convert, nil
		}
		return "", os.ErrNotExist
	}

	_, err := ResolveImageMagickWith(lookPath, "")
	if err == nil {
		t.Fatal("expected the Windows system convert.exe not to be selected")
	}
	if !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("error = %q, want a not-found error", err)
	}
}

func TestResolveImageMagickHonoursConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	convert := filepath.Join(dir, "convert.exe")
	if err := os.WriteFile(convert, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	lookPath := func(string) (string, error) { return "", os.ErrNotExist }

	resolved, err := ResolveImageMagickWith(lookPath, convert)
	if err != nil {
		t.Fatalf("ResolveImageMagickWith: %v", err)
	}
	if resolved != convert {
		t.Fatalf("resolved = %q, want %q", resolved, convert)
	}
}

func TestRunReturnsStdout(t *testing.T) {
	t.Setenv("MEDIABINS_HELPER", "1")
	t.Setenv("MEDIABINS_HELPER_MODE", "stdout")

	output, err := Run(context.Background(), os.Args[0], []string{"-test.run=TestRunHelperProcess"}, 1<<20)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(output), "hello from helper") {
		t.Fatalf("output = %q", output)
	}
}

func TestRunReportsStderrOnFailure(t *testing.T) {
	t.Setenv("MEDIABINS_HELPER", "1")
	t.Setenv("MEDIABINS_HELPER_MODE", "fail")

	_, err := Run(context.Background(), os.Args[0], []string{"-test.run=TestRunHelperProcess"}, 1<<20)
	if err == nil {
		t.Fatal("expected a failure")
	}
	if !strings.Contains(err.Error(), "converter says no") {
		t.Fatalf("error = %q, want the stderr text", err)
	}
}

func TestRunReportsDeadlineExceeded(t *testing.T) {
	t.Setenv("MEDIABINS_HELPER", "1")
	t.Setenv("MEDIABINS_HELPER_MODE", "sleep")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := Run(ctx, os.Args[0], []string{"-test.run=TestRunHelperProcess"}, 1<<20)
	if err == nil {
		t.Fatal("expected a deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
	if !IsContextError(err) {
		t.Fatalf("error = %v, want IsContextError to match", err)
	}
}

func TestRunBoundsOutput(t *testing.T) {
	t.Setenv("MEDIABINS_HELPER", "1")
	t.Setenv("MEDIABINS_HELPER_MODE", "noisy")

	output, err := Run(context.Background(), os.Args[0], []string{"-test.run=TestRunHelperProcess"}, 16)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(output) > 16 {
		t.Fatalf("output length = %d, want at most 16 bytes", len(output))
	}
}

// TestRunHelperProcess is the child process used by the Run tests above.
func TestRunHelperProcess(t *testing.T) {
	if os.Getenv("MEDIABINS_HELPER") != "1" {
		return
	}
	switch os.Getenv("MEDIABINS_HELPER_MODE") {
	case "stdout":
		os.Stdout.WriteString("hello from helper")
	case "fail":
		os.Stderr.WriteString("converter says no")
		os.Exit(3)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "noisy":
		os.Stdout.WriteString(strings.Repeat("x", 4096))
	}
	os.Exit(0)
}
