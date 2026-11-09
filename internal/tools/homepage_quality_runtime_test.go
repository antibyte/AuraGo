package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHomepageQualityResultUsesExitStatus(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"status":"ok","exit_code":0}`, true},
		{`{"status":"ok","exit_code":1}`, false},
		{`{"status":"ok"}`, false},
		{`{"status":"error","message":"not installed"}`, false},
		{`unreadable`, false},
	} {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(homepageQualityResult(tc.raw)), &result); err != nil {
			t.Fatal(err)
		}
		if (result["status"] == "ok") != tc.valid {
			t.Fatalf("invalid quality verdict for %s: %v", tc.raw, result)
		}
	}
}

func TestHomepageLintCommandPreservesFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command execution is covered by Linux and the container smoke")
	}
	for _, tc := range []struct {
		name       string
		lintScript string
		tscScript  string
		wantExit   int
	}{
		{"missing runtime", "", "", 127},
		{"missing config", "exit 2", "", 2},
		{"lint error", "exit 1", "", 1},
		{"typescript error", "echo should-not-run > lint-ran", "exit 2", 2},
		{"clean project", "exit 0", "exit 0", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "node_modules", ".bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			for name, script := range map[string]string{"eslint": tc.lintScript, "tsc": tc.tscScript} {
				if script != "" {
					if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.tscScript != "" {
				if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("/bin/sh", "-c", homepageLintCommand(dir))
			cmd.Env = []string{"PATH=" + bin}
			out, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if cmd.ProcessState == nil {
					t.Fatal(err)
				}
				exit = cmd.ProcessState.ExitCode()
			}
			if exit != tc.wantExit {
				t.Fatalf("exit=%d want=%d: %s", exit, tc.wantExit, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "lint-ran")); err == nil {
				t.Fatal("ESLint ran after TypeScript failed")
			}
		})
	}
}

func TestHomepageBrowserChecksUsePinnedRuntime(t *testing.T) {
	old := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = old })
	var commands []string
	homepageDockerExecFunc = func(_ DockerConfig, _, command, _ string) string {
		commands = append(commands, command)
		return `{"status":"ok","exit_code":0}`
	}
	HomepageScreenshot(context.Background(), HomepageConfig{}, "https://example.com", "", nil)
	HomepageCheckJS(context.Background(), HomepageConfig{}, "https://example.com", nil)
	for _, command := range commands {
		if !strings.Contains(command, homepagePlaywrightModule) || strings.Contains(command, "npx ") {
			t.Fatalf("browser check must use the installed pinned runtime: %s", command)
		}
	}
	if !strings.Contains(homepageDockerfile, "playwright@1.61.1") || !strings.Contains(homepageDockerfile, "playwright:v1.61.1-noble") {
		t.Fatal("Playwright package and browser image must match")
	}
}
