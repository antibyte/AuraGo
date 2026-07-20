package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/networkshares"
)

func TestNetworkSharesToolSchemaCapabilityGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.NetworkShares.Enabled = true
	cfg.NetworkShares.ReadOnly = true
	cfg.NetworkShares.AllowCreate = true
	cfg.NetworkShares.AllowUpdate = true
	cfg.NetworkShares.AllowDelete = true
	cfg.NetworkShares.SMB.Enabled = true

	if names := builtinToolNames(buildToolFlagsFromConfig(cfg)); containsOperation(names, "network_shares") {
		t.Fatalf("network_shares present without readable runtime: %v", names)
	}

	cfg.Runtime.NetworkShares.Usable = true
	cfg.Runtime.NetworkShares.SMB.Readable = true
	readOnlyOps := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "network_shares")
	for _, operation := range []string{"status", "list", "get"} {
		if !containsOperation(readOnlyOps, operation) {
			t.Fatalf("read-only operations = %v, missing %s", readOnlyOps, operation)
		}
	}
	for _, operation := range []string{"create", "update", "delete"} {
		if containsOperation(readOnlyOps, operation) {
			t.Fatalf("read-only operations unexpectedly contain %s: %v", operation, readOnlyOps)
		}
	}

	cfg.NetworkShares.ReadOnly = false
	cfg.Runtime.NetworkShares.SMB.Writable = true
	writeOps := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "network_shares")
	for _, operation := range []string{"create", "update", "delete"} {
		if !containsOperation(writeOps, operation) {
			t.Fatalf("write operations = %v, missing %s", writeOps, operation)
		}
	}
}

func TestNetworkSharesGranularFlagsChangeSchemaCacheKey(t *testing.T) {
	base := ToolFeatureFlags{NetworkSharesEnabled: true}
	create := base
	create.NetworkSharesCreateEnabled = true
	update := create
	update.NetworkSharesUpdateEnabled = true
	remove := update
	remove.NetworkSharesDeleteEnabled = true
	if base.Key() == create.Key() || create.Key() == update.Key() || update.Key() == remove.Key() {
		t.Fatal("network share permission variants must have distinct schema cache keys")
	}
}

func TestNetworkSharesDispatchAndInvokeToolRouteToSharedManager(t *testing.T) {
	resetToolCatalogForTest(t)
	manager, err := networkshares.OpenManager(filepath.Join(t.TempDir(), "network_shares.db"), slog.Default())
	if err != nil {
		t.Fatalf("OpenManager: %v", err)
	}
	networkshares.SetDefaultManager(manager)
	t.Cleanup(func() {
		networkshares.SetDefaultManager(nil)
		_ = manager.Close()
	})

	cfg := &config.Config{}
	cfg.NetworkShares.Enabled = true
	cfg.NetworkShares.ReadOnly = true
	cfg.NetworkShares.SMB.Enabled = true
	cfg.Runtime.NetworkShares.Usable = true
	cfg.Runtime.NetworkShares.SMB.Readable = true
	manager.Configure(config.NetworkSharesOptions(cfg, ""))

	dc := &DispatchContext{
		Cfg:       cfg,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID: "network-shares-invoke",
	}
	direct, handled := dispatchPlatform(context.Background(), ToolCall{
		Action:    "network_shares",
		Operation: "status",
		Params:    map[string]interface{}{"operation": "status"},
	}, dc)
	if !handled || !strings.Contains(direct, `"status":"ok"`) || !strings.Contains(direct, `"runtime"`) {
		t.Fatalf("direct network_shares dispatch = handled %v, output %s", handled, direct)
	}

	schemas := builtinToolSchemas(buildToolFlagsFromConfig(cfg))
	SetDiscoverToolsState(dc.SessionID, schemas, nil, "")
	invoked, handled := dispatchComm(context.Background(), ToolCall{
		Action: "invoke_tool",
		Params: map[string]interface{}{
			"tool_name": "network_shares",
			"arguments": map[string]interface{}{"operation": "status"},
		},
	}, dc)
	if !handled || !strings.Contains(invoked, `"status":"ok"`) || !strings.Contains(invoked, `"runtime"`) {
		t.Fatalf("invoke_tool network_shares dispatch = handled %v, output %s", handled, invoked)
	}
}

func TestNetworkSharesToolHidesUnexpectedInternalErrors(t *testing.T) {
	output := networkSharesToolResult(nil, errors.New("database failed at C:\\secret\\network_shares.db"))
	if strings.Contains(output, `C:\\secret`) || strings.Contains(output, "database failed") {
		t.Fatalf("tool output leaked internal error details: %s", output)
	}
	if !strings.Contains(output, networkshares.ErrorApplyFailed) {
		t.Fatalf("tool output missing stable apply code: %s", output)
	}
}
