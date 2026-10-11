package tools

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestMCPReconnectSingleFlightAndCancelledWaiter(t *testing.T) {
	old := startManagedMCPConn
	defer func() { startManagedMCPConn = old }()
	entered, release := make(chan struct{}), make(chan struct{})
	starts := 0
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		starts++
		close(entered)
		<-release
		return &mcpConn{ready: true}, nil
	}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{}, configs: map[string]MCPServerConfig{"s": {Name: "s", Enabled: true, Command: "fixture"}}}
	done := make(chan error, 1)
	go func() { _, err := mgr.ensureServerConnected(context.Background(), "s"); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := mgr.ensureServerConnected(ctx, "s"); err == nil {
		t.Fatal("waiting reconnect ignored cancellation")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("starts=%d", starts)
	}
	mgr.Close()
}

func TestMCPReloadRejectsOlderGeneration(t *testing.T) {
	old := startManagedMCPConn
	defer func() { startManagedMCPConn = old; ShutdownMCPManager() }()
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		t.Fatal("obsolete generation started")
		return nil, nil
	}
	first := RevokeMCPManager()
	latest := RevokeMCPManager()
	current := InitMCPManagerGeneration(nil, testLogger(), latest)
	obsolete := InitMCPManagerGeneration([]MCPServerConfig{{Name: "s", Enabled: true, Command: "fixture"}}, testLogger(), first)
	if GetMCPManager() != current || !obsolete.closed {
		t.Fatal("obsolete manager published")
	}
}

func TestMCPInvalidationPreservesReplacement(t *testing.T) {
	old, current := &mcpConn{}, &mcpConn{}
	mgr := &MCPManager{conns: map[string]*mcpConn{"s": current}}
	mgr.invalidateConnection("s", old, io.EOF)
	if mgr.conns["s"] != current {
		t.Fatal("old error invalidated new connection")
	}
}
func TestRegressionMCPCloseAllowsReconnect(t *testing.T) {
	oldStart, oldInvoke := startManagedMCPConn, invokeMCPConnTool
	defer func() { startManagedMCPConn, invokeMCPConnTool = oldStart, oldInvoke }()
	starts, calls := 0, 0
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		starts++
		return &mcpConn{name: "fixture", ready: true}, nil
	}
	invokeMCPConnTool = func(context.Context, *mcpConn, string, map[string]interface{}) (string, error) {
		calls++
		return "executed", nil
	}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{}, configs: map[string]MCPServerConfig{
		"fixture": {Name: "fixture", Transport: "streamable_http", URL: "https://unused.invalid", Enabled: true},
	}}
	mgr.Close()
	_, err := mgr.CallTool(context.Background(), "fixture", "read", nil)
	if err == nil || starts != 0 || calls != 0 {
		t.Fatalf("closed manager dispatched: err=%v starts=%d calls=%d", err, starts, calls)
	}
}

func TestRegressionMCPInFlightReconnectSurvivesClose(t *testing.T) {
	oldStart, oldInvoke := startManagedMCPConn, invokeMCPConnTool
	defer func() { startManagedMCPConn, invokeMCPConnTool = oldStart, oldInvoke }()
	entered, release := make(chan struct{}), make(chan struct{})
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		close(entered)
		<-release
		return &mcpConn{name: "fixture", ready: true}, nil
	}
	invokeMCPConnTool = func(context.Context, *mcpConn, string, map[string]interface{}) (string, error) {
		return "executed after shutdown", nil
	}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{}, configs: map[string]MCPServerConfig{
		"fixture": {Name: "fixture", Transport: "streamable_http", Enabled: true},
	}}
	done := make(chan string, 1)
	go func() {
		result, err := mgr.CallTool(context.Background(), "fixture", "read", nil)
		if err != nil {
			result = err.Error()
		}
		done <- result
	}()
	<-entered
	mgr.Close()
	close(release)
	if result := <-done; result == "executed after shutdown" {
		t.Fatalf("unexpected result %s", result)
	}
}

func TestRegressionMCPDockerCommandCannotReconnect(t *testing.T) {
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{}, configs: map[string]MCPServerConfig{
		"docker-only": {Name: "docker-only", Runtime: "docker", DockerImage: "fixture", DockerCommand: "server", Enabled: true},
	}}
	old := startManagedMCPConn
	defer func() { startManagedMCPConn = old }()
	starts := 0
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) {
		starts++
		return &mcpConn{ready: true}, nil
	}
	_, err := mgr.ensureServerConnected(context.Background(), "docker-only")
	if err != nil || starts != 1 || len(mgr.configuredServerNames()) != 1 {
		t.Fatalf("unexpected result: %v", err)
	}
}

type auditMCPEOFTransport struct{}

func (*auditMCPEOFTransport) Send(context.Context, string, interface{}) (*jsonRPCResponse, error) {
	return nil, io.EOF
}
func (*auditMCPEOFTransport) Notify(context.Context, string, interface{}) error { return nil }
func (*auditMCPEOFTransport) Close()                                            {}

func TestRegressionMCPCatalogFailureNeverReconnects(t *testing.T) {
	old := startManagedMCPConn
	defer func() { startManagedMCPConn = old }()
	starts := 0
	startManagedMCPConn = func(context.Context, MCPServerConfig, *slog.Logger) (*mcpConn, error) { starts++; return nil, io.EOF }
	conn := &mcpConn{name: "fixture", ready: true, transport: &auditMCPEOFTransport{}, toolsRefreshed: time.Now().Add(-2 * time.Minute)}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{"fixture": conn}, configs: map[string]MCPServerConfig{"fixture": {Name: "fixture", Enabled: true, Transport: "streamable_http"}}}
	for i := 0; i < 2; i++ {
		if _, err := mgr.CallTool(context.Background(), "fixture", "read", nil); err == nil {
			t.Fatal("expected EOF")
		}
	}
	if starts != 1 || mgr.conns["fixture"] != nil {
		t.Fatal("discovery failure did not invalidate the connection")
	}
}
