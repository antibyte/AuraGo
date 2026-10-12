package agent

import (
	"context"
	"encoding/json"
	"strings"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/tools"
	openai "github.com/sashabaranov/go-openai"
)

func gogAgentEnabled(cfg *config.Config) bool {
	if cfg == nil || !cfg.Gog.Enabled || !cfg.Agent.AllowShell || !cfg.Agent.AllowNetworkRequests {
		return false
	}
	for _, a := range cfg.Gog.Accounts {
		if a.Enabled && a.AgentAccess {
			return true
		}
	}
	return false
}

func appendGogSchemas(out []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GogEnabled {
		return out
	}
	out = append(out, tool("gog_accounts", "List enabled gog accounts and their explicit permissions. Local status only; no Google request. Native Google Workspace is a separate connection.", schema(map[string]interface{}{})))
	for _, service := range config.GogServices {
		props := map[string]interface{}{"account_id": prop("string", "Explicit stable account ID from gog_accounts.")}
		operations := []string{}
		for _, op := range gog.Catalog() {
			if op.Service != service {
				continue
			}
			operations = append(operations, op.Operation)
			for _, field := range op.Fields {
				if _, ok := props[field.Name]; ok {
					continue
				}
				p := map[string]interface{}{"type": field.Type, "description": field.Description}
				if field.Type == "array" {
					p["items"] = map[string]interface{}{"type": "string"}
				}
				props[field.Name] = p
			}
			if op.Input {
				props["input_path"] = prop("string", "Authorized local file to upload; never a URL.")
			}
			if op.Output {
				props["output_path"] = prop("string", "New local workspace file for download/export. Existing files are never replaced.")
			}
		}
		props["operation"] = map[string]interface{}{"type": "string", "enum": operations}
		out = append(out, tool("gog_"+service, "Use Google "+service+" through the selected gog account. Use get_tool_info for per-operation arguments. Follow read/create/update/delete/send grants. Never repeat an external_status_unknown write; inspect the account first.", schema(props, "account_id", "operation")))
	}
	return out
}

func dispatchGog(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	if tc.Action != "gog_accounts" && !strings.HasPrefix(tc.Action, "gog_") {
		return "", false
	}
	result := gog.Failure("disabled")
	if gogAgentEnabled(dc.Cfg) {
		if tc.Action == "gog_accounts" {
			accounts := []config.GogAccount{}
			for _, a := range dc.Cfg.Gog.Accounts {
				if a.Enabled && a.AgentAccess {
					a.ClientID = ""
					accounts = append(accounts, a)
				}
			}
			b, _ := json.Marshal(map[string]any{"readonly": dc.Cfg.Gog.ReadOnly, "accounts": accounts})
			result = gog.Result{Status: "ok", Data: b}
		} else {
			params := map[string]any{}
			for k, v := range tc.Params {
				params[k] = v
			}
			op, _ := params["operation"].(string)
			if op == "" {
				op = tc.Operation
			}
			account, _ := params["account_id"].(string)
			delete(params, "operation")
			delete(params, "account_id")
			delete(params, "action")
			delete(params, "params")
			result = tools.ExecuteGog(ctx, dc.Cfg, dc.Vault, gog.Request{AccountID: account, Service: strings.TrimPrefix(tc.Action, "gog_"), Operation: op, Args: params})
		}
	}
	b, _ := json.Marshal(result)
	return "Tool Output: " + string(b), true
}
