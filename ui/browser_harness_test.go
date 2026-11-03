package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBrowserExecutable(t *testing.T) {
	dir := t.TempDir()
	name := "google-chrome"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(bin, []byte("browser discovery fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CHROME_BIN", "")
	if got, ok := browserExecutable(); !ok || got != bin {
		t.Fatalf("PATH discovery = %q, %v; want %q", got, ok, bin)
	}
	t.Setenv("CHROME_BIN", " "+bin+" ")
	if got, ok := browserExecutable(); !ok || got != bin {
		t.Fatalf("explicit discovery = %q, %v; want %q", got, ok, bin)
	}
	for _, invalid := range []string{dir, filepath.Join(dir, "missing-browser")} {
		t.Setenv("CHROME_BIN", invalid)
		if got, ok := browserExecutable(); ok {
			t.Fatalf("invalid explicit browser %q silently fell back to %q", invalid, got)
		}
	}
}

func TestBrowserSmokeAvailability(t *testing.T) {
	if os.Getenv("AURAGO_BROWSER_HELPER_PROCESS") == "1" {
		newSmokeBrowser(t)
		t.Fatal("invalid browser unexpectedly started")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, flag, bin, diagnostic string
		fail                        bool
	}{
		{"optional_missing", "", filepath.Join(t.TempDir(), "missing"), "--- SKIP:", false},
		{"required_missing", "1", filepath.Join(t.TempDir(), "missing"), "requires Chrome or Edge", true},
		{"required_launch_failure", "1", self, "browser launch failed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AURAGO_BROWSER_HELPER_PROCESS", "1")
			t.Setenv("AURAGO_RUN_BROWSER_SMOKE", tc.flag)
			t.Setenv("CHROME_BIN", tc.bin)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, self, "-test.run=^TestBrowserSmokeAvailability$", "-test.v").CombinedOutput()
			if ctx.Err() != nil || (err != nil) != tc.fail || !strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("browser availability: err=%v, context=%v, output=%s", err, ctx.Err(), output)
			}
		})
	}
}
