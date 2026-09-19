package tools

import (
	"aurago/internal/config"
	"testing"
	"time"
)

func TestComposioDiscoveryRequiresVerifiedConnection(t *testing.T) {
	cfg := config.ComposioConfig{UserID: "disclosure-fixture-user"}
	client := NewComposioClientFromConfig(cfg)
	key := composioConnectionKey(client.baseURL, client.apiKey, cfg.UserID, "calendar")
	t.Cleanup(func() {
		composioConnections.Lock()
		delete(composioConnections.entries, key)
		composioConnections.Unlock()
	})
	if got := ComposioCachedConnectionState(cfg, "calendar"); got != "connection_unknown" {
		t.Fatal(got)
	}
	client.rememberConnections("calendar", cfg.UserID, ComposioListPage[ComposioConnectedAccount]{})
	if got := ComposioCachedConnectionState(cfg, "calendar"); got != "connect_required" {
		t.Fatal(got)
	}
	client.rememberConnections("calendar", cfg.UserID, ComposioListPage[ComposioConnectedAccount]{Items: []ComposioConnectedAccount{{ID: "fixture", ToolkitSlug: "calendar", Status: "ACTIVE"}}})
	if got := ComposioCachedConnectionState(cfg, "calendar"); got != "connected" {
		t.Fatal(got)
	}
	composioConnections.Lock()
	snapshot := composioConnections.entries[key]
	snapshot.expires = time.Now().Add(-time.Second)
	composioConnections.entries[key] = snapshot
	composioConnections.Unlock()
	if got := ComposioCachedConnectionState(cfg, "calendar"); got != "connection_unknown" {
		t.Fatal(got)
	}
}
