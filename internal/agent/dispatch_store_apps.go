package agent

import (
	"aurago/internal/tools"
	"bytes"
	"context"
	"encoding/json"
)

func dispatchStoreApplication(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	cfg := dc.Cfg
	perms := tools.RuntimePermissionsFromConfig(cfg)
	if previous, ok := tools.EffectiveRuntimePermissions(ctx); ok {
		perms.AllowFilesystemWrite = perms.AllowFilesystemWrite && previous.AllowFilesystemWrite
		perms.AllowNetworkRequests = perms.AllowNetworkRequests && previous.AllowNetworkRequests
	}
	if !perms.AllowNetworkRequests {
		return "Tool Output: " + tools.ErrorJSON("Network access is disabled for this run")
	}
	ctx = tools.WithRuntimePermissions(ctx, perms)
	if !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled || !cfg.Agent.AllowNetworkRequests {
		return "Tool Output: " + tools.ErrorJSON("Store app access is disabled by Desktop or network permissions")
	}
	var req tools.StoreAppRequest
	data, err := json.Marshal(tc.Params)
	if err != nil {
		return "Tool Output: " + tools.ErrorJSON("Invalid Store app arguments")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&req); err != nil {
		return "Tool Output: " + tools.ErrorJSON("Invalid Store app arguments")
	}
	if req.Operation == "" {
		req.Operation = tc.Operation
	}
	if cfg.VirtualDesktop.ReadOnly && (req.Operation == "trigger" || req.Operation == "execute") {
		return "Tool Output: " + tools.ErrorJSON("Desktop is read-only")
	}
	if tc.Action == "stirling_pdf" && (cfg.VirtualDesktop.ReadOnly || !cfg.Agent.AllowFilesystemWrite) {
		return "Tool Output: " + tools.ErrorJSON("PDF publication is disabled")
	}
	id := tc.Action
	if id == "node_red" {
		id = "node-red"
	}
	if id == "filebrowser" {
		id = "filebrowser-quantum"
	}
	if id == "stirling_pdf" {
		id = "stirling-pdf"
	}
	return externalToolOutput(ctx, tools.ExecuteStoreApplication(ctx, id, req))
}
