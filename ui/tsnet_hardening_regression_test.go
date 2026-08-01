package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTsNetConfigUIKeepsPerNodeLifecycleContract(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("cfg", "tailscale.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(source)
	for _, required := range []string{
		"['main', 'manifest', 'space_agent']",
		"/api/tsnet/credentials",
		"/api/tsnet/reauth",
		"TSNET_STATE_CORRUPT",
		"TSNET_NODE_NOT_CONFIGURED",
		"confirm_new_identity",
		"hasUnsavedConfigChanges",
		"_tsnetPollOperationID",
		"setTimeout(poll, 2000)",
		"state.configured !== true",
		"_tsnetStopPolling",
		"aurago:config-saved",
		"cfg:section-leave",
		"beforeunload",
	} {
		if !strings.Contains(js, required) {
			t.Fatalf("tailscale.js is missing lifecycle marker %q", required)
		}
	}
	if strings.Contains(js, "setInterval(") {
		t.Fatal("tsnet polling must not overlap requests through setInterval")
	}
}

func TestTsNetHardeningTranslationsExistInEveryLanguage(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("lang", "config", "tailscale", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 16 {
		t.Fatalf("translation file count = %d, want 16", len(files))
	}
	required := []string{
		"config.tailscale.tsnet_node_main",
		"config.tailscale.tsnet_node_manifest",
		"config.tailscale.tsnet_node_space_agent",
		"config.tailscale.tsnet_key_source_node_vault",
		"config.tailscale.tsnet_key_source_shared_vault",
		"config.tailscale.tsnet_credential_saved_pending",
		"config.tailscale.tsnet_reauth",
		"config.tailscale.tsnet_recover_state",
		"config.tailscale.tsnet_recover_confirm",
		"config.tailscale.tsnet_shared_key_warning",
		"config.tailscale.tsnet_error_auth_key_rejected",
		"config.tailscale.tsnet_error_state_corrupt",
		"config.tailscale.tsnet_error_backend_unavailable",
		"config.tailscale.tsnet_error_node_not_configured",
		"config.tailscale.tsnet_reconfigure",
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(messages[key]) == "" {
				t.Fatalf("%s is missing non-empty translation %q", path, key)
			}
		}
	}
}
