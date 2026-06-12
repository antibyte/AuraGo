package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const maxActivateToolNames = 8

type ActivateToolsResponse struct {
	Status        string   `json:"status"`
	Message       string   `json:"message,omitempty"`
	Activated     []string `json:"activated"`
	AlreadyActive []string `json:"already_active"`
	Disabled      []string `json:"disabled"`
	Unknown       []string `json:"unknown"`
	NextRequest   bool     `json:"next_request"`
}

func handleActivateTools(tc ToolCall, logger *slog.Logger, sessionID string) string {
	names := toolArgStringSlice(tc.Params, "names", "tool_names")
	if len(names) == 0 {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status:  "error",
			Message: "names is required",
		})
	}
	if len(names) > maxActivateToolNames {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status:  "error",
			Message: fmt.Sprintf("at most %d tool names can be activated per call", maxActivateToolNames),
		})
	}

	catalog := GetToolCatalogState(sessionID)
	if catalog == nil {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status:  "error",
			Message: "tool state is not available yet",
		})
	}

	resp := ActivateToolsResponse{
		Status:      "success",
		NextRequest: true,
	}
	seen := make(map[string]bool, len(names))
	for _, rawName := range names {
		rawName = strings.TrimSpace(rawName)
		if rawName == "" {
			continue
		}
		entry, ok := catalog.Get(rawName)
		if !ok || entry == nil {
			if !seen["unknown:"+rawName] {
				resp.Unknown = append(resp.Unknown, rawName)
				seen["unknown:"+rawName] = true
			}
			continue
		}
		canonicalName := entry.Name
		if canonicalName == "" {
			canonicalName = rawName
		}
		if seen["entry:"+canonicalName] {
			continue
		}
		seen["entry:"+canonicalName] = true

		switch {
		case entry.Status == ToolStatusDisabled || !entry.Enabled:
			resp.Disabled = append(resp.Disabled, canonicalName)
		case entry.Active:
			resp.AlreadyActive = append(resp.AlreadyActive, canonicalName)
		default:
			resp.Activated = append(resp.Activated, canonicalName)
			if schemaName := activationSchemaName(entry); schemaName != "" {
				MarkActivatedTool(sessionID, schemaName)
			}
		}
	}
	if logger != nil && len(resp.Activated) > 0 {
		logger.Debug("[NativeTools] Activated hidden tools for next request",
			"session_id", normalizeDiscoverSessionID(sessionID),
			"tools", strings.Join(resp.Activated, ","))
	}
	return encodeActivateToolsResponse(resp)
}

func activationSchemaName(entry *ToolCatalogEntry) string {
	if entry == nil {
		return ""
	}
	if entry.Schema.Function != nil && entry.Schema.Function.Name != "" {
		return entry.Schema.Function.Name
	}
	if entry.Routing.NativeAction != "" {
		return entry.Routing.NativeAction
	}
	if entry.Routing.SkillName != "" {
		return "skill__" + entry.Routing.SkillName
	}
	if entry.Routing.CustomName != "" {
		return "tool__" + entry.Routing.CustomName
	}
	return entry.Name
}

func encodeActivateToolsResponse(resp ActivateToolsResponse) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to encode response: %s"}`, err)
	}
	return "Tool Output: " + string(b)
}
