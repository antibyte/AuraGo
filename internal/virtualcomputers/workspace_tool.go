package virtualcomputers

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"aurago/internal/security"
)

func ExecuteWorkspaceTool(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, args map[string]interface{}) string {
	manager := DefaultWorkspaceManager()
	if manager == nil {
		return toolJSON("error", "workspace_manager_unavailable", "virtual workspace manager is unavailable", nil)
	}
	if !cfg.Enabled || !cfg.ToolGate {
		return toolJSON("error", "disabled", "virtual computers are disabled", nil)
	}
	if !cfg.AgentControl.Enabled {
		return toolJSON("error", "agent_control_disabled", "virtual computer agent control is disabled", nil)
	}
	operation := strings.ToLower(strings.TrimSpace(toolString(args, "operation", "action")))
	workspaceID := toolString(args, "workspace_id", "id")
	switch operation {
	case "open":
		workspace, err := manager.Open(ctx, cfg, identity, WorkspaceOpenRequest{
			Template: toolString(args, "template"), NetworkProfile: toolString(args, "network_profile"), VolumeID: toolString(args, "volume_id"),
		})
		return workspaceToolResult("workspace opened", map[string]interface{}{"workspace": workspace}, err)
	case "get":
		workspace, err := manager.Get(ctx, identity, workspaceID)
		return workspaceToolResult("workspace loaded", map[string]interface{}{"workspace": workspace}, err)
	case "list":
		workspaces, err := manager.List(ctx, identity, toolBool(args, "include_closed"))
		return workspaceToolResult("workspaces listed", map[string]interface{}{"workspaces": workspaces}, err)
	case "close":
		err := manager.CloseWorkspace(ctx, cfg, identity, workspaceID)
		return workspaceToolResult("workspace closed", nil, err)
	case "exec":
		job, output, err := manager.Exec(ctx, cfg, identity, workspaceID, WorkspaceExecRequest{
			Command: toolString(args, "command"), WorkingDir: toolString(args, "working_dir"), TimeoutSeconds: toolInt(args, 0, "timeout_seconds"),
		})
		return workspaceToolResult("command executed", map[string]interface{}{"job": job, "output": output}, err)
	case "start_job":
		job, err := manager.StartJob(ctx, cfg, identity, workspaceID, WorkspaceStartJobRequest{
			Command: toolString(args, "command"), WorkingDir: toolString(args, "working_dir"), PTY: toolBool(args, "pty"),
			Rows: uint16(toolInt(args, 24, "rows")), Cols: uint16(toolInt(args, 80, "cols")), TimeoutSeconds: toolInt(args, 0, "timeout_seconds"),
			WaitForCredentialGrant: toolBool(args, "wait_for_credential_grant"),
		})
		return workspaceToolResult("job started", map[string]interface{}{"job": job}, err)
	case "job_status":
		job, err := manager.JobStatus(ctx, cfg, identity, workspaceID, toolString(args, "job_id"))
		return workspaceToolResult("job status loaded", map[string]interface{}{"job": job}, err)
	case "job_output":
		output, err := manager.JobOutput(ctx, cfg, identity, workspaceID, toolString(args, "job_id"), int64(toolInt(args, 0, "cursor")), toolInt(args, workspaceOutputPageBytes, "limit"))
		return workspaceToolResult("job output loaded", map[string]interface{}{"output": output}, err)
	case "job_input":
		err := manager.JobInput(ctx, cfg, identity, workspaceID, toolString(args, "job_id"), toolString(args, "input"), uint16(toolInt(args, 0, "rows")), uint16(toolInt(args, 0, "cols")))
		return workspaceToolResult("job input sent", nil, err)
	case "cancel_job":
		err := manager.CancelJob(ctx, cfg, identity, workspaceID, toolString(args, "job_id"))
		return workspaceToolResult("job canceled", nil, err)
	case "list_files":
		entries, err := manager.ListFiles(ctx, cfg, identity, workspaceID, toolString(args, "path"))
		return workspaceToolResult("workspace files listed", map[string]interface{}{"files": entries}, err)
	case "read_file", "download":
		data, eof, err := manager.ReadFile(ctx, cfg, identity, workspaceID, toolString(args, "path"), int64(toolInt(args, 0, "offset")), int64(toolInt(args, 4*1024*1024, "limit")))
		payload := map[string]interface{}{"content_base64": base64.StdEncoding.EncodeToString(data), "eof": eof}
		if operation == "read_file" && toolBool(args, "text") {
			payload["content"] = string(data)
		}
		return workspaceToolResult("workspace file read", payload, err)
	case "write_file", "upload":
		data := []byte(toolString(args, "content"))
		if encoded := toolString(args, "content_base64"); encoded != "" {
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return toolJSON("error", "invalid_base64", "content_base64 is invalid", nil)
			}
			data = decoded
		}
		err := manager.WriteFile(ctx, cfg, identity, workspaceID, toolString(args, "path"), data, toolBool(args, "append"))
		return workspaceToolResult("workspace file written", map[string]interface{}{"bytes": len(data)}, err)
	case "checkpoint":
		volume, err := manager.Checkpoint(ctx, cfg, identity, workspaceID, toolInt(args, 0, "ttl_seconds"))
		return workspaceToolResult("workspace checkpoint created", map[string]interface{}{"volume": volume}, err)
	case "list_grantable_credentials":
		items, err := manager.ListGrantableCredentials(ctx, cfg)
		return workspaceToolResult("grantable credential metadata listed", map[string]interface{}{"credentials": items}, err)
	case "request_credential_grant":
		grant, err := manager.RequestCredentialGrant(ctx, cfg, identity, workspaceID, CredentialGrantRequest{
			CredentialID: toolString(args, "credential_id"), UsageType: toolString(args, "usage_type"), Origin: toolString(args, "origin"),
			JobID: toolString(args, "job_id"), FieldNames: toolStringSlice(args, "field_names"), Purpose: toolString(args, "purpose"),
		})
		return workspaceToolResult("credential grant awaits authenticated user approval", map[string]interface{}{"grant": grant, "requires_user_approval": true}, err)
	case "revoke_credential_grant":
		err := manager.RevokeCredentialGrant(ctx, cfg, identity, workspaceID, toolString(args, "grant_id"))
		return workspaceToolResult("credential grant revoked", nil, err)
	default:
		return toolJSON("error", "unsupported_operation", "unsupported virtual_workspace operation", map[string]interface{}{"operation": operation})
	}
}

