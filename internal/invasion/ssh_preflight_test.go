package invasion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/sandbox"
)

func TestSSHEggMasterKeyPreflightPreservesExistingFiles(t *testing.T) {
	bash := "bash"
	if runtime.GOOS == "windows" {
		bash = `C:\Program Files\Git\bin\bash.exe`
		if _, err := os.Stat(bash); err != nil {
			t.Skip("Git Bash unavailable; Linux CI requires bash")
		}
	}
	for _, tc := range []struct {
		name, env      string
		vault, allowed bool
	}{
		{"fresh", "", false, true}, {"missing-key", "", true, false}, {"invalid-key", "AURAGO_MASTER_KEY=invalid\n", true, false},
		{"existing-key", "AURAGO_MASTER_KEY=" + strings.Repeat("a", 64) + "\n", true, true},
		{"ambiguous-key", "AURAGO_MASTER_KEY=" + strings.Repeat("a", 64) + "\nAURAGO_MASTER_KEY=" + strings.Repeat("b", 64) + "\n", true, false},
		{"quoted-key", "AURAGO_MASTER_KEY=\"" + strings.Repeat("a", 64) + "\"\n", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "data"), 0700); err != nil {
				t.Fatal(err)
			}
			if tc.env != "" {
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tc.env), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.vault {
				if err := os.WriteFile(filepath.Join(dir, "data/vault.bin"), []byte("encrypted-vault-fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			run := func(script string) ([]byte, error) {
				cmd := exec.Command(bash, "-s")
				cmd.Env = append(sandbox.FilterEnv(os.Environ()), "BASH_ENV=")
				cmd.Dir = dir
				cmd.Stdin = strings.NewReader(script)
				return cmd.CombinedOutput()
			}
			output, err := run(sshEggPreflightScript("."))
			if (err == nil) != tc.allowed {
				t.Fatalf("preflight allowed=%v: %s, %v", tc.allowed, output, err)
			}
			if strings.Contains(string(output), strings.Repeat("a", 64)) {
				t.Fatal("preflight returned the original key")
			}
			_, err = run(sshDeployFileScript("./.env", []byte("AURAGO_MASTER_KEY="+strings.Repeat("b", 64)+"\n"), true))
			if (err == nil) != tc.allowed {
				t.Fatalf("publish allowed=%v: %v", tc.allowed, err)
			}
			got, readErr := os.ReadFile(filepath.Join(dir, ".env"))
			if tc.env != "" && (readErr != nil || string(got) != tc.env) {
				t.Fatal("existing .env changed")
			}
			if tc.vault {
				data, err := os.ReadFile(filepath.Join(dir, "data/vault.bin"))
				if err != nil || string(data) != "encrypted-vault-fixture" {
					t.Fatal("existing Vault changed")
				}
			}
			if !tc.allowed && tc.env == "" && !os.IsNotExist(readErr) {
				t.Fatal("new key created over an existing Vault")
			}
		})
	}
}

func TestSSHPreflightFailureNeverStopsOrReconfigures(t *testing.T) {
	steps := fakeSSHDeploy(t, func(name string) bool { return name == "key-preflight" })
	if err := (&SSHConnector{}).Deploy(context.Background(), sshDeployTestNest(), nil, sshDeployTestPayload()); !errors.Is(err, ErrEggConfigNotDelivered) {
		t.Fatal(err)
	}
	if got := strings.Join(*steps, ","); got != "key-preflight" {
		t.Fatal("preflight failure changed deployment", got)
	}
	*steps = nil
	if err := (&SSHConnector{}).Reconfigure(context.Background(), sshDeployTestNest(), nil, []byte("egg_mode: {}"), []byte("bootstrap")); !errors.Is(err, ErrEggConfigNotDelivered) {
		t.Fatal(err)
	}
	if got := strings.Join(*steps, ","); got != "key-preflight" {
		t.Fatal("preflight failure changed configuration", got)
	}
}
