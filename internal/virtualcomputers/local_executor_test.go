package virtualcomputers

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"

	"aurago/internal/sudoticket"
)

const localExecutorSudoPassword = "vault-sudo-secret"

// fakeSudoTicket stands in for the sudo processes behind the shared ticket.
type fakeSudoTicket struct {
	mu          sync.Mutex
	passwords   []string
	valid       bool
	drops       int
	validateErr error
	probeErr    error
}

func installFakeSudoTicket(t *testing.T) *fakeSudoTicket {
	t.Helper()
	f := &fakeSudoTicket{}
	t.Cleanup(sudoticket.ReplaceProcessesForTesting(sudoticket.Processes{
		Validate: func(_ context.Context, _ string, password string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.passwords = append(f.passwords, password)
			if f.validateErr != nil {
				return []byte("Sorry, try again."), f.validateErr
			}
			f.valid = true
			return nil, nil
		},
		Probe: func(context.Context, string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.probeErr
		},
		Drop: func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.drops++
			f.valid = false
		},
	}))
	return f
}

func (f *fakeSudoTicket) snapshot() (passwords []string, valid bool, drops int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.passwords...), f.valid, f.drops
}

// assertNoPasswordOnSudoCommand builds the process the way the default input
// runner does and checks that only the caller's input reaches its stdin.
func assertNoPasswordOnSudoCommand(t *testing.T, cmd *exec.Cmd, wantStdin string) {
	t.Helper()
	for _, arg := range cmd.Args {
		if arg == "-S" || strings.Contains(arg, localExecutorSudoPassword) {
			t.Fatalf("sudo command must not read or carry the password: %q", cmd.Args)
		}
	}
	if len(cmd.Args) < 2 || cmd.Args[0] != "sudo" || cmd.Args[1] != "-n" {
		t.Fatalf("sudo command must be non-interactive (sudo -n ...): %q", cmd.Args)
	}
	var stdin []byte
	if cmd.Stdin != nil {
		var err error
		if stdin, err = io.ReadAll(cmd.Stdin); err != nil {
			t.Fatalf("read stdin: %v", err)
		}
	}
	if strings.Contains(string(stdin), localExecutorSudoPassword) {
		t.Fatalf("sudo stdin carries the password: %q", stdin)
	}
	if string(stdin) != wantStdin {
		t.Fatalf("sudo stdin = %q, want %q", stdin, wantStdin)
	}
}

func TestLocalCommandExecutorRunScriptRunsSudoUnderTheSharedTicket(t *testing.T) {
	ticket := installFakeSudoTicket(t)
	var scriptPath string
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		TempDir:      t.TempDir(),
		PathExists:   func(string) bool { return false },
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(ctx context.Context, name string, args ...string) (string, error) {
			assertNoPasswordOnSudoCommand(t, exec.CommandContext(ctx, name, args...), "")
			if reflect.DeepEqual(args, []string{"-n", "true"}) {
				return "", errors.New("password required")
			}
			if len(args) != 3 || args[1] != "bash" {
				t.Fatalf("sudo args = %v, want [-n bash script]", args)
			}
			if _, valid, _ := ticket.snapshot(); !valid {
				t.Fatal("sudo -n bash must run while the shared ticket is held")
			}
			scriptPath = args[2]
			if _, err := os.Stat(scriptPath); err != nil {
				t.Fatalf("script should exist during execution: %v", err)
			}
			return "ok", nil
		},
		InputCommandRunner: func(_ context.Context, name, input string, args ...string) (string, error) {
			t.Fatalf("unexpected stdin command %s %v", name, args)
			return "", nil
		},
	}

	out, err := executor.RunScript(context.Background(), "echo local setup")
	if err != nil || strings.TrimSpace(out) != "ok" {
		t.Fatalf("RunScript output=%q err=%v", out, err)
	}
	if passwords, valid, drops := ticket.snapshot(); !reflect.DeepEqual(passwords, []string{localExecutorSudoPassword}) || valid || drops != 1 {
		t.Fatalf("ticket passwords=%d valid=%v drops=%d, want one validation and a dropped ticket", len(passwords), valid, drops)
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("temporary script should be removed, stat err=%v", err)
	}
}

func TestLocalCommandExecutorSystemdRunPipesOnlyTheScript(t *testing.T) {
	const script = "echo local setup"
	ticket := installFakeSudoTicket(t)
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		PathExists:   func(path string) bool { return path == "/run/systemd/system" },
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(_ context.Context, name string, args ...string) (string, error) {
			if name != "sudo" || !reflect.DeepEqual(args, []string{"-n", "true"}) {
				t.Fatalf("passwordless probe command=%q args=%v", name, args)
			}
			return "", errors.New("password required")
		},
		InputCommandRunner: func(ctx context.Context, name, input string, args ...string) (string, error) {
			// bash -s reads the script from stdin: a password line in front of
			// it would run as the first root command.
			assertNoPasswordOnSudoCommand(t, newInputCommand(ctx, name, input, args...), script)
			wantArgs := append([]string{"-n", "systemd-run"}, expectedTransientSystemdScriptArgs()...)
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("sudo args=%v, want %v", args, wantArgs)
			}
			if _, valid, _ := ticket.snapshot(); !valid {
				t.Fatal("sudo -n systemd-run must run while the shared ticket is held")
			}
			return "ok", nil
		},
	}

	out, err := executor.RunScript(context.Background(), script)
	if err != nil || strings.TrimSpace(out) != "ok" {
		t.Fatalf("RunScript output=%q err=%v", out, err)
	}
	if passwords, valid, drops := ticket.snapshot(); len(passwords) != 1 || valid || drops != 1 {
		t.Fatalf("ticket validations=%d valid=%v drops=%d, want one validation and a dropped ticket", len(passwords), valid, drops)
	}
}

