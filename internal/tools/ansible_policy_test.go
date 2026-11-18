package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnsibleSharedLiteralPolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "ansible_api", "policy_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		ExtraVars []struct {
			Name, Raw string
			Valid     bool
		} `json:"extra_vars"`
		ModuleArgs []struct {
			Name, Raw string
			Valid     bool
		} `json:"module_args"`
		SelectedHosts []struct {
			Name, Stdout string
			Valid        bool
		} `json:"selected_hosts"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases.ExtraVars {
		t.Run(c.Name, func(t *testing.T) {
			if err := validateAnsibleExtraVars(c.Raw); (err == nil) != c.Valid {
				t.Fatalf("accepted=%v, want %v: %v", err == nil, c.Valid, err)
			}
		})
	}
	for _, c := range cases.ModuleArgs {
		t.Run(c.Name, func(t *testing.T) {
			err := validateAnsibleModuleArgs(c.Raw)
			if (err == nil) != c.Valid {
				t.Fatalf("accepted=%v, want %v: %v", err == nil, c.Valid, err)
			}
			// Deny process execution so the test exercises only admission.
			cfg := AnsibleLocalConfig{ReadOnly: true, Context: WithRuntimePermissions(context.Background(), RuntimePermissions{})}
			_, _, runErr := ansibleRunCmd(cfg, "ansible", "server", "-m", "ping", "-a", c.Raw)
			if runErr == nil || (err != nil && runErr.Error() != err.Error()) || (err == nil && !strings.Contains(runErr.Error(), "shell")) {
				t.Fatalf("local admission error = %v, validation error = %v", runErr, err)
			}
		})
	}
	for _, c := range cases.SelectedHosts {
		t.Run(c.Name, func(t *testing.T) {
			if err := validateAnsibleSelectedHosts(c.Stdout, map[string]bool{"server": true, "worker": true}); (err == nil) != c.Valid {
				t.Fatalf("accepted=%v, want %v: %v", err == nil, c.Valid, err)
			}
		})
	}
}

func TestAnsibleRejectsTemplatesBeforeStagingAndAfterInventoryDecode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hosts.ini"), []byte(`server ansible_host="{{ lookup('env','FIXTURE') }}"`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := stageAnsibleInputs(context.Background(), AnsibleLocalConfig{PlaybooksDir: root, WorkDir: t.TempDir()}, "ansible", []string{"all", "-i", "hosts.ini"}); err == nil {
		t.Fatal("raw inventory template accepted")
	}
	if _, err := ansibleDeclaredHosts(`{"all":{"hosts":["server"]},"_meta":{"hostvars":{"server":{"ansible_host":"\u007b\u007b lookup('env','FIXTURE') }}"}}}}`); err == nil {
		t.Fatal("encoded inventory template accepted")
	}
	for _, args := range [][]string{{"all", "-e", `{"ansible_host":"192.0.2.2"}`}, {"all", "--limit", "@/fixture/private"}} {
		if _, _, err := stageAnsibleInputs(context.Background(), AnsibleLocalConfig{PlaybooksDir: root, WorkDir: t.TempDir()}, "ansible", args); err == nil {
			t.Fatalf("unsafe arguments accepted: %v", args)
		}
	}
}
