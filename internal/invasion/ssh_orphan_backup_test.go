package invasion

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
	"testing"

	"aurago/internal/sandbox"
)

func TestSSHConnectorDeployPreservesOrphanRecoveryBackup(t *testing.T) {
	for _, emptyPrimary := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty-primary-%v", emptyPrimary), func(t *testing.T) {
			root := t.TempDir()
			nest := sshDeployTestNest()
			base, err := sshEggBaseDir(nest.ID)
			if err != nil {
				t.Fatal(err)
			}
			backup := filepath.Join(root, strings.TrimPrefix(base, "~/")) + ".bak"
			rel, err := filepath.Rel(root, backup)
			if err != nil || strings.HasPrefix(rel, "..") {
				t.Fatal("fixture escaped temporary root")
			}
			if emptyPrimary {
				if err := os.MkdirAll(filepath.Join(root, strings.TrimPrefix(base, "~/"), "data"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			dataPath := filepath.Join(backup, "data", "original.db")
			if err := os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dataPath, []byte("only recovery copy"), 0600); err != nil {
				t.Fatal(err)
			}
			bash := "bash"
			if runtime.GOOS == "windows" {
				bash = `C:\Program Files\Git\bin\bash.exe`
				if _, err := os.Stat(bash); err != nil {
					t.Skip("Git Bash unavailable; Linux CI requires bash")
				}
			}
			previousCommand, previousTransfer := sshRemoteCommand, sshTransferFile
			t.Cleanup(func() { sshRemoteCommand, sshTransferFile = previousCommand, previousTransfer })
			mutations, uploads := 0, 0
			sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, command string, input ...io.Reader) (string, error) {
				if command == "bash -s" {
					if len(input) != 1 {
						t.Fatal("missing preflight input")
					}
					data, err := io.ReadAll(input[0])
					if err != nil {
						t.Fatal(err)
					}
					command = string(data)
				} else {
					mutations++
					if !strings.HasPrefix(command, "mkdir -p ") && !strings.Contains(command, "cp -a ") {
						return "", nil
					}
				}
				command = strings.ReplaceAll(command, "$HOME", shellQuote(filepath.ToSlash(root)))
				cmd := exec.CommandContext(ctx, bash, "-c", command)
				cmd.Env = append(sandbox.FilterEnv(os.Environ()), "BASH_ENV=")
				cmd.Dir = root
				output, err := cmd.CombinedOutput()
				if err != nil {
					return string(output), fmt.Errorf("command execution failed: %w", err)
				}
				return string(output), nil
			}
			sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, local, remote, direction string, extra ...string) error {
				uploads++
				return errors.New("synthetic binary transfer failure")
			}
			for attempt := 0; attempt < 2; attempt++ {
				err = (&SSHConnector{}).Deploy(context.Background(), nest, nil, sshDeployTestPayload())
				if !errors.Is(err, ErrEggConfigNotDelivered) {
					t.Fatalf("unexpected failure phase: %v", err)
				}
				if got, readErr := os.ReadFile(dataPath); readErr != nil || string(got) != "only recovery copy" {
					t.Fatalf("attempt %d destroyed the orphan recovery copy: read=%v, deploy=%v", attempt+1, readErr, err)
				}
			}
			err = (&SSHConnector{}).Reconfigure(context.Background(), nest, nil, []byte("egg_mode: {}"))
			if !errors.Is(err, ErrEggConfigNotDelivered) {
				t.Fatalf("reconfigure bypassed orphan preflight: %v", err)
			}
			if mutations != 0 || uploads != 0 || !strings.Contains(err.Error(), filepath.Base(backup)) {
				t.Fatalf("orphan recovery did not stop at preflight: mutations=%d uploads=%d error=%v", mutations, uploads, err)
			}
		})
	}
}
