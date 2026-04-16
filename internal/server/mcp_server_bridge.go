package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"
)

var mcpVSCodeDebugBridgeTools = []string{
	"ask_aurago",
	"filesystem",
	"smart_file_read",
	"execute_shell",
	"api_request",
	"query_memory",
	"context_memory",
	"homepage",
	"netlify",
	"web_capture",
}

func mcpServerEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.MCPServer.Enabled || cfg.MCPServer.VSCodeDebugBridge
}

func mcpServerRequireAuth(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.MCPServer.RequireAuth || cfg.MCPServer.VSCodeDebugBridge
}

func mcpEffectiveAllowedTools(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	if !cfg.MCPServer.VSCodeDebugBridge {
		return append([]string(nil), cfg.MCPServer.AllowedTools...)
	}
	if len(cfg.MCPServer.AllowedTools) == 0 {
		return append([]string(nil), mcpVSCodeDebugBridgeTools...)
	}
	return uniqueStrings(append(append([]string(nil), cfg.MCPServer.AllowedTools...), mcpVSCodeDebugBridgeTools...))
}

func mcpVSCodeBridgeToolSchema() mcpToolSchema {
	return mcpToolSchema{
		Name:        "ask_aurago",
		Description: "Ask AuraGo's live agent to inspect the current system, explain issues, or perform guided debugging steps using its own reasoning and available tools.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Your debugging question or instruction for AuraGo. Be specific about the symptom, logs, API, or live system behavior you want investigated.",
				},
			},
			"required": []string{"message"},
		},
	}
}

func mcpBuildToolCatalog(s *Server) []mcpToolSchema {
	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()

	ff := mcpFeatureFlags(s)
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	oaiTools := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, s.Logger)

	result := make([]mcpToolSchema, 0, len(oaiTools)+1)
	for _, t := range oaiTools {
		if t.Function == nil {
			continue
		}
		schema := mcpToolSchema{
			Name:        t.Function.Name,
			Description: t.Function.Description,
		}
		if t.Function.Parameters != nil {
			schema.InputSchema = t.Function.Parameters
		} else {
			schema.InputSchema = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}
		result = append(result, schema)
	}

	if cfg.MCPServer.VSCodeDebugBridge {
		result = appendToolSchemaIfMissing(result, mcpVSCodeBridgeToolSchema())
	}

	return result
}

func appendToolSchemaIfMissing(list []mcpToolSchema, schema mcpToolSchema) []mcpToolSchema {
	for _, item := range list {
		if item.Name == schema.Name {
			return list
		}
	}
	return append(list, schema)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mcpServerEndpointURL(r *http.Request, cfg *config.Config) string {
	scheme := "http"
	if r != nil && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}

	host := ""
	if r != nil {
		host = r.Host
	}
	if host == "" && cfg != nil {
		if cfg.Server.Host == "" || cfg.Server.Host == "0.0.0.0" {
			host = fmt.Sprintf("localhost:%d", cfg.Server.Port)
		} else {
			host = fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		}
	}

	return fmt.Sprintf("%s://%s/mcp", scheme, host)
}

func buildVSCodeBridgeConfigSnippet(endpointURL string) (string, error) {
	payload := map[string]interface{}{
		"inputs": []map[string]interface{}{
			{
				"type":        "promptString",
				"id":          "aurago-mcp-token",
				"description": "AuraGo MCP token",
				"password":    true,
			},
		},
		"servers": map[string]interface{}{
			"aurago-live": map[string]interface{}{
				"type": "http",
				"url":  endpointURL,
				"headers": map[string]string{
					"Authorization": "Bearer ${input:aurago-mcp-token}",
				},
			},
		},
	}

	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

func buildCursorBridgeConfigSnippet(endpointURL string) (string, error) {
	payload := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"aurago-live": map[string]interface{}{
				"url": endpointURL,
				"headers": map[string]string{
					"Authorization": "Bearer ${env:AURAGO_MCP_TOKEN}",
				},
			},
		},
	}

	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

func buildCursorBridgeInstallLink(endpointURL string) (string, error) {
	payload := map[string]interface{}{
		"url": endpointURL,
		"headers": map[string]string{
			"Authorization": "Bearer ${env:AURAGO_MCP_TOKEN}",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "cursor://anysphere.cursor-deeplink/mcp/install?name=aurago-live&config=" + base64.StdEncoding.EncodeToString(raw), nil
}

func buildClaudeDesktopBridgeConfigSnippet(endpointURL string) (string, error) {
	payload := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"aurago-live": map[string]interface{}{
				"type": "http",
				"url":  endpointURL,
				"headers": map[string]string{
					"Authorization": "Bearer ${AURAGO_MCP_TOKEN}",
				},
			},
		},
	}

	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

func handleMCPServerVSCodeBridgeInfo(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		endpointURL := mcpServerEndpointURL(r, cfg)
		vsCodeSnippet, err := buildVSCodeBridgeConfigSnippet(endpointURL)
		if err != nil {
			jsonError(w, "Failed to build VS Code config", http.StatusInternalServerError)
			return
		}
		cursorSnippet, err := buildCursorBridgeConfigSnippet(endpointURL)
		if err != nil {
			jsonError(w, "Failed to build Cursor config", http.StatusInternalServerError)
			return
		}
		cursorLink, err := buildCursorBridgeInstallLink(endpointURL)
		if err != nil {
			jsonError(w, "Failed to build Cursor install link", http.StatusInternalServerError)
			return
		}
		claudeDesktopSnippet, err := buildClaudeDesktopBridgeConfigSnippet(endpointURL)
		if err != nil {
			jsonError(w, "Failed to build Claude Desktop config", http.StatusInternalServerError)
			return
		}

		tokenPresent := false
		if s.Vault != nil {
			if token, readErr := s.Vault.ReadSecret(mcpVaultTokenKey); readErr == nil && strings.TrimSpace(token) != "" {
				tokenPresent = true
			}
		}

		mcpWriteJSON(w, http.StatusOK, map[string]interface{}{
			"enabled":             mcpServerEnabled(cfg),
			"require_auth":        mcpServerRequireAuth(cfg),
			"vscode_debug_bridge": cfg.MCPServer.VSCodeDebugBridge,
			"endpoint_url":        endpointURL,
			"recommended_tools":   append([]string(nil), mcpVSCodeDebugBridgeTools...),
			"token_present":       tokenPresent,
			"clients": map[string]interface{}{
				"vscode": map[string]interface{}{
					"label":  "VS Code",
					"config": vsCodeSnippet,
				},
				"cursor": map[string]interface{}{
					"label":        "Cursor",
					"config":       cursorSnippet,
					"install_link": cursorLink,
				},
				"claude_desktop": map[string]interface{}{
					"label":  "Claude Desktop",
					"config": claudeDesktopSnippet,
				},
			},
		})
	}
}
