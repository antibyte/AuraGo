package invasion

import (
	"context"
	"errors"
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
	}
	previousCommand, previousTransfer := sshRemoteCommand, sshTransferFile
	t.Cleanup(func() { sshRemoteCommand, sshTransferFile = previousCommand, previousTransfer })
	sshRemoteCommand = func(ctx context.Context, host string, port int, user string, secret []byte, command string, input ...io.Reader) (string, error) {
		if command == "bash -s" {
			return "fresh\n", nil
		}
		if !strings.HasPrefix(command, "mkdir -p ") && !strings.Contains(command, "cp -a ") {
			return "", nil
		}
		command = strings.ReplaceAll(command, "$HOME", shellQuote(filepath.ToSlash(root)))
		cmd := exec.CommandContext(ctx, bash, "-c", command)
		cmd.Env = append(sandbox.FilterEnv(os.Environ()), "BASH_ENV=")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		return string(output), err
	}
	sshTransferFile = func(ctx context.Context, host string, port int, user string, secret []byte, local, remote, direction string, extra ...string) error {
		return errors.New("synthetic binary transfer failure")
	}
	err = (&SSHConnector{}).Deploy(context.Background(), nest, nil, sshDeployTestPayload())
	if !errors.Is(err, ErrEggConfigNotDelivered) {
		t.Fatalf("unexpected failure phase: %v", err)
	}
	if got, readErr := os.ReadFile(dataPath); readErr != nil || string(got) != "only recovery copy" {
		t.Fatalf("existing orphan recovery copy was destroyed before configuration transfer: read=%v, deploy=%v", readErr, err)
	}
}
