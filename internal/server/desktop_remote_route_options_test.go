package server

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopRemoteRoutesPassConfiguredProxyOptions(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("server_routes.go")
	if err != nil {
		t.Fatalf("read server_routes.go: %v", err)
	}
	source := string(data)
	for _, marker := range []string{
		"remoteProxyOptions := desktop.RemoteProxyOptionsFromConfig(desktop.ConfigFromAuraConfig(s.Cfg))",
		"desktop.HandleSSHProxy(s.InventoryDB, s.Vault, s.Logger, remoteProxyOptions)",
		"desktop.HandleVNCProxy(s.InventoryDB, s.Vault, s.Logger, remoteProxyOptions)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("server routes missing remote proxy option marker %q", marker)
		}
	}
}
