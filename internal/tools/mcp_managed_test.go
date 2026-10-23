package tools

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
)

type managedProbeTransport struct{ closed atomic.Bool }

func (*managedProbeTransport) Send(context.Context, string, interface{}) (*jsonRPCResponse, error) {
	return nil, errors.New("unexpected request")
}
func (*managedProbeTransport) Notify(context.Context, string, interface{}) error { return nil }
func (t *managedProbeTransport) Close()                                          { t.closed.Store(true) }

func managedMCPManagerFixture(t *testing.T, configs map[string]MCPServerConfig) *MCPManager {
	t.Helper()
	mgr := &MCPManager{configs: configs, conns: map[string]*mcpConn{}, logger: testLogger()}
	mcpManagerMu.Lock()
	previous := globalMCPManager
	globalMCPManager = mgr
	mcpManagerMu.Unlock()
	t.Cleanup(func() { mgr.Close(); mcpManagerMu.Lock(); globalMCPManager = previous; mcpManagerMu.Unlock() })
	return mgr
}

func TestManagedMCPRevocationPreservesOtherServers(t *testing.T) {
	mgr := managedMCPManagerFixture(t, map[string]MCPServerConfig{"printguard-store": {Name: "printguard-store", Enabled: true, ManagedRevision: "old", AllowedTools: []string{"get_state"}}, "custom": {Name: "custom", Enabled: true}})
	managed, custom := &managedProbeTransport{}, &managedProbeTransport{}
	mgr.conns["printguard-store"] = &mcpConn{ready: true, transport: managed}
	mgr.conns["custom"] = &mcpConn{ready: true, transport: custom}
	if err := mgr.requireToolAllowed("printguard-store", "control_printer"); err == nil {
		t.Fatal("read grant permitted control")
	}
	if err := SetManagedMCPServer("printguard-store", nil); err != nil {
		t.Fatal(err)
	}
	if !managed.closed.Load() || custom.closed.Load() || mgr.configs["custom"].Name != "custom" {
		t.Fatal("revocation crossed server ownership")
	}
	if err := SetManagedMCPServer("custom", &MCPServerConfig{Name: "custom", Enabled: true, ManagedRevision: "managed"}); err == nil {
		t.Fatal("custom server overwritten")
	}
	if custom.closed.Load() {
		t.Fatal("collision interrupted custom connection")
	}
}

func TestManagedMCPRejectsInFlightStaleConnect(t *testing.T) {
	mgr := managedMCPManagerFixture(t, map[string]MCPServerConfig{"managed": {Name: "managed", Enabled: true, Command: "fixture", ManagedRevision: "old"}})
	oldStart := startManagedMCPConn
	t.Cleanup(func() { startManagedMCPConn = oldStart })
	entered, release := make(chan struct{}), make(chan struct{})
	transport := &managedProbeTransport{}
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		close(entered)
		<-release
		return &mcpConn{ready: true, transport: transport}, nil
	}
	done := make(chan error, 1)
	go func() { _, err := mgr.ensureServerConnected(context.Background(), "managed"); done <- err }()
	<-entered
	if err := SetManagedMCPServer("managed", &MCPServerConfig{Name: "managed", Enabled: true, Command: "fixture", ManagedRevision: "new"}); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("stale connection was published")
	}
	if !transport.closed.Load() || mgr.conns["managed"] != nil {
		t.Fatal("stale transport survived")
	}
}

func TestManagedMCPRechecksOwnerBeforeExecution(t *testing.T) {
	revoked := false
	mgr := managedMCPManagerFixture(t, map[string]MCPServerConfig{"managed": {Name: "managed", Enabled: true, Command: "fixture", ManagedRevision: "r", AllowedTools: []string{"control_printer"}, CheckAccess: func() error {
		if revoked {
			return errors.New("owner revoked")
		}
		return nil
	}}})
	if err := mgr.requireToolAllowed("managed", "control_printer"); err != nil {
		t.Fatal(err)
	}
	revoked = true
	if _, err := mgr.CallTool(context.Background(), "managed", "control_printer", map[string]interface{}{"action": "pause"}); err == nil || err.Error() != "owner revoked" {
		t.Fatalf("revoked owner reached transport: %v", err)
	}
}
