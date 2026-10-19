package tools

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestBrowserAutomationTokenSurvivesPrivateStartupKeyHandoff(t *testing.T) {
	previous := security.ProcessMasterKey()
	t.Cleanup(func() { security.RetainProcessMasterKey(previous) })
	key := strings.Repeat("a7", 32)
	security.RetainProcessMasterKey(key)
	t.Setenv("AURAGO_MASTER_KEY", "")
	t.Setenv("AURAGO_BROWSER_AUTOMATION_TOKEN", "")
	if err := os.Unsetenv("AURAGO_MASTER_KEY"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte("aurago/browser-automation-sidecar/v1"))
	want := hex.EncodeToString(mac.Sum(nil))
	if browserAutomationAuthToken(cfg) != want {
		t.Fatal("browser sidecar lost its stable token after the startup handoff")
	}
	if _, present := os.LookupEnv("AURAGO_MASTER_KEY"); present || cfg.Server.MasterKey != "" {
		t.Fatal("token derivation restored an ambient/config master key")
	}
	// A stale run snapshot cannot change the process-owned sidecar identity.
	cfg.Server.MasterKey = strings.Repeat("b8", 32)
	if browserAutomationAuthToken(cfg) != want {
		t.Fatal("run snapshot replaced the retained startup key")
	}
	cfg.Server.MasterKey = ""
	if _, present := os.LookupEnv("AURAGO_MASTER_KEY"); present || cfg.Server.MasterKey != "" {
		t.Fatal("token derivation restored an ambient/config master key")
	}
	t.Setenv("AURAGO_BROWSER_AUTOMATION_TOKEN", "explicit-sidecar-fixture")
	if browserAutomationAuthToken(cfg) != "explicit-sidecar-fixture" {
		t.Fatal("explicit sidecar token lost precedence")
	}
	for _, env := range [][]string{filteredShellEnv(), dockerCLIMinimalEnvironment(os.Environ())} {
		for _, assignment := range env {
			if strings.Contains(assignment, key) || strings.HasPrefix(assignment, "AURAGO_MASTER_KEY=") || strings.HasPrefix(assignment, "AURAGO_BROWSER_AUTOMATION_TOKEN=") {
				t.Fatal("private startup or sidecar credential leaked into a child environment")
			}
		}
	}
	for _, assignment := range dockerCLIEnvironment(os.Environ()) {
		if strings.Contains(assignment, key) || strings.HasPrefix(assignment, "AURAGO_MASTER_KEY=") {
			t.Fatal("Docker child inherited the private startup key")
		}
	}
	cmd := exec.Command("unexecuted-child-fixture")
	ensureFilteredEnv(cmd)
	for _, assignment := range cmd.Env {
		if strings.Contains(assignment, key) || strings.HasPrefix(assignment, "AURAGO_MASTER_KEY=") || strings.HasPrefix(assignment, "AURAGO_BROWSER_AUTOMATION_TOKEN=") {
			t.Fatal("integration child inherited a private startup or sidecar credential")
		}
	}
}

func TestBrowserAutomationTokenStandaloneKeyFallbacks(t *testing.T) {
	previous := security.ProcessMasterKey()
	t.Cleanup(func() { security.RetainProcessMasterKey(previous) })
	security.RetainProcessMasterKey("")
	t.Setenv("AURAGO_BROWSER_AUTOMATION_TOKEN", "")
	key := strings.Repeat("d9", 32)
	cfg := &config.Config{}
	cfg.Server.MasterKey = key
	fromConfig := browserAutomationAuthToken(cfg)
	cfg.Server.MasterKey = ""
	t.Setenv("AURAGO_MASTER_KEY", key)
	if fromConfig == "" || browserAutomationAuthToken(cfg) != fromConfig {
		t.Fatal("standalone config/environment key compatibility changed")
	}
}