func ExecuteBrowserTool(ctx context.Context, cfg ToolConfig, identity WorkspaceIdentity, args map[string]interface{}) string {
	manager := DefaultWorkspaceManager()
	if manager == nil {
		return toolJSON("error", "workspace_manager_unavailable", "virtual workspace manager is unavailable", nil)
	}
	operation := strings.ToLower(strings.TrimSpace(toolString(args, "operation", "action")))
	request := BrowserActionRequest{
		Operation: operation, SessionID: toolString(args, "browser_session_id", "session_id"), PageID: toolString(args, "page_id"),
		URL: toolString(args, "url"), ElementRef: toolString(args, "element_ref", "ref"), Selector: toolString(args, "selector"),
		Text: toolString(args, "text"), Value: toolString(args, "value"), Key: toolString(args, "key"),
		TimeoutMS: toolInt(args, 0, "timeout_ms"), FullPage: toolBool(args, "full_page"),
		X: toolFloat(args, "x"), Y: toolFloat(args, "y"), DeltaX: toolFloat(args, "delta_x"), DeltaY: toolFloat(args, "delta_y"),
		ToX: toolFloat(args, "to_x"), ToY: toolFloat(args, "to_y"), Path: toolString(args, "path"), GrantID: toolString(args, "grant_id"),
		Options: map[string]interface{}{"submit": toolBool(args, "submit")},
	}
	workspaceID := toolString(args, "workspace_id", "id")
	var result BrowserActionResult
	var err error
	if operation == "credential_fill" {
		result, err = manager.UseBrowserGrant(ctx, cfg, identity, workspaceID, request)
	} else {
		result, err = manager.BrowserAction(ctx, cfg, identity, workspaceID, request)
	}
	return workspaceToolResult("browser operation completed", map[string]interface{}{"result": result}, err)
}

func workspaceToolResult(message string, payload interface{}, err error) string {
	if err == nil {
		return security.Scrub(toolJSON("ok", "", message, payload))
	}
	if rpcErr, ok := err.(WorkspaceRPCError); ok {
		return security.Scrub(toolJSON("error", rpcErr.Code, rpcErr.Message, rpcErr.Data))
	}
	if rpcErr, ok := err.(*WorkspaceRPCError); ok {
		return security.Scrub(toolJSON("error", rpcErr.Code, rpcErr.Message, rpcErr.Data))
	}
	return toolJSON("error", "workspace_error", securityScrubError(err), nil)
}

func securityScrubError(err error) string {
	if err == nil {
		return ""
	}
	return security.Scrub(strings.TrimSpace(fmt.Sprint(err)))
}

func toolFloat(args map[string]interface{}, key string) float64 {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
