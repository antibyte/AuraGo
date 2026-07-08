package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func dispatchComposioCall(ctx context.Context, req composioCallArgs, cfg *config.Config) string {
	if cfg == nil || !cfg.Composio.Enabled {
		return `Tool Output: {"status":"error","message":"Composio is disabled. Enable composio.enabled and configure the API key in the vault."}`
	}
	if strings.TrimSpace(cfg.Composio.APIKey) == "" {
		return `Tool Output: {"status":"error","message":"Composio API key is not configured in the vault."}`
	}

	client := tools.NewComposioClientFromConfig(cfg.Composio)
	policy := tools.ComposioPolicyFromConfig(cfg.Composio)
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	switch op {
	case "capabilities":
		return dispatchComposioCapabilities(ctx, client, policy, req, cfg)

	case "search_toolkits":
		page, err := client.ListToolkits(ctx, tools.ComposioListQuery{Query: req.Query, Cursor: req.Cursor, Limit: limit})
		if err != nil {
			return composioErrorOutput("search_toolkits", err)
		}
		return composioExternalOutput(map[string]interface{}{
			"status":      "success",
			"operation":   op,
			"toolkits":    page.Items,
			"next_cursor": page.NextCursor,
			"total":       page.Total,
		})

	case "search_tools":
		if req.ToolkitSlug != "" && !composioToolkitEnabled(policy, req.ToolkitSlug) {
			return composioJSONOutput(map[string]interface{}{
				"status":       "policy_denied",
				"message":      fmt.Sprintf("Composio toolkit %q is not enabled by the user.", req.ToolkitSlug),
				"toolkit_slug": req.ToolkitSlug,
			})
		}
		page, err := client.ListTools(ctx, tools.ComposioToolQuery{
			ComposioListQuery: tools.ComposioListQuery{Query: req.Query, Cursor: req.Cursor, Limit: limit},
			ToolkitSlug:       req.ToolkitSlug,
		})
		if err != nil {
			return composioErrorOutput("search_tools", err)
		}
		items := filterComposioToolsForPolicy(policy, page.Items, req.ToolkitSlug)
		return composioExternalOutput(map[string]interface{}{
			"status":      "success",
			"operation":   op,
			"tools":       items,
			"next_cursor": page.NextCursor,
			"total":       page.Total,
		})

	case "get_tool":
		if strings.TrimSpace(req.ToolSlug) == "" {
			return `Tool Output: {"status":"error","message":"tool_slug is required for get_tool"}`
		}
		toolInfo, err := client.GetTool(ctx, req.ToolSlug)
		if err != nil {
			return composioErrorOutput("get_tool", err)
		}
		decision := tools.EvaluateComposioToolPolicy(policy, toolInfo)
		return composioExternalOutput(map[string]interface{}{
			"status":          "success",
			"operation":       op,
			"tool":            toolInfo,
			"policy_decision": decision,
		})

	case "list_connected_accounts":
		if strings.TrimSpace(req.ToolkitSlug) != "" && !composioToolkitEnabled(policy, req.ToolkitSlug) {
			return composioJSONOutput(map[string]interface{}{
				"status":       "policy_denied",
				"message":      fmt.Sprintf("Composio toolkit %q is not enabled by the user.", req.ToolkitSlug),
				"toolkit_slug": req.ToolkitSlug,
			})
		}
		page, err := client.ListConnectedAccounts(ctx, req.ToolkitSlug, cfg.Composio.UserID)
		if err != nil {
			return composioErrorOutput("list_connected_accounts", err)
		}
		return composioExternalOutput(map[string]interface{}{
			"status":             "success",
			"operation":          op,
			"connected_accounts": page.Items,
			"next_cursor":        page.NextCursor,
			"total":              page.Total,
		})

	case "execute_tool":
		return dispatchComposioExecute(ctx, client, policy, req, cfg)

	default:
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"unknown composio_call operation %q. Use capabilities, search_toolkits, search_tools, get_tool, list_connected_accounts, or execute_tool."}`, op)
	}
}

func dispatchComposioCapabilities(ctx context.Context, client *tools.ComposioClient, policy tools.ComposioPolicyConfig, req composioCallArgs, cfg *config.Config) string {
	toolkitSlug := strings.ToLower(strings.TrimSpace(req.ToolkitSlug))
	if toolkitSlug != "" && !composioToolkitEnabled(policy, toolkitSlug) {
		return composioJSONOutput(map[string]interface{}{
			"status":       "policy_denied",
			"operation":    "capabilities",
			"message":      fmt.Sprintf("Composio toolkit %q is not enabled by the user.", toolkitSlug),
			"toolkit_slug": toolkitSlug,
		})
	}

	toolkits := composioCapabilityToolkits(policy, toolkitSlug)
	payload := map[string]interface{}{
		"status":                "success",
		"operation":             "capabilities",
		"toolkits":              toolkits,
		"enabled_toolkit_count": len(toolkits),
		"read_only":             policy.ReadOnly,
		"allow_destructive":     policy.AllowDestructive,
	}
	if toolkitSlug == "" {
		return composioJSONOutput(payload)
	}

	payload["toolkit_slug"] = toolkitSlug
	page, err := client.ListConnectedAccounts(ctx, toolkitSlug, cfg.Composio.UserID)
	if err != nil {
		payload["connection_status"] = "error"
		payload["message"] = security.Scrub(err.Error())
		return composioJSONOutput(payload)
	}
	activeCount := 0
	for _, account := range page.Items {
		if composioAccountIsActive(account) {
			activeCount++
		}
	}
	payload["connected_account_count"] = activeCount
	if activeCount > 0 {
		payload["connection_status"] = "connected"
	} else {
		payload["connection_status"] = "connect_required"
		payload["message"] = "No connected Composio account is available for this toolkit. Connect it in the Composio configuration UI first."
	}
	return composioJSONOutput(payload)
}

func composioCapabilityToolkits(policy tools.ComposioPolicyConfig, onlySlug string) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(policy.Toolkits))
	seen := make(map[string]bool, len(policy.Toolkits))
	for _, tk := range policy.Toolkits {
		slug := strings.ToLower(strings.TrimSpace(tk.Slug))
		if slug == "" || !tk.Enabled || seen[slug] {
			continue
		}
		if onlySlug != "" && slug != onlySlug {
			continue
		}
		seen[slug] = true
		readOnly := policy.ReadOnly
		if tk.ReadOnly != nil {
			readOnly = *tk.ReadOnly
		}
		allowDestructive := policy.AllowDestructive
		if tk.AllowDestructive != nil {
			allowDestructive = *tk.AllowDestructive
		}
		allowNL := policy.AllowNaturalLanguageInput
		if tk.AllowNaturalLanguageInput != nil {
			allowNL = *tk.AllowNaturalLanguageInput
		}
		items = append(items, map[string]interface{}{
			"toolkit_slug":                 slug,
			"read_only":                    readOnly,
			"allow_destructive":            allowDestructive,
			"allow_natural_language_input": allowNL,
			"allowed_tool_count":           len(tk.AllowedToolSlugs),
			"blocked_tool_count":           len(tk.BlockedToolSlugs),
		})
	}
	return items
}

func composioAccountIsActive(account tools.ComposioConnectedAccount) bool {
	status := strings.ToLower(strings.TrimSpace(account.Status))
	return account.ID != "" && (status == "" || status == "active" || status == "connected" || status == "success")
}

func dispatchComposioExecute(ctx context.Context, client *tools.ComposioClient, policy tools.ComposioPolicyConfig, req composioCallArgs, cfg *config.Config) string {
	if strings.TrimSpace(req.ToolSlug) == "" {
		return `Tool Output: {"status":"error","message":"tool_slug is required for execute_tool"}`
	}
	toolInfo, err := client.GetTool(ctx, req.ToolSlug)
	if err != nil {
		if strings.TrimSpace(req.ToolkitSlug) == "" {
			return composioErrorOutput("get_tool", err)
		}
		toolInfo = tools.ComposioToolInfo{Slug: req.ToolSlug, ToolkitSlug: req.ToolkitSlug}
	}
	if toolInfo.ToolkitSlug == "" {
		toolInfo.ToolkitSlug = req.ToolkitSlug
	}
	decision := tools.EvaluateComposioToolPolicy(policy, toolInfo)
	if !decision.Allowed {
		return composioJSONOutput(map[string]interface{}{
			"status":          "policy_denied",
			"message":         decision.Reason,
			"policy_decision": decision,
		})
	}
	if strings.TrimSpace(req.Text) != "" && !tools.ComposioToolkitAllowsNaturalLanguage(policy, toolInfo.ToolkitSlug) {
		return composioJSONOutput(map[string]interface{}{
			"status":       "policy_denied",
			"message":      "Composio natural-language tool input is disabled by policy.",
			"toolkit_slug": toolInfo.ToolkitSlug,
			"tool_slug":    toolInfo.Slug,
		})
	}

	accountID := strings.TrimSpace(req.ConnectedAccountID)
	if accountID == "" {
		accountID = tools.ComposioPreferredConnectedAccount(policy, toolInfo.ToolkitSlug)
	}
	if accountID == "" {
		accountID, err = firstActiveComposioAccount(ctx, client, toolInfo.ToolkitSlug, cfg.Composio.UserID)
		if err != nil {
			return composioErrorOutput("list_connected_accounts", err)
		}
	}
	if accountID == "" {
		return composioJSONOutput(map[string]interface{}{
			"status":       "connect_required",
			"message":      "No connected Composio account is available for this toolkit. Connect it in the Composio configuration UI first.",
			"toolkit_slug": toolInfo.ToolkitSlug,
			"tool_slug":    toolInfo.Slug,
		})
	}

	result, err := client.ExecuteTool(ctx, tools.ComposioExecuteRequest{
		ToolSlug:           req.ToolSlug,
		ToolkitSlug:        toolInfo.ToolkitSlug,
		ConnectedAccountID: accountID,
		UserID:             cfg.Composio.UserID,
		Arguments:          req.Arguments,
		Text:               req.Text,
	})
	if err != nil {
		return composioErrorOutput("execute_tool", err)
	}
	return composioExternalOutput(map[string]interface{}{
		"status":               "success",
		"operation":            "execute_tool",
		"tool_slug":            req.ToolSlug,
		"toolkit_slug":         toolInfo.ToolkitSlug,
		"connected_account_id": accountID,
		"result":               json.RawMessage(result.Raw),
	})
}

func composioToolkitEnabled(policy tools.ComposioPolicyConfig, toolkitSlug string) bool {
	decision := tools.EvaluateComposioToolPolicy(policy, tools.ComposioToolInfo{
		Slug:        "GET",
		ToolkitSlug: toolkitSlug,
	})
	return !strings.Contains(strings.ToLower(decision.Reason), "not enabled")
}

func filterComposioToolsForPolicy(policy tools.ComposioPolicyConfig, items []tools.ComposioToolInfo, fallbackToolkit string) []tools.ComposioToolInfo {
	filtered := make([]tools.ComposioToolInfo, 0, len(items))
	for _, item := range items {
		if item.ToolkitSlug == "" {
			item.ToolkitSlug = fallbackToolkit
		}
		if item.ToolkitSlug == "" || composioToolkitEnabled(policy, item.ToolkitSlug) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func firstActiveComposioAccount(ctx context.Context, client *tools.ComposioClient, toolkitSlug, userID string) (string, error) {
	if strings.TrimSpace(toolkitSlug) == "" {
		return "", nil
	}
	page, err := client.ListConnectedAccounts(ctx, toolkitSlug, userID)
	if err != nil {
		return "", err
	}
	for _, account := range page.Items {
		status := strings.ToLower(strings.TrimSpace(account.Status))
		if account.ID != "" && (status == "" || status == "active" || status == "connected" || status == "success") {
			return account.ID, nil
		}
	}
	return "", nil
}

func composioErrorOutput(operation string, err error) string {
	return composioJSONOutput(map[string]interface{}{
		"status":    "error",
		"operation": operation,
		"message":   security.Scrub(err.Error()),
	})
}

func composioJSONOutput(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + string(raw)
}

func composioExternalOutput(payload map[string]interface{}) string {
	raw, _ := json.Marshal(payload)
	return "Tool Output: " + security.IsolateExternalData(security.Scrub(string(raw)))
}
