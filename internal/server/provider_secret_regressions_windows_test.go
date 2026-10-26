package server

import (
	"bytes"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProviderSetupStorageFailureDoesNotChangeAnyCredential(t *testing.T) {
	s, vault := newProviderTestServer(t, providerSafetyConfig)
	for _, key := range []string{"ai_gateway_token", "provider_main_api_key"} {
		if err := vault.WriteSecret(key, "syntheticOldSetupKey123"); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, revision := s.ConfigSnapshot(), s.providerETag()
	path, err := windows.UTF16PtrFromString(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	patch := map[string]interface{}{
		"providers":  []interface{}{map[string]interface{}{"id": "main", "type": "openai", "model": "gpt-4o-mini", "api_key": "syntheticNewProviderKey123"}},
		"ai_gateway": map[string]interface{}{"token": "syntheticNewGatewayToken123"},
	}
	if _, err := applyConfigPatch(s, patch); err == nil {
		t.Fatal("fixture did not reject the config replacement")
	}
	after, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed save changed the config file")
	}
	if s.ConfigSnapshot() != snapshot || s.providerETag() != revision {
		t.Fatal("failed save published a snapshot or revision")
	}
	for _, key := range []string{"ai_gateway_token", "provider_main_api_key"} {
		got, err := vault.ReadSecret(key)
		if err != nil || got != "syntheticOldSetupKey123" {
			t.Fatal("failed setup save changed a Vault credential")
		}
	}
}