func TestLocalCommandExecutorPasswordlessSudoSkipsTheTicket(t *testing.T) {
	const script = "echo local setup"
	ticket := installFakeSudoTicket(t)
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		PathExists:   func(path string) bool { return path == "/run/systemd/system" },
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(context.Context, string, ...string) (string, error) {
			return "", nil
		},
		InputCommandRunner: func(ctx context.Context, name, input string, args ...string) (string, error) {
			assertNoPasswordOnSudoCommand(t, newInputCommand(ctx, name, input, args...), script)
			return "ok", nil
		},
	}

	if _, err := executor.RunScript(context.Background(), script); err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if passwords, _, drops := ticket.snapshot(); len(passwords) != 0 || drops != 0 {
		t.Fatalf("validations=%d drops=%d, want no ticket when sudo -n true already succeeds", len(passwords), drops)
	}
}

func TestLocalCommandExecutorExplainsSudoRefusalAfterFailedProbe(t *testing.T) {
	ticket := installFakeSudoTicket(t)
	ticket.probeErr = errors.New("exit status 1")
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		PathExists:   func(path string) bool { return path == "/run/systemd/system" },
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(context.Context, string, ...string) (string, error) {
			return "", errors.New("password required")
		},
		InputCommandRunner: func(context.Context, string, string, ...string) (string, error) {
			return "sudo: a password is required", errors.New("exit status 1")
		},
	}

	_, err := executor.RunScript(context.Background(), "echo local setup")
	if !errors.Is(err, sudoticket.ErrTimestampDisabled) {
		t.Fatalf("RunScript err = %v, want ErrTimestampDisabled attached", err)
	}
	if _, _, drops := ticket.snapshot(); drops != 1 {
		t.Fatalf("drops = %d, want the ticket released after the failed run", drops)
	}
}

func TestLocalCommandExecutorReportsSudoAuthenticationFailure(t *testing.T) {
	ticket := installFakeSudoTicket(t)
	ticket.validateErr = errors.New("exit status 1")
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		PathExists:   func(path string) bool { return path == "/run/systemd/system" },
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(context.Context, string, ...string) (string, error) {
			return "", errors.New("password required")
		},
		InputCommandRunner: func(_ context.Context, name string, _ string, args ...string) (string, error) {
			t.Fatalf("command must not run after a failed sudo authentication: %s %v", name, args)
			return "", nil
		},
	}

	out, err := executor.RunScript(context.Background(), "echo local setup")
	if err == nil || !strings.Contains(err.Error(), "sudo authentication failed") || !strings.Contains(out, "Sorry, try again.") {
		t.Fatalf("RunScript output=%q err=%v, want the sudo authentication failure", out, err)
	}
}

func TestLocalCommandExecutorPreflightValidatesVaultSudoPasswordThroughTicket(t *testing.T) {
	ticket := installFakeSudoTicket(t)
	var probes int
	executor := LocalCommandExecutor{
		RuntimeGOOS:  "linux",
		EffectiveUID: func() int { return 1000 },
		SudoPassword: localExecutorSudoPassword,
		CommandRunner: func(ctx context.Context, name string, args ...string) (string, error) {
			assertNoPasswordOnSudoCommand(t, exec.CommandContext(ctx, name, args...), "")
			if !reflect.DeepEqual(args, []string{"-n", "true"}) {
				t.Fatalf("command=%q args=%v, want sudo -n true", name, args)
			}
			probes++
			if _, valid, _ := ticket.snapshot(); !valid {
				return "", errors.New("passwordless sudo denied")
			}
			return "", nil
		},
		InputCommandRunner: func(_ context.Context, name, _ string, args ...string) (string, error) {
			t.Fatalf("unexpected stdin command %s %v", name, args)
			return "", nil
		},
	}

	if !executor.hasSudoOrRoot(context.Background()) {
		t.Fatal("Vault sudo password should satisfy preflight")
	}
	if passwords, valid, drops := ticket.snapshot(); !reflect.DeepEqual(passwords, []string{localExecutorSudoPassword}) || valid || drops != 1 || probes != 2 {
		t.Fatalf("validations=%d valid=%v drops=%d probes=%d, want one validation, two sudo -n true runs and a dropped ticket", len(passwords), valid, drops, probes)
	}

	ticket.validateErr = errors.New("exit status 1")
	if executor.hasSudoOrRoot(context.Background()) {
		t.Fatal("a rejected Vault sudo password must not satisfy preflight")
	}
}
