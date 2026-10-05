//go:build linux

package networkshares

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"aurago/internal/sudoticket"
)

const runnerSudoPassword = "vault-sudo-secret"

func sudoRunnerOptions() Options {
	return Options{SudoEnabled: true, SudoUnrestricted: true, SudoPassword: runnerSudoPassword}
}

func skipWhenRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root runs privileged commands without sudo")
	}
}

func TestRunnerSudoCommandKeepsPasswordOffStdin(t *testing.T) {
	skipWhenRoot(t)
	for _, payload := range [][]byte{nil, []byte("[share]\npath = /srv/share\n")} {
		cmd, err := newRunnerCommand(context.Background(), sudoRunnerOptions(), true, "net", []string{"conf", "addshare", "--", "share", "/srv/share"}, payload)
		if err != nil {
			t.Fatalf("newRunnerCommand: %v", err)
		}
		wantArgs := []string{"sudo", "-n", "--", "net", "conf", "addshare", "--", "share", "/srv/share"}
		if !reflect.DeepEqual(cmd.Args, wantArgs) {
			t.Fatalf("sudo args = %q, want %q (non-interactive, no -S)", cmd.Args, wantArgs)
		}
		var stdin []byte
		if cmd.Stdin != nil {
			if stdin, err = io.ReadAll(cmd.Stdin); err != nil {
				t.Fatalf("read stdin: %v", err)
			}
		}
		if strings.Contains(string(stdin), runnerSudoPassword) || string(stdin) != string(payload) {
			t.Fatalf("sudo stdin = %q, want only the command input %q", stdin, payload)
		}
	}
}

// fakeRunnerSudo records the sudo processes behind the shared ticket and puts
// a recording sudo stand-in first on PATH, so Run never executes real sudo.
type fakeRunnerSudo struct {
	mu        sync.Mutex
	passwords []string
	drops     int
	ranFirst  bool
	argsFile  string
	stdinFile string
	probeErr  error
	authErr   error
}

func installFakeRunnerSudo(t *testing.T, exitStatus int) *fakeRunnerSudo {
	t.Helper()
	dir := t.TempDir()
	f := &fakeRunnerSudo{argsFile: filepath.Join(dir, "args"), stdinFile: filepath.Join(dir, "stdin")}
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > '%s'\ncat > '%s'\n", f.argsFile, f.stdinFile)
	if exitStatus != 0 {
		script += fmt.Sprintf("echo 'sudo: a password is required' >&2\nexit %d\n", exitStatus)
	}
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o755); err != nil {
		t.Fatalf("write sudo stand-in: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Cleanup(sudoticket.ReplaceProcessesForTesting(sudoticket.Processes{
		Validate: func(_ context.Context, _ string, password string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.passwords = append(f.passwords, password)
			if f.authErr != nil {
				return []byte("Sorry, try again."), f.authErr
			}
			return nil, nil
		},
		Probe: func(context.Context, string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.probeErr
		},
		Drop: func() {
			_, statErr := os.Stat(f.argsFile)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.drops++
			f.ranFirst = statErr == nil
		},
	}))
	return f
}

func TestExecCommandRunnerRunsSudoUnderTheSharedTicket(t *testing.T) {
	skipWhenRoot(t)
	f := installFakeRunnerSudo(t, 0)
	payload := []byte("payload line\n")

	if _, err := (execCommandRunner{}).Run(context.Background(), sudoRunnerOptions(), true, "net", []string{"conf", "list"}, payload); err != nil {
		t.Fatalf("Run: %v", err)
	}
	args, err := os.ReadFile(f.argsFile)
	if err != nil {
		t.Fatalf("sudo stand-in did not run: %v", err)
	}
	if got := strings.Fields(string(args)); !reflect.DeepEqual(got, []string{"-n", "--", "net", "conf", "list"}) {
		t.Fatalf("sudo args = %q, want -n -- net conf list", got)
	}
	stdin, err := os.ReadFile(f.stdinFile)
	if err != nil || string(stdin) != string(payload) {
		t.Fatalf("sudo stdin = %q (err %v), want only the command input", stdin, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.passwords, []string{runnerSudoPassword}) || f.drops != 1 || !f.ranFirst {
		t.Fatalf("validations=%d drops=%d dropAfterRun=%v, want one validation and a drop after the run", len(f.passwords), f.drops, f.ranFirst)
	}
}

func TestExecCommandRunnerExplainsSudoRefusalAfterFailedProbe(t *testing.T) {
	skipWhenRoot(t)
	f := installFakeRunnerSudo(t, 1)
	f.probeErr = errors.New("exit status 1")

	_, err := (execCommandRunner{}).Run(context.Background(), sudoRunnerOptions(), true, "exportfs", []string{"-ra"}, nil)
	if !errors.Is(err, sudoticket.ErrTimestampDisabled) || !strings.Contains(err.Error(), "a password is required") {
		t.Fatalf("Run err = %v, want sudo's message with ErrTimestampDisabled attached", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.drops != 1 {
		t.Fatalf("drops = %d, want the ticket released after the failed run", f.drops)
	}
}

func TestExecCommandRunnerStopsOnSudoAuthenticationFailure(t *testing.T) {
	skipWhenRoot(t)
	f := installFakeRunnerSudo(t, 0)
	f.authErr = errors.New("exit status 1")

	_, err := (execCommandRunner{}).Run(context.Background(), sudoRunnerOptions(), true, "exportfs", []string{"-ra"}, nil)
	if err == nil || !strings.Contains(err.Error(), "sudo authentication failed") || !strings.Contains(err.Error(), "Sorry, try again.") {
		t.Fatalf("Run err = %v, want the sudo authentication failure", err)
	}
	if _, statErr := os.Stat(f.argsFile); !os.IsNotExist(statErr) {
		t.Fatalf("the command must not run after a failed sudo authentication (stat err %v)", statErr)
	}
}
