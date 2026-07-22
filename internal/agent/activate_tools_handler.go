package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const maxActivateToolNames = 8

type ActivateToolsResponse struct {
	Status              string            `json:"status"`
	Disabled            []string          `json:"disabled"`
	Unknown             []string          `json:"unknown"`
	RequiredCallMethods map[string]string `json:"required_call_methods,omitempty"`
}

func handleActivateTools(tc ToolCall, logger *slog.Logger, sessionID string) string {
	names := toolArgStringSlice(tc.Params, "names", "tool_names")
	if len(names) == 0 {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status: "error",
		})
	}
	if len(names) > maxActivateToolNames {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status: "error",
		})
	}

	catalog := GetToolCatalogState(sessionID)
	if catalog == nil {
		return encodeActivateToolsResponse(ActivateToolsResponse{
			Status: "error",
		})
	}

	resp := ActivateToolsResponse{
		Status:              "success",
		RequiredCallMethods: make(map[string]string),
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

		if entry.Status == ToolStatusDisabled || !entry.Enabled {
			resp.Disabled = append(resp.Disabled, canonicalName)
			continue
		}
		resp.RequiredCallMethods[canonicalName] = callMethodForEntry(entry)
	}
	if logger != nil {
		logger.Debug("[NativeTools] Legacy activate_tools call resolved without changing tool state",
			"session_id", normalizeDiscoverSessionID(sessionID),
			"requested_count", len(names))
	}
	return encodeActivateToolsResponse(resp)
}

func encodeActivateToolsResponse(resp ActivateToolsResponse) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to encode response: %s"}`, err)
	}
	return "Tool Output: " + string(b)
}
