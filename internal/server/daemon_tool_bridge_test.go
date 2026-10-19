package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/tools"
)

func TestDaemonToolBridgeKeepsScopeQuotaExpiryAndBackgroundPolicy(t *testing.T) {
	calls := 0
	s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	dc.SessionID, dc.MessageSource = "daemon:fixture:generation-one", "background"
	dc.ToolScopeRestricted, dc.AllowedTools = true, map[string]struct{}{"docker": {}}
	dc.Cfg = s.Cfg.Clone()
	s.bindConfigAuthorization(dc.Cfg)
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	run := s.daemonToolBridgeRun(ctx, cancel, func() bool { return false }, func() {}, dc, func() time.Time { return now })
	defer run.Release()
	request := tools.DaemonToolRequest{ID: 1, Tool: "docker", Parameters: map[string]interface{}{"operation": "list"}, Timeout: 1}
	for i := 0; i < bridgeGrantCallLimit; i++ {
		if response := run.Call(ctx, request); response.Status != "success" {
			t.Fatalf("call %d: %+v", i, response)
		}
	}
	if response := run.Call(ctx, request); response.Status != "error" || calls != bridgeGrantCallLimit {
		t.Fatal("daemon refilled a lease before its expiry")
	}
	now = now.Add(bridgeGrantLifetime + time.Second)
	if response := run.Call(ctx, request); response.Status != "success" || calls != bridgeGrantCallLimit+1 {
		t.Fatal("living daemon could not renew its expired lease")
	}
	request.Parameters = map[string]interface{}{"operation": "remove", "container_id": "fixture"}
	if response := run.Call(ctx, request); response.Status != "error" || !strings.Contains(response.Result, "action_rule_required") || calls != bridgeGrantCallLimit+1 {
		t.Fatalf("daemon bypassed unattended action policy: %+v", response)
	}
	if len(agent.PendingActionApprovals(dc.SessionID)) != 0 {
		t.Fatal("background daemon created an interactive approval")
	}
	request.Tool = "proxmox"
	if response := run.Call(ctx, request); response.Status != "error" {
		t.Fatal("daemon expanded its original manifest scope")
	}
	request.Tool, request.Parameters = "docker", map[string]interface{}{"operation": "list"}
	current := s.Cfg.Clone()
	current.Docker.ReadOnly = true
	current.Tools.PythonToolBridge.AllowedTools = nil
	s.cfgSnapshot.Store(current)
	if response := run.Call(ctx, request); response.Status != "error" || calls != bridgeGrantCallLimit+1 {
		t.Fatal("daemon ignored live bridge revocation")
	}
	run.Release()
	if response := run.Call(context.Background(), request); response.Status != "error" {
		t.Fatal("released daemon generation remained usable")
	}
}

func TestDaemonToolBridgePreservesSQLSnapshotAndCancelsRunningRequest(t *testing.T) {
	started := make(chan struct{})
	s, dc := bridgeSecurityFixture(t, func(ctx context.Context, call agent.ToolCall) (string, bool) {
		if call.Params["wait"] == true {
			close(started)
			<-ctx.Done()
		}
		return `{"status":"success"}`, true
	})
	s.Cfg.Tools.PythonToolBridge.AllowedTools = []string{"sql_query", "docker"}
	s.Cfg.Tools.PythonToolBridge.AllowedSQLConnections = []string{"original"}
	dc.Cfg = s.Cfg.Clone()
	s.bindConfigAuthorization(dc.Cfg)
	dc.SessionID, dc.MessageSource = "daemon:fixture:sql-generation", "background"
	ctx, cancel := context.WithCancel(context.Background())
	run := s.daemonToolBridgeRun(ctx, cancel, func() bool { return false }, func() {}, dc)
	defer run.Release()
	current := s.Cfg.Clone()
	current.Tools.PythonToolBridge.AllowedSQLConnections = []string{"original", "later"}
	s.cfgSnapshot.Store(current)
	response := run.Call(ctx, tools.DaemonToolRequest{Tool: "sql_query", Parameters: map[string]interface{}{"connection_name": "later"}})
	if response.Status != "error" || !strings.Contains(response.Result, "originating run capability") {
		t.Fatalf("SQL scope expanded after start: %+v", response)
	}
	run.Release()
	dc.Cfg = s.ConfigSnapshot().Clone()
	s.bindConfigAuthorization(dc.Cfg)
	ctx, cancel = context.WithCancel(context.Background())
	run = s.daemonToolBridgeRun(ctx, cancel, func() bool { return false }, func() {}, dc)
	defer run.Release()
	requestCtx, requestCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		run.Call(requestCtx, tools.DaemonToolRequest{Tool: "docker", Parameters: map[string]interface{}{"operation": "list", "wait": true}, Timeout: 60})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("daemon callback did not start")
	}
	requestCancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request cancellation did not reach daemon callback")
	}
}

func TestDaemonToolBridgeFactoryKeepsOwnerAndManifestScope(t *testing.T) {
	s, _ := bridgeSecurityFixture(t, nil)
	shutdown, stopServer := context.WithCancel(context.Background())
	defer stopServer()
	s.integrationCtx = shutdown
	parent, stopRunner := context.WithCancel(context.Background())
	defer stopRunner()
	run, err := s.newDaemonToolBridge(parent, "daemon:fixture:server-owned-generation", []string{"proxmox"})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Release()
	// Config allows Docker, but this exact process's manifest does not.
	response := run.Call(run.Owner, tools.DaemonToolRequest{Tool: "docker", Parameters: map[string]interface{}{"operation": "list"}})
	if response.Status != "error" {
		t.Fatal("factory widened the manifest scope")
	}
	stopServer()
	select {
	case <-run.Owner.Done():
	case <-time.After(time.Second):
		t.Fatal("server shutdown did not cancel the runner owner")
	}
	if response := run.Call(context.Background(), tools.DaemonToolRequest{Tool: "docker"}); response.Status != "error" {
		t.Fatal("stopped daemon accepted a later request")
	}
}
