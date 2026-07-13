package server

import "testing"

func TestManusConfigAPIKeyUsesCanonicalVaultKey(t *testing.T) {
	t.Parallel()

	if got := vaultKeyMap["manus.api_key"]; got != "manus_api_key" {
		t.Fatalf("vaultKeyMap[manus.api_key] = %q", got)
	}
}
